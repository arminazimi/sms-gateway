package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/model"
	amqp "sms-gateway/pkg/queue"

	"github.com/rabbitmq/amqp091-go"
)

func StartConsumers(ctx context.Context) error {
	expressConsumer := amqp.MakeConsumerWithWorkers(
		config.AppName,
		config.RabbitmqUri,
		config.ExpressQueue,
		config.ExpressQueue,
		config.SmsExchange,
		10,
		func(ctx context.Context, evt amqp091.Delivery) error {
			var message model.SMS
			if err := json.Unmarshal(evt.Body, &message); err != nil {
				app.Logger.Error("cannot unmarshal sms", "error", err)
				return fmt.Errorf("cannot unmarshal sms : %w", err)
			}

			app.Logger.Info("got message", "message", message)

			if err := sendSmsToProvider(ctx, message); err != nil {
				return err
			}

			return nil
		},
		0,
	)

	normalConsumer := amqp.MakeConsumerWithWorkers(
		config.AppName,
		config.RabbitmqUri,
		config.NormalQueue,
		config.NormalQueue,
		config.SmsExchange,
		10,
		func(ctx context.Context, evt amqp091.Delivery) error {
			var message model.SMS
			if err := json.Unmarshal(evt.Body, &message); err != nil {
				app.Logger.Error("cannot unmarshal sms", "error", err)
				return fmt.Errorf("cannot unmarshal sms : %w", err)
			}

			app.Logger.Info("got message", "message", message)

			if err := sendSmsToProvider(ctx, message); err != nil {
				return err
			}

			return nil
		},
		0,
	)

	if err := normalConsumer(ctx); err != nil {
		return err
	}

	if err := expressConsumer(ctx); err != nil {
		return err
	}

	return nil
}
