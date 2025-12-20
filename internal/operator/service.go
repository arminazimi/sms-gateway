package operator

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"sms-gateway/internal/model"
	operatorA "sms-gateway/internal/operator/operatorA"
	operatorB "sms-gateway/internal/operator/operatorB"
	"sms-gateway/pkg/circuitbreaker"
	"sms-gateway/pkg/metrics"
)

type Operator interface {
	Send(ctx context.Context, s model.SMS) error
}

var (
	primaryOperator  Operator = operatorA.OA{}
	fallbackOperator Operator = operatorB.OB{}

	operatorBreaker = circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		OpenTimeout:      5 * time.Second,
	})

	retryBackoff    = 200 * time.Millisecond
	maxSendRetries  = 2
	operatorTimeout = 2 * time.Second
)

func Send(ctx context.Context, s model.SMS) (string, error) {
	if provider, err := dispatch(ctx, "operatorA", primaryOperator, operatorBreaker, s); err == nil {
		return provider, nil
	}

	slog.Warn("primary operator failed, falling back")

	return dispatch(ctx, "operatorB", fallbackOperator, nil, s)
}

func dispatch(ctx context.Context, name string, op Operator, breaker *circuitbreaker.Breaker, s model.SMS) (string, error) {
	if breaker != nil {
		if err := breaker.Allow(); err != nil {
			return "", err
		}
	}

	wrap := func(call func(context.Context) error) func(context.Context) error {
		return metrics.OperatorObserver(name, call)
	}

	var lastErr error
	backoff := retryBackoff

	for attempt := 0; attempt <= maxSendRetries; attempt++ {
		sendCtx, cancel := context.WithTimeout(ctx, operatorTimeout)
		err := wrap(func(c context.Context) error { return op.Send(c, s) })(sendCtx)
		cancel()

		if err == nil {
			if breaker != nil {
				breaker.MarkSuccess()
			}
			return name, nil
		}

		lastErr = err
		if breaker != nil {
			breaker.MarkFailure()
		}

		if attempt == maxSendRetries {
			break
		}

		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if lastErr == nil {
		return "", fmt.Errorf("%s failed without an explicit error", name)
	}

	return "", lastErr
}
