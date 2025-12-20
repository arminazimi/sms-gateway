package main

import (
	"context"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/sms"

	_ "sms-gateway/docs"

	echSwagger "github.com/swaggo/echo-swagger"
)

// @title           SMS Gateway API
// @version         1.0
// @description     Simple SMS gateway with balance management and operator failover.
// @host            localhost:8080
// @BasePath        /
func main() {
	app.Init()

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
