package balance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sms-gateway/testutil"
	"testing"

	"sms-gateway/app"
)

// handlerCtx now comes from main_test.go via testutil.SetupAppTest
// remove local handlerCtx var; rely on HandlerCtx set by ensureSetup

func TestGetBalanceAndHistoryHandler(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	_, err := app.DB.ExecContext(ctx, "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 1, 100)
	if err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	_, err = app.DB.ExecContext(ctx, "INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)", 1, 50, Deposit, "seed", "tx-seed")
	if err != nil {
		t.Fatalf("seed tx: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/balance?user_id=1", nil)
	rec := httptest.NewRecorder()
	c := app.Echo.NewContext(req, rec)

	if err := GetBalanceAndHistoryHandler(c); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !bytes.Contains(rec.Body.Bytes(), []byte("balance")) || !bytes.Contains(rec.Body.Bytes(), []byte("transactions")) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestGetBalanceAndHistoryHandlerMissingUser(t *testing.T) {
	_ = testutil.EnsureSetup(t)
	req := httptest.NewRequest(http.MethodGet, "/balance", nil)
	rec := httptest.NewRecorder()
	c := app.Echo.NewContext(req, rec)

	err := GetBalanceAndHistoryHandler(c)
	if err == nil {
		t.Fatalf("expected error for missing user_id")
	}
}

func TestGetBalanceAndHistoryHandlerCanceledContext(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/balance?user_id=123", nil).WithContext(cancelCtx)
	rec := httptest.NewRecorder()
	c := app.Echo.NewContext(req, rec)

	if err := GetBalanceAndHistoryHandler(c); err == nil {
		t.Fatalf("expected error due to canceled context")
	}
}

func TestAddBalanceHandler(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	payload := AddBalancePayload{UserID: 2, Balance: 20, Description: "test add"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/balance/add", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := app.Echo.NewContext(req, rec)

	if err := AddBalanceHandler(c); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	bal, err := GetUserBalance(ctx, fmt.Sprint(payload.UserID))
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if bal != int64(payload.Balance) {
		t.Fatalf("expected balance %d, got %d", payload.Balance, bal)
	}
}

func TestAddBalanceHandlerInvalid(t *testing.T) {
	_ = testutil.EnsureSetup(t)
	req := httptest.NewRequest(http.MethodPost, "/balance/add", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	c := app.Echo.NewContext(req, rec)

	err := AddBalanceHandler(c)
	if err == nil {
		t.Fatalf("expected error for invalid input")
	}
}

func TestAddBalanceHandlerCanceledContext(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	payload := AddBalancePayload{UserID: 3, Balance: 10, Description: "ctx canceled"}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/balance/add", bytes.NewReader(b)).WithContext(cancelCtx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := app.Echo.NewContext(req, rec)

	if err := AddBalanceHandler(c); err == nil {
		t.Fatalf("expected error due to canceled context")
	}
}
