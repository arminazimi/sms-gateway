package main

import (
	"context"
	"os/signal"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/sms"
	"sms-gateway/pkg/metrics"
	"syscall"
	"time"

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

	// Handlers
	app.Echo.POST("/sms/send", sms.SendHandler)
	app.Echo.GET("/sms/history", sms.HistoryHandler)

	app.Echo.GET("/balance", balance.GetBalanceAndHistoryHandler)
	app.Echo.POST("/balance/add", balance.AddBalanceHandler)

	app.Echo.GET("/swagger/*", echSwagger.WrapHandler)
	app.Echo.GET("/metrics", metrics.Handler())

	// Graceful ShoutDown
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- app.Echo.Start(config.AppListenAddr)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	consumerErrCh := make(chan error, 1)
	go func() {
		consumerErrCh <- sms.StartConsumers(ctx)
	}()

	select {
	case err := <-consumerErrCh:
		if err != nil {
			app.Logger.Error("consumer error", "err", err)
		}
	case err := <-serverErrCh:
		if err != nil {
			app.Logger.Error("server error", "err", err)
		}
	case <-ctx.Done():
		app.Logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Echo.Shutdown(shutdownCtx); err != nil {
		app.Logger.Error("echo shutdown", "err", err)
	}

	stop()
	app.Shutdown()
}
