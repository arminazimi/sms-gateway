package app

import (
	"context"
	"log/slog"
	"os"
	"sms-gateway/config"
	"sms-gateway/pkg/db"
	amqp "sms-gateway/pkg/queue"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
)

var (
	Echo   *echo.Echo
	Logger *slog.Logger
	DB     *db.DB
	Rabbit *amqp.RabbitConnection
)

func Init() {
	initLogger()
	initDB()
	initEcho()
	iniRabbit()
}

func initLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	Logger = slog.New(handler)
}

func initDB() {
	var err error
	DB, err = db.ConnectDB(db.Config{
		Username: config.DBUsername,
		Password: config.DBPassword,
		Host:     config.DBHost,
		Port:     config.DBPort,
		DBName:   config.DBName,
	})
	if err != nil {
		panic(err)
	}
	if err := db.MigrateFromFile(DB, "db/db.sql"); err != nil {
		panic(err)
	}
}

func iniRabbit() {
	var err error
	Rabbit, err = amqp.NewRabbitConnection(config.RabbitmqUri)
	if err != nil {
		panic(err)
	}
	if err := amqp.SetupQueues(context.Background(), amqp.QueueSetup{
		URI:      config.RabbitmqUri,
		Exchange: config.SmsExchange,
		Bindings: []amqp.QueueBinding{
			{Queue: config.ExpressQueue, RoutingKey: config.ExpressQueue},
			{Queue: config.NormalQueue, RoutingKey: config.NormalQueue},
		},
	}); err != nil {
		panic(err)
	}
}

func initEcho() {
	Echo = echo.New()
}
