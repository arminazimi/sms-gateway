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
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
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
