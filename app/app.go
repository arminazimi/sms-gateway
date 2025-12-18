package app

import (
	"fmt"
	"log/slog"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

var (
	Echo   *echo.Echo
	Logger *slog.Logger
	DB     *sqlx.DB
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

type dbConfig struct {
	Username string
	Password string
	Host     string
	Port     int
	DBName   string
}

func ConnectionString(dbConfig dbConfig) string {
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DBName)
	return url
}

func initDB() {
	var err error
	DB, err = sqlx.Open("mysql", ConnectionString(dbConfig{
		Username: "sms_user",
		Password: "sms_pass",
		Host:     "localhost",
		Port:     3306,
		DBName:   "sms_gateway",
	}))
	if err != nil {
		panic(err)
	}

	if err := DB.Ping(); err != nil {
		panic(err)
	}

	// migrate

}

func initEcho() {
	Echo = echo.New()
}
