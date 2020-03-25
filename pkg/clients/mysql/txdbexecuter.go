package mysql

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	tags "github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
)

type txDBExecuter struct {
	tx             *sql.Tx
	counter        *prometheus.CounterVec
	histogram      *prometheus.HistogramVec
	statementMap   *sync.Map
	invalidateFunc []func()
}

func (m *manager) getTxDBExecuter(tx *sql.Tx) *txDBExecuter {
	exec := &txDBExecuter{
		tx:           tx,
		counter:      m.counter,
		histogram:    m.histogram,
		statementMap: m.statementMap,
	}
	return exec
}

func (s *txDBExecuter) commit() {
	var wg sync.WaitGroup
	for _, inval := range s.invalidateFunc {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inval()
		}()
	}
	wg.Wait()
}

// It is not thread-safe and should not be called in concurrent goroutines
func (s *txDBExecuter) Invalidate(f func()) error {
	s.invalidateFunc = append(s.invalidateFunc, f)
	return nil
}

func (s *txDBExecuter) cleanStatement(statement string) string {
	if raw, ok := s.statementMap.Load(statement); ok {
		return raw.(string)
	}
	ret := strings.Replace(statement, "\n", " ", -1)
	ret = strings.Replace(ret, "\t", " ", -1)
	ret = strings.TrimSpace(ret)
	if strings.HasPrefix(ret, "SELECT") {
		kl := strings.Split(ret, "FROM")
		if len(kl) >= 2 {
			if len(kl[0]) > maxSelectLen {
				ret = kl[0][:maxSelectLen] + "... FROM" + strings.Join(kl[1:], "FROM")
			}
		}
	}
	s.statementMap.Store(statement, ret)
	return ret
}

func (s *txDBExecuter) Query(ctx context.Context, unprepared string, args ...interface{}) (*sql.Rows, error) {
	if s.counter != nil {
		s.counter.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Inc()
	}
	if s.histogram != nil {
		startTime := time.Now()
		defer func() {
			s.histogram.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Observe(time.Now().Sub(startTime).Seconds())
		}()
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "SQL TX QUERY")
	tags.SpanKindRPCClient.Set(span)
	tags.PeerService.Set(span, "mysql")
	span.SetTag("db.statement", unprepared)
	r, err := s.tx.QueryContext(ctx, unprepared, args...)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (s *txDBExecuter) Exec(ctx context.Context, unprepared string, args ...interface{}) (sql.Result, error) {
	if s.counter != nil {
		s.counter.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Inc()
	}
	if s.histogram != nil {
		startTime := time.Now()
		defer func() {
			s.histogram.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Observe(time.Now().Sub(startTime).Seconds())
		}()
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "SQL TX EXEC")
	tags.SpanKindRPCClient.Set(span)
	tags.PeerService.Set(span, "mysql")
	span.SetTag("db.statement", unprepared)
	r, err := s.tx.ExecContext(ctx, unprepared, args...)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (s *txDBExecuter) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	return s.tx.PrepareContext(ctx, query)
}
