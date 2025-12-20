package main

import (
	"context"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/sms"

	_ "sms-gateway/docs"

	"github.com/rabbitmq/amqp091-go"
	echSwagger "github.com/swaggo/echo-swagger"
)

// @title           SMS Gateway API
// @version         1.0
// @description     Simple SMS gateway with balance management and operator failover.
// @host            localhost:8080
// @BasePath        /
func main() {
	app.Init()

	CreateHermesAndIVRQueue()

	// Consumers
	if err := sms.StartConsumers(context.Background()); err != nil {
		panic(err)
	}

	// Handlers
	// sms
	app.Echo.POST("/sms/send", sms.SendHandler)
	app.Echo.GET("/sms/history", sms.HistoryHandler)

	// balance
	app.Echo.GET("/balance", balance.GetBalanceAndHistoryHandler)
	app.Echo.POST("/balance/add", balance.AddBalanceHandler)

	// swagger
	app.Echo.GET("/swagger/*", echSwagger.WrapHandler)

	if err := app.Echo.Start(config.AppListenAddr); err != nil {
		panic(err)
	}
}

func CreateHermesAndIVRQueue() {
	conn, err := amqp091.DialConfig(config.RabbitmqUri, amqp091.Config{
		Properties: amqp091.NewConnectionProperties(),
	})

	if err != nil {
		panic(err)
	}

	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = ch.Close()
		_ = conn.Close()
	}()

	if err = ch.ExchangeDeclare(config.SmsExchange, "direct", true, false, false, false, amqp091.Table{}); err != nil {
		panic(err)
	}

	// EXPRESS
	if _, err = ch.QueueDeclare(config.ExpressQueue, true, false, false, false, amqp091.Table{}); err != nil {
		panic(err)
	}

	if err = ch.QueueBind(
		config.ExpressQueue,
		config.ExpressQueue,
		config.SmsExchange, false, amqp091.Table{}); err != nil {
		panic(err)
	}

	// Normal
	if _, err = ch.QueueDeclare(config.NormalQueue, true, false, false, false, amqp091.Table{}); err != nil {
		panic(err)
	}

	if err = ch.QueueBind(
		config.NormalQueue,
		config.NormalQueue,
		config.SmsExchange, false, amqp091.Table{}); err != nil {
		panic(err)
	}
}
