package sms

import (
	"bytes"
	"context"
	"log/slog"
	testutil2 "sms-gateway/testutil"

	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/model"
	amqp "sms-gateway/pkg/queue"

	"github.com/labstack/echo/v4"
)

func initTestLogger() {
	if app.Logger == nil {
		app.Logger = slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	}
}

func startApp(t *testing.T) func() {
	t.Helper()
	ctx := context.Background()

	appCtx, cleanupApp := testutil2.SetupAppTest(t)
	_ = appCtx // appContext used by DB

	rabbitC, host, port := testutil2.Rabbit(ctx, t)
	uri := fmt.Sprintf("amqp://rabbit_user:rabbit_pass@%s:%d/", host, port)
	_ = os.Setenv("RABBIT_URI", uri)

	conn, err := amqp.NewRabbitConnection(uri)
	if err != nil {
		t.Fatalf("rabbit connect: %v", err)
	}
	app.Rabbit = conn

	return func() {
		_ = conn.Close()
		_ = rabbitC.Terminate(ctx)
		cleanupApp()
	}
}

func TestSendHandler_InvalidJSON(t *testing.T) {
	initTestLogger()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := SendHandler(ctx); err == nil {
		t.Fatalf("expected error for invalid json")
	} else if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestSendHandler_ZeroRecipients(t *testing.T) {
	initTestLogger()
	e := echo.New()
	body := `{"customer_id":1,"recipients":[],"type":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := SendHandler(ctx)
	if err == nil {
		t.Fatalf("expected error for zero recipients")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestHistoryHandler_MissingUserID(t *testing.T) {
	initTestLogger()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sms/history", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := HistoryHandler(ctx)
	if err == nil {
		t.Fatalf("expected error when user_id missing")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestSendHandler_BalanceError(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	// Close DB to force error on balance check
	_ = app.DB.Close()

	e := echo.New()
	body := `{"customer_id":1,"recipients":["+1"],"type":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := SendHandler(ctx)
	if err == nil {
		t.Fatalf("expected error")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %v", err)
	}
}

func TestSendHandler_NotEnoughBalance(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	// seed low balance
	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_transactions")
	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_balances")
	_, _ = app.DB.ExecContext(context.Background(), "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 1, 1)

	e := echo.New()
	body := `{"customer_id":1,"recipients":["+1","+2"],"type":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := SendHandler(ctx)
	if err == nil {
		t.Fatalf("expected error")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %v", err)
	}
}

func TestSendHandler_DeductError(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	// remove balance table to force deduct error after balance check passes
	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_transactions")
	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_balances")
	_, _ = app.DB.ExecContext(context.Background(), "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 1, 1000)
	_, _ = app.DB.ExecContext(context.Background(), "DROP TABLE user_balances")

	e := echo.New()
	body := `{"customer_id":1,"recipients":["+1"],"type":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := SendHandler(ctx)
	if err == nil {
		t.Fatalf("expected error")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %v", err)
	}
}

func TestSendHandler_PublishError(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_transactions")
	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_balances")
	_, _ = app.DB.ExecContext(context.Background(), "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 1, 1000)

	// Handler no longer publishes to Rabbit directly (outbox pattern).
	// Force failure by breaking outbox insert.
	_, _ = app.DB.ExecContext(context.Background(), "DROP TABLE outbox_events")

	e := echo.New()
	body := `{"customer_id":1,"recipients":["+1"],"type":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := SendHandler(ctx)
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %v", err)
	}
}

func TestSendHandler_Success(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_transactions")
	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM user_balances")
	_, err := app.DB.ExecContext(context.Background(), "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 1, 1000)
	if err != nil {
		t.Fatalf("seed balance: %v", err)
	}

	e := echo.New()
	body := `{"customer_id":1,"recipients":["+1"],"type":"normal"}`
	req := httptest.NewRequest(http.MethodPost, "/sms/send", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := SendHandler(ctx); err != nil {
		t.Fatalf("send handler err: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHistoryHandler_Error(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	_ = app.DB.Close()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sms/history?user_id=1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := HistoryHandler(ctx)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestHistoryHandler_Success(t *testing.T) {
	initTestLogger()
	cleanup := startApp(t)
	t.Cleanup(cleanup)

	_, _ = app.DB.ExecContext(context.Background(), "DELETE FROM sms_status")
	_, err := app.DB.ExecContext(context.Background(), `INSERT INTO sms_status (user_id,type,status,recipient,provider,sms_identifier,created_at,updated_at) VALUES (?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, 5, model.NORMAL, Pending, "+1", "operatorA", "sid-1")
	if err != nil {
		t.Fatalf("seed history: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/sms/history?user_id=5&status=pending&sms_identifier=sid-1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := HistoryHandler(ctx); err != nil {
		t.Fatalf("history err: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetQueue(t *testing.T) {
	if q := getQueue(model.NORMAL); q != config.NormalQueue {
		t.Fatalf("unexpected queue for normal: %s", q)
	}
	if q := getQueue(model.EXPRESS); q != config.ExpressQueue {
		t.Fatalf("unexpected queue for express: %s", q)
	}
	if q := getQueue(model.Type("unknown")); q != config.NormalQueue {
		t.Fatalf("unexpected queue for default: %s", q)
	}
}
