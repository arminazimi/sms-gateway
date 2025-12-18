package env

import (
	"fmt"
	"os"

	"testing"

	"github.com/joho/godotenv"
)

var dotEnvMap map[string]string

func init() {
	var err error
	dotEnvMap, err = godotenv.Read(".env")
	if err != nil {
		panic(err)
	}
}

func getEnv(key string) string {
	// .env
	value := dotEnvMap[key]
	// os.Getenv
	if v := os.Getenv(key); v != "" {
		value = v
	}
	return value
}

func Default(key, def string) string {
	value := getEnv(key)
	if value == "" {
		return def
	}
	return value
}

func RequiredNotEmpty(key string) string {
	value := getEnv(key)
	if value == "" {
		if !testing.Testing() {
			panic(fmt.Sprintf("`%s` is not set or is empty", key))
		}
	}
	return value
}

func Required(key string) string {
	_, osSet := os.LookupEnv(key)
	_, dotEnvSet := dotEnvMap[key]
	if !osSet && !dotEnvSet {
		if !testing.Testing() {
			panic(fmt.Sprintf("`%s` is not set", key))
		}
	}
	return getEnv(key)
}
