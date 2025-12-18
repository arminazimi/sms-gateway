package app

import (
	"log/slog"
	"os"
	"sms-gateway/config"
	"sms-gateway/pkg/db"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
)

var (
	Echo   *echo.Echo
	Logger *slog.Logger
	DB     *db.DB
)

func Init() {
	initLogger()
	initDB()
	initEcho()
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
	//TODO: migrate
}

func initEcho() {
	Echo = echo.New()
}
