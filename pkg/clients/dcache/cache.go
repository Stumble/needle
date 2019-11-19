package dcache

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
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
)

// SetNowFunc is a helper function to replace time.Now()
func SetNowFunc(f func() time.Time) { nowFunc = f }

// PassThroughFunc is the actual call to underlying data source
type PassThroughFunc = func() (interface{}, error)

// Cache defines interface to cache
type Cache interface {
	// Get returns value of f while caching in redis and inmemcache
	// Inputs:
	// queryKey	 - key used in cache
	// target	 	 - an instance of the same type as the return interface{}
	// expire 	 - expiration of cache key
	// f				 - actual call that hits underlying data source
	// noCache 	 - whether force read from data source
	Get(ctx context.Context, queryKey QueryKey, target interface{}, expire time.Duration, f PassThroughFunc, noCache bool) (interface{}, error)

	// Set explicitly set a cache key to a val
	// Inputs:
	// key		- key to set
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
	// Ignore error
	_ = prometheus.Register(counter)
	c := &Client{
		primaryConn:            primaryClient,
		promCounter:            counter,
		id:                     id.String(),
		invalidateKeys:         make(map[string]struct{}),
		invalidateMu:           &sync.Mutex{},
		invalidateCh:           make(chan struct{}),
		inMemCache:             inMemCache,
		readThroughPerKeyLimit: readThroughPerKeyLimit,
	}
	if inMemCache != nil {
		c.pubsub = c.primaryConn.Subscribe(redisCacheInvalidateTopic)
		go c.aggregateSend()
		go c.listenKeyInvalidate()
	}
	return c, nil
}

// Close terminates redis pubsub gracefully
func (c *Client) Close() {
	if c.pubsub != nil {
		c.pubsub.Unsubscribe()
		c.pubsub.Close()
	}
}

// QueryKey is an alias to string
type QueryKey = string

type nilPlaceholder struct {
	SomeRandomFieldToPreventDecoding struct{}
}

// getNoCache read through using f and populate cache if no error
func (c *Client) getNoCache(ctx context.Context, queryKey QueryKey, expire time.Duration, f PassThroughFunc) (interface{}, error) {
	if c.promCounter != nil {
		c.promCounter.WithLabelValues("MISS").Inc()
	}
	dbres, err := f()
	go func() {
		if err == nil {
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			var e error
			v := reflect.ValueOf(dbres)
			if dbres == nil || v.Kind() == reflect.Ptr && v.IsNil() {
				e = enc.Encode(&nilPlaceholder{})
			} else {
				e = enc.Encode(dbres)
			}
			if e == nil {
				c.setKey(queryKey, buf.Bytes(), expire)
			}
		} else {
			c.deleteKey(queryKey)
		}
	}()
	return dbres, err
}

// setKey set key in redis and inMemCache
func (c *Client) setKey(queryKey QueryKey, b []byte, expire time.Duration) {
	if c.primaryConn.Set(store(queryKey), b, maxCacheTime).Err() == nil {
		c.primaryConn.Set(ttl(queryKey), strconv.FormatInt(nowFunc().UTC().Add(expire).Unix(), 10), expire)
	}
	if c.inMemCache != nil {
		value, err := c.inMemCache.Get([]byte(store(queryKey)))
		if err == nil && !bytes.Equal(value, b) {
			c.broadcastKeyInvalidate(queryKey)
		}
		c.inMemCache.Set([]byte(store(queryKey)), b, int(expire/time.Second))
	}
}

