package testutil

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/pkg/db"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
)

var (
	setupOnce sync.Once
	setupCtx  context.Context
)

func SetupAppTest(t *testing.T) (context.Context, func()) {
	t.Helper()
	ctx := context.Background()

	// change to repo root so migrations resolve
	wd, _ := os.Getwd()
	repoRoot := filepath.Join(wd, "..", "..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	cleanupChdir := func() { _ = os.Chdir(wd) }

	// base env defaults
	_ = os.Setenv("LISTEN_ADDR", ":0")
	_ = os.Setenv("DB_USER_NAME", "sms_user")
	_ = os.Setenv("DB_PASSWORD", "sms_pass")
	_ = os.Setenv("DB_HOST", "localhost")
	_ = os.Setenv("DB_PORT", "3306")
	_ = os.Setenv("DB_NAME", "sms_gateway")
	_ = os.Setenv("RABBIT_URI", "amqp://rabbit_user:rabbit_pass@localhost:5672/")
	_ = os.Setenv("RABBIT_SMS_EXCHANGE", "sms_exchange")
	_ = os.Setenv("EXPRESS_QUEUE", "sms_express")
	_ = os.Setenv("NORMAL_QUEUE", "sms_normal")

	mysqlC, host, port := MySQL(ctx, t)

	// override to container values and load config
	_ = os.Setenv("DB_HOST", host)
	_ = os.Setenv("DB_PORT", fmt.Sprintf("%d", port))
	config.Init()

	var err error
	app.DB, err = db.ConnectDB(db.Config{
		Username: config.DBUsername,
		Password: config.DBPassword,
		Host:     config.DBHost,
		Port:     config.DBPort,
		DBName:   config.DBName,
	})
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	if err := db.MigrateFromFile(app.DB, "db/db.sql"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	app.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	app.Echo = echo.New()

	cleanup := func() {
		_ = app.DB.Close()
		_ = mysqlC.Terminate(ctx)
		cleanupChdir()
	}

	setupCtx = ctx

	return ctx, cleanup
}

func EnsureSetup(t *testing.T) context.Context {
	t.Helper()
	setupOnce.Do(func() {
		c, _ := SetupAppTest(t)
		setupCtx = c
		// intentionally no cleanup to keep DB alive across tests
	})
	return setupCtx
}

func ResetTables(ctx context.Context, t *testing.T) {
	t.Helper()
	if _, err := app.DB.ExecContext(ctx, "DELETE FROM user_transactions"); err != nil {
		t.Fatalf("truncate user_transactions: %v", err)
	}
	if _, err := app.DB.ExecContext(ctx, "DELETE FROM user_balances"); err != nil {
		t.Fatalf("truncate user_balances: %v", err)
	}
}
