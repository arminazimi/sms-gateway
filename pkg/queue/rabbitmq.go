package amqp

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"sms-gateway/pkg/metrics"
	"sms-gateway/pkg/tracing"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

type RabbitConnection struct {
	Conn *amqp091.Connection
}

func NewRabbitConnection(rabbitURI string) (*RabbitConnection, error) {
	cfg := amqp091.Config{
		Properties: amqp091.NewConnectionProperties(),
	}

	conn, err := amqp091.DialConfig(rabbitURI, cfg)
	if err != nil {
		slog.Error("cannot connect to rabbit", "err", err)
		return nil, err
	}

	return &RabbitConnection{Conn: conn}, nil
}

type PublishRequest struct {
	Exchange string
	Key      string
	Msg      []byte
}

func (rp *RabbitConnection) PublishContext(ctx context.Context, req PublishRequest) error {
	ctx, span := tracing.Start(ctx, "rabbit.publish",
		tracing.Attr("exchange", req.Exchange),
		tracing.Attr("routing_key", req.Key),
		tracing.Attr("user_id", tracing.UserIDFromContext(ctx)),
	)
	defer span.End()

	ch, err := rp.Conn.Channel()
	if err != nil {
		slog.Error("cannot create channel from rabbit mq connection", "err", err)
		return err
	}
	defer func() {
		err := ch.Close()
		if err != nil {
			slog.Error("cannot close channel from rabbit mq connection", "err", err)
		}
	}()

	publishFn := metrics.OperatorObserver("rabbit_publish", func(c context.Context) error {
		return ch.PublishWithContext(c, req.Exchange, req.Key, false, false, amqp091.Publishing{
			Timestamp: time.Now(),
			Body:      req.Msg,
		})
	})

	if err = publishFn(ctx); err != nil {
		return err
	}
	return nil
}

func (rp *RabbitConnection) ConsumeContext(ctx context.Context, appName string, queueName string, routingKey string, exchangeName string, prefetch int,
) (<-chan amqp091.Delivery, error) {
	ctx, span := tracing.Start(ctx, "rabbit.consume",
		tracing.Attr("queue", queueName),
		tracing.Attr("routing_key", routingKey),
		tracing.Attr("user_id", tracing.UserIDFromContext(ctx)),
	)
	defer span.End()

	ch, err := rp.Conn.Channel()
	if err != nil {
		slog.Error("cannot create channel from rabbit mq connection", "err", err)
		return nil, err
	}

	_, err = ch.QueueDeclare(queueName, true, false, false, false, amqp091.Table{})
	if err != nil {
		slog.Error("cannot declare rabbit queue", "err", err)
		return nil, err
	}

	err = ch.QueueBind(queueName, routingKey, exchangeName, false, amqp091.Table{})
	if err != nil {
		slog.Error("cannot bind rabbit queue", "err", err)
		return nil, err
	}
	if prefetch != 0 {
		err = ch.Qos(
			prefetch, // prefetch count
			0,        // prefetch size
			false,    // global
		)

		if err != nil {
			slog.Error("cannot set qos (prefetch) rabbit queue", "err", err)
			return nil, err
		}
	}

	delivery, err := ch.ConsumeWithContext(ctx, queueName, fmt.Sprintf("consumer-%s-%s", appName, uuid.NewString()),
		false,
		false,
		false,
		false,
		amqp091.Table{})
	if err != nil {
		slog.Error("cannot consume rabbit queue", "err", err)
		return nil, err
	}

	wrapped := make(chan amqp091.Delivery)
	go func() {
		defer close(wrapped)
		for d := range delivery {
			metrics.WorkerProcessed(queueName, "received")
			wrapped <- d
		}
	}()

	return wrapped, nil
}

func (rp *RabbitConnection) Close() error {
	if rp == nil || rp.Conn == nil {
		return nil
	}
	return rp.Conn.Close()
}
