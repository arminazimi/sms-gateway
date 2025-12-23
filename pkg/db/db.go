package db

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/jmoiron/sqlx"
)

type Config struct {
	Username string
	Password string
	Host     string
	Port     int
	DBName   string
}

type DB struct {
	*sqlx.DB
}

func ConnectionString(dbConfig Config) string {
	// parseTime=true is required so DATETIME/TIMESTAMP scan into time.Time instead of []uint8.
	// loc=Local keeps times consistent with the host/container locale.
	// charset/collation are set for safe UTF-8 text handling.
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DBName)
	return url
}

func ConnectDB(config Config) (*DB, error) {
	db, err := sqlx.Open("mysql", ConnectionString(Config{
		Username: config.Username,
		Password: config.Password,
		Host:     config.Host,
		Port:     config.Port,
		DBName:   config.DBName,
	}))
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}
