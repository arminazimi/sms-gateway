package metrics

import (
	"context"

	prom "github.com/prometheus/client_golang/prometheus"
)

var (
	operatorCalls = prom.NewCounterVec(
		prom.CounterOpts{
			Name: "operator_calls_total",
			Help: "Count of operator send attempts",
		},
		[]string{"operator", "result"},
	)
	operatorDuration = prom.NewHistogramVec(
		prom.HistogramOpts{
			Name:    "operator_duration_seconds",
			Help:    "Duration of operator send attempts",
			Buckets: prom.DefBuckets,
		},
		[]string{"operator"},
	)
)

func init() {
	prom.MustRegister(operatorCalls, operatorDuration)
}

func OperatorObserver(name string, fn func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		timer := prom.NewTimer(operatorDuration.WithLabelValues(name))
		err := fn(ctx)
		timer.ObserveDuration()
		result := "success"
		if err != nil {
			result = "error"
		}
		operatorCalls.WithLabelValues(name, result).Inc()
		return err
	}
}
