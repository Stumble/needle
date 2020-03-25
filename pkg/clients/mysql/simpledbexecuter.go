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

const (
	maxSelectLen = 30
)

type simpleDBExecuter struct {
	conn         *sql.DB
	counter      *prometheus.CounterVec
	histogram    *prometheus.HistogramVec
	statementMap *sync.Map
}

func (m *manager) GetDBExecuter() DBExecuter {
	return m.simpleExecuter
}

func (s *simpleDBExecuter) Invalidate(f func()) error {
	f()
	return nil
}

func (s *simpleDBExecuter) cleanStatement(statement string) string {
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

func (s *simpleDBExecuter) Query(ctx context.Context, unprepared string, args ...interface{}) (*sql.Rows, error) {
	if s.counter != nil {
		s.counter.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Inc()
	}
	if s.histogram != nil {
		startTime := time.Now()
		defer func() {
			s.histogram.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Observe(time.Now().Sub(startTime).Seconds())
		}()
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "SQL QUERY")
	tags.SpanKindRPCClient.Set(span)
	tags.PeerService.Set(span, "mysql")
	span.SetTag("db.statement", unprepared)
	defer span.Finish()
	// r, err := s.conn.QueryContext(ctx, unprepared, args...)
	r, err := s.conn.QueryContext(ctx, unprepared, args...)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (s *simpleDBExecuter) QueryRow(ctx context.Context, unprepared string, args ...interface{}) (*sql.Row, error) {
	if s.counter != nil {
		s.counter.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Inc()
	}
	if s.histogram != nil {
		startTime := time.Now()
		defer func() {
			s.histogram.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Observe(time.Now().Sub(startTime).Seconds())
		}()
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "SQL QUERY")
	tags.SpanKindRPCClient.Set(span)
	tags.PeerService.Set(span, "mysql")
	span.SetTag("db.statement", unprepared)
	defer span.Finish()
	return s.conn.QueryRowContext(ctx, unprepared, args...), nil
	// return prepared.QueryRowContext(ctx, args...)
}

func (s *simpleDBExecuter) Exec(ctx context.Context, unprepared string, args ...interface{}) (sql.Result, error) {
	if s.counter != nil {
		s.counter.WithLabelValues(s.cleanStatement(unprepared), "EXEC").Inc()
	}
	if s.histogram != nil {
		startTime := time.Now()
		defer func() {
			s.histogram.WithLabelValues(s.cleanStatement(unprepared), "QUERY").Observe(time.Now().Sub(startTime).Seconds())
		}()
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, "SQL EXEC")
	tags.SpanKindRPCClient.Set(span)
	tags.PeerService.Set(span, "mysql")
	span.SetTag("db.statement", unprepared)
	defer span.Finish()
	// r, err := prepared.ExecContext(ctx, args...)
	r, err := s.conn.ExecContext(ctx, unprepared, args...)
	if err != nil {
		return r, err
	}
	return r, nil
}

func (s *simpleDBExecuter) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	return s.conn.PrepareContext(ctx, query)
}
