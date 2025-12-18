package main

import (
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/sms"
)

func main() {
	app.Init()

	// sms
	app.Echo.POST("/sms/send", sms.SendHandler)

	// balance
	app.Echo.GET("/balance", balance.GetBalanceAndHistoryHandler)
	app.Echo.POST("/balance/add", balance.AddBalanceHandler)

	if err := app.Echo.Start(config.AppListenAddr); err != nil {
		panic(err)
	}
}
