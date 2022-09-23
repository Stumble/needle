package dcache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coocood/freecache"
	redis "github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	ttlSuffix                 = "_TTL"
	lockSuffix                = "_LOCK"
	minSleep                  = 50 * time.Millisecond
	maxCacheTime              = time.Hour * 24
	inMemCacheTime            = time.Second * 2
	redisCacheInvalidateTopic = "CacheInvalidatePubSub"
	maxInvalidate             = 100
	delimiter                 = "~|~"
)

var (
	nowFunc = time.Now
)

var (
	// ErrTimeout is timeout error
	ErrTimeout = errors.New("timeout")
	// ErrInternal should never happen
	ErrInternal = errors.New("internal")

	ErrNil = errors.New("nil")
)

// SetNowFunc is a helper function to replace time.Now()
func SetNowFunc(f func() time.Time) { nowFunc = f }

// PassThroughFunc is the actual call to underlying data source
type PassThroughFunc = func() (interface{}, error)

// PassThroughExpireFunc is the actual call to underlying data source while
// returning a duration as expire timer
type PassThroughExpireFunc = func() (interface{}, time.Duration, error)

// Cache defines interface to cache
type Cache interface {
	// Get returns value of f while caching in redis and inmemcache
	// Inputs:
	// queryKey	 - key used in cache
	// target	 - receive the cached value, must be pointer
	// expire 	 - expiration of cache key
	// f		 - actual call that hits underlying data source
	// noCache 	 - whether force read from data source
	Get(ctx context.Context, queryKey QueryKey, target interface{}, expire time.Duration, f PassThroughFunc, noCache bool) error

	// GetWithExpire returns value of f while caching in redis
	// Inputs:
	// queryKey	 - key used in cache
	// target	 - receive the cached value, must be pointer
	// f		 - actual call that hits underlying data source, sets expire duration
	// noCache 	 - whether force read from data source
	GetWithExpire(ctx context.Context, queryKey string, target interface{}, f PassThroughExpireFunc, noCache bool) error

	// Set explicitly set a cache key to a val
	// Inputs:
	// key	  - key to set
	// val	  - val to set
	// ttl    - ttl of key
	Set(ctx context.Context, key QueryKey, val interface{}, ttl time.Duration) error

	// Invalidate explicitly invalidates a cache key
	// Inputs:
	// key    - key to invalidate
	Invalidate(ctx context.Context, key QueryKey) error

	// Close closes resources used by cache
	Close()
}

// Client captures redis connection
type Client struct {
	primaryConn            redis.UniversalClient
	promCounter            *prometheus.CounterVec
	inMemCache             *freecache.Cache
	pubsub                 *redis.PubSub
	id                     string
	invalidateKeys         map[string]struct{}
	invalidateMu           *sync.Mutex
	invalidateCh           chan struct{}
	readThroughPerKeyLimit time.Duration
	ctx                    context.Context
	cancel                 context.CancelFunc
	wg                     sync.WaitGroup
}

// NewCache creates a new redis cache with inmem cache
func NewCache(
	appName string,
	primaryClient redis.UniversalClient,
	inMemCache *freecache.Cache,
	readThroughPerKeyLimit time.Duration,
) (Cache, error) {
	id := uuid.NewV4()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: fmt.Sprintf("%s_cache", appName),
		Help: "Cache operations",
	}, []string{"name"})
	_ = prometheus.Register(counter)

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		primaryConn:            primaryClient,
		promCounter:            counter,
		id:                     id.String(),
		invalidateKeys:         make(map[string]struct{}),
		invalidateMu:           &sync.Mutex{},
		invalidateCh:           make(chan struct{}),
		inMemCache:             inMemCache,
		readThroughPerKeyLimit: readThroughPerKeyLimit,
		ctx:                    ctx,
		cancel:                 cancel,
	}
	if inMemCache != nil {
		c.pubsub = c.primaryConn.Subscribe(ctx, redisCacheInvalidateTopic)
		go c.aggregateSend()
		go c.listenKeyInvalidate()
	}
	return c, nil
}

// Close terminates redis pubsub gracefully
func (c *Client) Close() {
	if c.pubsub != nil {
		// todo: handle close
		c.pubsub.Unsubscribe(c.ctx)
		c.pubsub.Close()
	}
	c.cancel()
	c.wg.Wait()
}

// QueryKey is an alias to string
type QueryKey = string

type nilPlaceholder struct {
	SomeRandomFieldToPreventDecoding struct{}
}

