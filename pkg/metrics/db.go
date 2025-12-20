package metrics

import (
	"context"

	prom "github.com/prometheus/client_golang/prometheus"
)

var (
	dbQueries = prom.NewCounterVec(
		prom.CounterOpts{
			Name: "db_queries_total",
			Help: "Total DB queries executed",
		},
		[]string{"query", "result"},
	)
	dbDuration = prom.NewHistogramVec(
		prom.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duration of DB queries",
			Buckets: prom.DefBuckets,
		},
		[]string{"query"},
	)
)

func init() {
	prom.MustRegister(dbQueries, dbDuration)
}

func DBExecObserver(query string, fn func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		timer := prom.NewTimer(dbDuration.WithLabelValues(query))
		err := fn(ctx)
		timer.ObserveDuration()
		result := "success"
		if err != nil {
			result = "error"
		}
		dbQueries.WithLabelValues(query, result).Inc()
		return err
	}
}

type RowScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

func DBQueryObserver(query string, fn func(context.Context) (RowScanner, error)) func(context.Context) (RowScanner, error) {
	return func(ctx context.Context) (RowScanner, error) {
		timer := prom.NewTimer(dbDuration.WithLabelValues(query))
		rows, err := fn(ctx)
		timer.ObserveDuration()
		result := "success"
		if err != nil {
			result = "error"
		}
		dbQueries.WithLabelValues(query, result).Inc()
		return rows, err
	}
}
