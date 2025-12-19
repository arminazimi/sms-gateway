package amqp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rabbitmq/amqp091-go"
)

type ConsumerConfig struct {
	AppName              string
	RabbitMQURI          string
	QueueName            string
	RoutingKey           string
	Exchange             string
	WorkerGoRoutineCount int
	DeliveryHandler      func(ctx context.Context, dv amqp091.Delivery) error
	Prefetch             int
}

func MakeConsumerFromConfig(c ConsumerConfig) func(ctx context.Context) error {
	return MakeConsumerWithWorkers(
		c.AppName,
		c.RabbitMQURI,
		c.QueueName,
		c.RoutingKey,
		c.Exchange,
		c.WorkerGoRoutineCount,
		c.DeliveryHandler,
		c.Prefetch,
	)
}

func MakeConsumerWithWorkers(
	appName string,
	rabbitMQURI string,
	queueName string,
	routingKey string,
	exchangeName string,
	workerCount int,
	deliveryHandler func(ctx context.Context, dv amqp091.Delivery) error,
	prefetch int,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var conn *RabbitConnection
		var delivery <-chan amqp091.Delivery
		var amqpCloseNotifyC chan *amqp091.Error

		restartConsumer := func() {
			fmt.Printf("[Re]Starting %s consumer\n", queueName)
			var err error

			conn, err = NewRabbitConnection(rabbitMQURI)
			if err != nil {
				panic(fmt.Sprintf("cannot make a rabbitmq connection: %s", err.Error()))
			}
			amqpCloseNotifyC = make(chan *amqp091.Error, 100)

			conn.Conn.NotifyClose(amqpCloseNotifyC)

			delivery, err = conn.ConsumeContext(ctx,
				appName,
				queueName,
				routingKey,
				exchangeName,
				prefetch,
			)

			if err != nil {
				panic(fmt.Sprintf("Cannot restart %s consumer: %s\n", queueName, err.Error()))
			}

		}

		restartConsumer()

		workerChans := []chan amqp091.Delivery{}

		for i := 0; i < workerCount; i++ {
			thisWorkerChan := make(chan amqp091.Delivery, 20)
			go func(ctx context.Context, id int, c chan amqp091.Delivery) {
				for {
					dv, ok := <-c
					if !ok {
						fmt.Printf("Worker-%d is shutting down.\n", i)
						return
					}

					err := deliveryHandler(ctx, dv)

					if err != nil {
						slog.Error("cannot process delivery",
							"queueName", queueName,
							"err", err,
						)
						continue
					}
				}

			}(ctx, i, thisWorkerChan)
			workerChans = append(workerChans, thisWorkerChan)
		}

		go func() {
			defer conn.Conn.Close()
			var workerIndex int
			for {
				if workerIndex > workerCount-1 {
					workerIndex = 0
				}
				select {
				case <-amqpCloseNotifyC:
					fmt.Printf("notified of a close event on %s.\n", queueName)
					restartConsumer()
				case evt, ok := <-delivery:
					if !ok {
						fmt.Printf("delivery channel is closed for %s.\n", queueName)
						restartConsumer()
					}

					go func(evt amqp091.Delivery) {
						workerChans[workerIndex] <- evt

					}(evt)
					workerIndex++
				}
			}
		}()

		return nil

	}

}