// getNoCache read through using f and populate cache if no error
func (c *Client) getNoCacheWithValue(ctx context.Context, queryKey QueryKey, f PassThroughExpireFunc, v interface{}, noCache bool) error {
	if c.promCounter != nil {
		c.promCounter.WithLabelValues("MISS").Inc()
	}
	dbres, expire, err := f()
	if err != nil {
		c.deleteKey(ctx, queryKey)
		return err
	}
	bs, e := marshal(dbres)
	if e != nil {
		return e
	}
	if !noCache {
		c.setKey(ctx, queryKey, bs, expire)
	}

	e = unmarshal(bs, v)
	if e != nil {
		return e
	}

	return nil
}

// setKey set key in redis and inMemCache
func (c *Client) setKey(ctx context.Context, queryKey QueryKey, b []byte, expire time.Duration) {
	if c.primaryConn.Set(ctx, store(queryKey), b, maxCacheTime).Err() == nil {
		c.primaryConn.Set(ctx, ttl(queryKey), strconv.FormatInt(nowFunc().UTC().Add(expire).Unix(), 10), expire)
	}
	if c.inMemCache != nil {
		value, err := c.inMemCache.Get([]byte(store(queryKey)))
		if err == nil && !bytes.Equal(value, b) {
			c.broadcastKeyInvalidate(queryKey)
		}
		// ignore inmem cache error
		_ = c.inMemCache.Set([]byte(store(queryKey)), b, int(expire/time.Second))
	}
}

// deleteKey delete key in redis and inMemCache
func (c *Client) deleteKey(ctx context.Context, queryKey QueryKey) {
	if e := c.primaryConn.Get(ctx, store(queryKey)).Err(); e != redis.Nil {
		// Delete key if error should not be cached
		c.primaryConn.Del(ctx, store(queryKey), ttl(queryKey))
	}
	if c.inMemCache != nil {
		_, err := c.inMemCache.Get([]byte(store(queryKey)))
		if err == nil {
			c.broadcastKeyInvalidate(queryKey)
		}
		c.inMemCache.Del([]byte(store(queryKey)))
	}
}

// broadcastKeyInvalidate pushes key into a list and wait for broadcast
func (c *Client) broadcastKeyInvalidate(queryKey QueryKey) {
	c.invalidateMu.Lock()
	c.invalidateKeys[store(queryKey)] = struct{}{}
	l := len(c.invalidateKeys)
	c.invalidateMu.Unlock()
	if l == maxInvalidate {
		c.invalidateCh <- struct{}{}
	}
}

// aggregateSend waits for 1 seconds or list accumulating more than maxInvalidate
// to send to redis pubsub
func (c *Client) aggregateSend() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-ticker.C:
		case <-c.invalidateCh:
		case <-c.ctx.Done():
			return
		}
		go func() {
			c.invalidateMu.Lock()
			if len(c.invalidateKeys) == 0 {
				c.invalidateMu.Unlock()
				return
			}
			toSend := c.invalidateKeys
			c.invalidateKeys = make(map[string]struct{})
			c.invalidateMu.Unlock()
			keys := make([]string, 0)
			for key := range toSend {
				keys = append(keys, key)
			}
			msg := c.id + delimiter + strings.Join(keys, delimiter)
			c.primaryConn.Publish(c.ctx, redisCacheInvalidateTopic, msg)
		}()
	}
}

// listenKeyInvalidate subscribe to invalidate key requests and invalidates inmemcache
func (c *Client) listenKeyInvalidate() {
	ch := c.pubsub.Channel()
	c.wg.Add(1)
	defer c.wg.Done()

	for {
		msg, ok := <-ch
		if !ok {
			return
		}
		payload := msg.Payload
		go func(payload string) {
			l := strings.Split(payload, delimiter)
			if len(l) < 2 {
				// Invalid payload
				log.Warn().Msgf("Received invalidate payload %s", payload)
				return
			}
			if l[0] == c.id {
				// Receive message from self
				return
			}
			// Invalidate key
			for _, key := range l[1:] {
				c.inMemCache.Del([]byte(key))
			}
		}(payload)
	}
}

func store(key QueryKey) string {
	return fmt.Sprintf(":{%s}", key)
}

func lock(key QueryKey) string {
	return fmt.Sprintf(":%s%s", store(key), lockSuffix)
}

func ttl(key QueryKey) string {
	return fmt.Sprintf(":%s%s", store(key), ttlSuffix)
}

