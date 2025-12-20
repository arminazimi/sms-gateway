package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/model"
	"sms-gateway/pkg/metrics"
	amqp "sms-gateway/pkg/queue"
	"sms-gateway/pkg/tracing"

	"github.com/rabbitmq/amqp091-go"
)

func StartConsumers(ctx context.Context) error {
	tracer := tracing.Tracer()
	wrap := func(queue string, fn func(context.Context, amqp091.Delivery) error) func(context.Context, amqp091.Delivery) error {
		return func(c context.Context, d amqp091.Delivery) error {
			ctxWithSpan, span := tracer.Start(c, "consumer.process", tracing.WithAttributes(
				tracing.Attr("queue", queue),
				tracing.Attr("routing_key", d.RoutingKey),
			))
			defer span.End()
			return metrics.WorkerObserver(queue, func(innerCtx context.Context) error {
				var msg model.SMS
				if err := json.Unmarshal(d.Body, &msg); err == nil {
					innerCtx = tracing.WithUser(innerCtx, fmt.Sprint(msg.CustomerID))
				}
				return fn(innerCtx, d)
			})(ctxWithSpan)
		}
	}

	expressConsumer := amqp.MakeConsumerWithWorkers(
		config.AppName,
		config.RabbitmqUri,
		config.ExpressQueue,
		config.ExpressQueue,
		config.SmsExchange,
		10,
		wrap(config.ExpressQueue, func(ctx context.Context, evt amqp091.Delivery) error {
			defer func() {
				if err := evt.Ack(false); err != nil {
					app.Logger.Error("ack failed", "err", err)
				}
			}()
			var message model.SMS
			if err := json.Unmarshal(evt.Body, &message); err != nil {
				app.Logger.Error("cannot unmarshal sms", "error", err)
				return fmt.Errorf("cannot unmarshal sms : %w", err)
			}

			app.Logger.Info("got message", "message", message)

			if err := sendSms(ctx, message); err != nil {
				return err
			}

			return nil
		}),
		0,
	)

	normalConsumer := amqp.MakeConsumerWithWorkers(
		config.AppName,
		config.RabbitmqUri,
		config.NormalQueue,
		config.NormalQueue,
		config.SmsExchange,
		10,
		wrap(config.NormalQueue, func(ctx context.Context, evt amqp091.Delivery) error {
			defer func() {
				if err := evt.Ack(false); err != nil {
					app.Logger.Error("ack failed", "err", err)
				}
			}()
			var message model.SMS
			if err := json.Unmarshal(evt.Body, &message); err != nil {
				app.Logger.Error("cannot unmarshal sms", "error", err)
				return fmt.Errorf("cannot unmarshal sms : %w", err)
			}

			app.Logger.Info("got message", "message", message)

			if err := sendSms(ctx, message); err != nil {
				return err
			}

			return nil
		}),
		0,
	)

	go func() {
		_ = normalConsumer(ctx)
	}()

	go func() {
		_ = expressConsumer(ctx)
	}()

	<-ctx.Done()

	return nil
}
