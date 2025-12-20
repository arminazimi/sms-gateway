package metrics

import (
	"context"

	prom "github.com/prometheus/client_golang/prometheus"
)

var (
	workerProcessed = prom.NewCounterVec(
		prom.CounterOpts{
			Name: "worker_events_total",
			Help: "Total worker events processed",
		},
		[]string{"queue", "result"},
	)
	workerDuration = prom.NewHistogramVec(
		prom.HistogramOpts{
			Name:    "worker_event_duration_seconds",
			Help:    "Worker event processing duration",
			Buckets: prom.DefBuckets,
		},
		[]string{"queue"},
	)
)

func init() {
	prom.MustRegister(workerProcessed, workerDuration)
}

func WorkerObserver(queue string, handler func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		timer := prom.NewTimer(workerDuration.WithLabelValues(queue))
		err := handler(ctx)
		timer.ObserveDuration()
		result := "success"
		if err != nil {
			result = "error"
		}
		workerProcessed.WithLabelValues(queue, result).Inc()
		return err
	}
}

func WorkerProcessed(queue, result string) {
	workerProcessed.WithLabelValues(queue, result).Inc()
}
