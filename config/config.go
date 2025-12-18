package config

import "sms-gateway/pkg/env"

var (
	AppName       string
	AppBaseURL    string
	AppListenAddr string
)

func init() {
	AppName = env.Default("APP_NAME", "sms-gateway")
	AppListenAddr = env.RequiredNotEmpty("LISTEN_ADDR")
}
