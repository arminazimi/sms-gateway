package amqp

import (
	"context"

	"github.com/rabbitmq/amqp091-go"
)

type QueueBinding struct {
	Queue      string
	RoutingKey string
}

type QueueSetup struct {
	URI      string
	Exchange string
	Bindings []QueueBinding
}

func SetupQueues(ctx context.Context, cfg QueueSetup) error {
	conn, err := amqp091.DialConfig(cfg.URI, amqp091.Config{Properties: amqp091.NewConnectionProperties()})
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := conn.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := ch.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if err = ch.ExchangeDeclare(cfg.Exchange, "direct", true, false, false, false, amqp091.Table{}); err != nil {
		return err
	}

	for _, b := range cfg.Bindings {
		if _, err = ch.QueueDeclare(b.Queue, true, false, false, false, amqp091.Table{}); err != nil {
			return err
		}
		if err = ch.QueueBind(b.Queue, b.RoutingKey, cfg.Exchange, false, amqp091.Table{}); err != nil {
			return err
		}
	}

	return nil
}