// deleteKey delete key in redis and inMemCache
func (c *Client) deleteKey(queryKey QueryKey) {
	if e := c.primaryConn.Get(store(queryKey)).Err(); e != redis.Nil {
		// Delete key if error should not be cached
		c.primaryConn.Del(store(queryKey), ttl(queryKey))
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
	for {
		select {
		case <-ticker.C:
		case <-c.invalidateCh:
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
			c.primaryConn.Publish(redisCacheInvalidateTopic, msg)
		}()
	}
}

// listenKeyInvalidate subscribe to invalidate key requests and invalidates inmemcache
func (c *Client) listenKeyInvalidate() {
	ch := c.pubsub.Channel()
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
	return "{" + key + "}"
}

func lock(key QueryKey) string {
	return store(key) + lockSuffix
}

func ttl(key QueryKey) string {
	return store(key) + ttlSuffix
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
func (c *Client) Get(ctx context.Context, queryKey QueryKey, target interface{}, expire time.Duration, f PassThroughFunc, noCache bool) (interface{}, error) {
	if c.promCounter != nil {
		c.promCounter.WithLabelValues("TOTAL").Inc()
	}
	if noCache {
		return c.getNoCache(ctx, queryKey, expire, f)
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
		resList, e := readConn.MGet(store(queryKey), ttl(queryKey)).Result()
		if e == nil {
			if len(resList) != 2 {
				// Should never happen
				return typedNil(target), ErrInternal
			}
			if resList[0] != nil {
				res, _ = resList[0].(string)
			}
			if resList[1] != nil {
				ttlRes, _ = resList[1].(string)
			}
		}
		if e != nil || resList[0] == nil {
			// Empty cache, obtain lock first to query db
			// If timeout or not cacheable error, another thread will obtain lock after ratelimit
			updated, _ := c.primaryConn.SetNX(lock(queryKey), "", c.readThroughPerKeyLimit).Result()
			if updated {
				return c.getNoCache(ctx, queryKey, expire, f)
			}
			// Did not obtain lock, sleep and retry to wait for update
			select {
			case <-ctx.Done():
				return typedNil(target), ErrTimeout
			case <-time.After(minSleep):
				goto retry
			}
		}
		bRes = []byte(res)

		var inMemExpire int
		// Tries to update ttl key if it doesn't exist
		if ttlRes == "" {
			// Key has expired, try to grab update lock
			updated, _ := c.primaryConn.SetNX(ttl(queryKey), strconv.FormatInt(nowFunc().UTC().Add(expire).Unix(), 10), expire).Result()
			if updated {
				// Got update lock
				return c.getNoCache(ctx, queryKey, expire, f)
			}
			inMemExpire = int(inMemCacheTime / time.Second)
		} else {
			// ttlRes should be unix time of expireAt
			t, e := strconv.ParseInt(ttlRes, 10, 64)
			if e != nil {
				t = nowFunc().UTC().Add(inMemCacheTime).Unix()
			}
			inMemExpire = int(t - nowFunc().UTC().Unix())
		}
		// Populate inMemCache
		if c.inMemCache != nil && inMemExpire > 0 {
			c.inMemCache.Set([]byte(store(queryKey)), bRes, inMemExpire)
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

	cachedNil := &nilPlaceholder{}
	dec := gob.NewDecoder(bytes.NewBuffer(bRes))
	e := dec.Decode(cachedNil)
	if e == nil {
		return typedNil(target), nil
	}

	// check for actual value
	dec = gob.NewDecoder(bytes.NewBuffer(bRes))
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Ptr {
		// If target is not a pointer, create a pointer of target type and decode to it
		t := reflect.New(value.Type())
		e := dec.Decode(t.Interface())
		if e != nil {
			return c.getNoCache(ctx, queryKey, expire, f)
		}
		// Dereference and return the underlying target
		return t.Elem().Interface(), nil
	}
	// target is a pointer, decode directly. Use a new pointer to avoid nil target
	t := reflect.New(value.Type().Elem())
	e = dec.Decode(t.Interface())
	if e != nil {
		return c.getNoCache(ctx, queryKey, expire, f)
	}
	return t.Interface(), nil
}

// Invalidate implements Cache interface
func (c *Client) Invalidate(ctx context.Context, key QueryKey) error {
	c.deleteKey(key)
	return nil
}

// Set implements Cache interface
func (c *Client) Set(ctx context.Context, key QueryKey, val interface{}, ttl time.Duration) error {
	_, err := c.getNoCache(ctx, key, ttl, func() (interface{}, error) { return val, nil })
	return err
}
