package env

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

var dotEnvMap map[string]string

func init() {
	// Try to read .env if present; do not panic when missing.
	if info, err := os.Stat(".env"); err == nil && !info.IsDir() {
		if m, err := godotenv.Read(".env"); err == nil {
			dotEnvMap = m
		}
	}
	if dotEnvMap == nil {
		dotEnvMap = map[string]string{}
	}
}

func getEnv(key string) string {
	// OS environment has priority over .env
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dotEnvMap[key]
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
