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

	"github.com/rabbitmq/amqp091-go"
)

func StartConsumers(ctx context.Context) error {
	wrap := func(queue string, fn func(context.Context, amqp091.Delivery) error) func(context.Context, amqp091.Delivery) error {
		return func(c context.Context, d amqp091.Delivery) error {
			return metrics.WorkerObserver(queue, func(innerCtx context.Context) error {
				return fn(innerCtx, d)
			})(c)
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
