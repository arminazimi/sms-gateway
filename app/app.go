package app

import (
	"context"
	"log/slog"
	"os"
	"sms-gateway/config"
	"sms-gateway/pkg/db"
	"sms-gateway/pkg/metrics"
	amqp "sms-gateway/pkg/queue"
	"sms-gateway/pkg/tracing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
)

var (
	Echo          *echo.Echo
	Logger        *slog.Logger
	DB            *db.DB
	Rabbit        *amqp.RabbitConnection
	TraceShutdown func(context.Context) error
)

func Init() {
	config.Init()
	initLogger()
	initTracing()
	initDB()
	initEcho()
	iniRabbit()
}

func initLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	Logger = slog.New(handler)
}

func initTracing() {
	shutdown, err := tracing.Init(context.Background(), config.AppName)
	if err != nil {
		Logger.Error("tracing init failed", "err", err)
		return
	}
	TraceShutdown = shutdown
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
	Echo.HideBanner = true
	Echo.HidePort = true
	Echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, span := tracing.Start(c.Request().Context(), "http.request",
				tracing.Attr("path", c.Path()),
				tracing.Attr("method", c.Request().Method),
			)
			defer span.End()
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	Echo.Use(metrics.EchoMiddleware())
}

func Shutdown() {
	if Rabbit != nil {
		if err := Rabbit.Close(); err != nil {
			Logger.Error("failed to close rabbit", "err", err)
		}
	}

	if DB != nil {
		if err := DB.Close(); err != nil {
			Logger.Error("failed to close db", "err", err)
		}
	}

	if TraceShutdown != nil {
		if err := TraceShutdown(context.Background()); err != nil {
			Logger.Error("failed to shutdown tracing", "err", err)
		}
	}
}
