package config

import (
	"sms-gateway/pkg/env"
	"strconv"
)

var (
	AppName       string
	AppBaseURL    string
	AppListenAddr string
	DBUsername    string
	DBPassword    string
	DBHost        string
	DBPort        int
	DBName        string
)

func init() {
	AppName = env.Default("APP_NAME", "sms-gateway")
	AppListenAddr = env.RequiredNotEmpty("LISTEN_ADDR")
	DBUsername = env.RequiredNotEmpty("DB_USER_NAME")
	DBPassword = env.RequiredNotEmpty("DB_PASSWORD")
	DBHost = env.RequiredNotEmpty("DB_HOST")
	port, err := strconv.Atoi(env.RequiredNotEmpty("DB_PORT"))
	if err != nil {
		panic("invalid DB_PORT: " + err.Error())
	}
	DBPort = port
	DBName = env.RequiredNotEmpty("DB_NAME")
}
