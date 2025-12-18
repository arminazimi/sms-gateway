package main

import (
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/sms"
)

func main() {
	app.Init()

	app.Echo.POST("/sms/send", sms.SendHandler)
	app.Echo.GET("/balance", balance.GetBalanceAndHistory)

	if err := app.Echo.Start(config.AppListenAddr); err != nil {
		panic(err)
	}
}
