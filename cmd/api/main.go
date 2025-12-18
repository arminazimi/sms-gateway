package main

import (
	"fmt"
	"sms-gateway/app"
	"sms-gateway/config"
)

func main() {
	app.Init()

	fmt.Printf("Starting server on %s ...\n", config.AppListenAddr)
	if err := app.Echo.Start(config.AppListenAddr); err != nil {
		panic(err)
	}
}
