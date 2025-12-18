package main

import (
	"fmt"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/sms"
)

func main() {
	app.Init()

	app.Echo.POST("/sms/send", sms.SendHandler)

	fmt.Printf("Starting server on %s ...\n", config.AppListenAddr)
	if err := app.Echo.Start(config.AppListenAddr); err != nil {
		panic(err)
	}
}