// typedNil cast the ret to the nil pointer of same type if it is a pointer
func typedNil(target interface{}) interface{} {
	retReflect := reflect.ValueOf(target)
	if retReflect.Kind() == reflect.Ptr {
		value := reflect.New(retReflect.Type())
		return value.Elem().Interface()
	}
	return target
}

// Get implements Cache interface
func (c *Client) Get(ctx context.Context, queryKey QueryKey, target interface{}, expire time.Duration, f PassThroughFunc, noCache bool) error {
	fn := func() (interface{}, time.Duration, error) {
		res, err := f()
		return res, expire, err
	}

	return c.GetWithExpire(ctx, queryKey, target, fn, noCache)
}

// GetWithExpire implements Cache interface
func (c *Client) GetWithExpire(ctx context.Context, queryKey QueryKey, target interface{}, f PassThroughExpireFunc, noCache bool) error {
	if c.promCounter != nil {
		c.promCounter.WithLabelValues("TOTAL").Inc()
	}
	if noCache {
		return c.getNoCacheWithValue(ctx, queryKey, f, target, noCache)
	}
	readConn := c.primaryConn

	var bRes []byte
	if c.inMemCache != nil {
		// Check for inmemcache first
		bRes, _ = c.inMemCache.Get([]byte(store(queryKey)))
	}
	if bRes == nil {
		var res, ttlRes string
	retry:
		resList, e := readConn.MGet(ctx, store(queryKey), ttl(queryKey)).Result()
		if e == nil {
			if len(resList) != 2 {
				// Should never happen
				return ErrInternal
			}
			if resList[0] != nil {
				res, _ = resList[0].(string)
			}
			if resList[1] != nil {
				ttlRes, _ = resList[1].(string)
			}
		}
		if e != nil || resList[0] == nil || resList[1] == nil {
			// Empty cache, obtain lock first to query db
			// If timeout or not cacheable error, another thread will obtain lock after ratelimit
			updated, _ := c.primaryConn.SetNX(ctx, lock(queryKey), "", c.readThroughPerKeyLimit).Result()
			if updated {
				return c.getNoCacheWithValue(ctx, queryKey, f, target, noCache)
			}
			// Did not obtain lock, sleep and retry to wait for update
			select {
			case <-ctx.Done():
				return ErrTimeout
			case <-time.After(minSleep):
				goto retry
			}
		}
		bRes = []byte(res)

		// ttlRes should be unix time of expireAt
		t, e := strconv.ParseInt(ttlRes, 10, 64)
		if e != nil {
			t = nowFunc().UTC().Add(inMemCacheTime).Unix()
		}
		inMemExpire := int(t - nowFunc().UTC().Unix())

		// Populate inMemCache
		if c.inMemCache != nil && inMemExpire > 0 {
			_ = c.inMemCache.Set([]byte(store(queryKey)), bRes, inMemExpire)
		}
		if c.promCounter != nil {
			c.promCounter.WithLabelValues("REDIS HIT").Inc()
		}
	} else {
		if c.promCounter != nil {
			c.promCounter.WithLabelValues("INMEMCACHE HIT").Inc()
		}
	}
	if c.promCounter != nil {
		c.promCounter.WithLabelValues("HIT").Inc()
	}

	e := unmarshal(bRes, target)
	if e != nil {
		return e
	}
	return nil
}

// Invalidate implements Cache interface
func (c *Client) Invalidate(ctx context.Context, key QueryKey) error {
	c.deleteKey(ctx, key)
	return nil
}

// Set implements Cache interface
func (c *Client) Set(ctx context.Context, key QueryKey, val interface{}, ttl time.Duration) error {
	err := c.getNoCacheWithValue(ctx, key, func() (interface{}, time.Duration, error) { return val, ttl, nil }, nil, false)
	if err != ErrNil {
		return err
	}
	return nil
}

// marshal copy from https://github.com/go-redis/cache/blob/v8/cache.go#L331 and removed compression
func marshal(value interface{}) ([]byte, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return value, nil
	case string:
		return []byte(value), nil
	}

	b, err := msgpack.Marshal(value)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// unmarshal copy from https://github.com/go-redis/cache/blob/v8/cache.go#L369
func unmarshal(b []byte, value interface{}) error {
	if len(b) == 0 {
		return nil
	}

	switch value := value.(type) {
	case nil:
		return ErrNil
	case *[]byte:
		clone := make([]byte, len(b))
		copy(clone, b)
		*value = clone
		return nil
	case *string:
		*value = string(b)
		return nil
	}

	return msgpack.Unmarshal(b, value)
}
