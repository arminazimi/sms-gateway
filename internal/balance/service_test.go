package balance

import (
	"sms-gateway/testutil"
	"testing"

	"sms-gateway/app"
	"sms-gateway/internal/model"
)

func TestUserHasBalance(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	testutil.ResetTables(ctx, t)
	_, err := app.DB.ExecContext(ctx, "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 101, 10)
	if err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	ok, err := UserHasBalance(ctx, UserHasEnoughBalanceRequest{CustomerID: 101, Quantity: 2, Type: model.NORMAL})
	if err != nil || !ok {
		t.Fatalf("expected sufficient balance, err=%v ok=%v", err, ok)
	}
	ok, err = UserHasBalance(ctx, UserHasEnoughBalanceRequest{CustomerID: 102, Quantity: 1, Type: model.NORMAL})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatalf("expected insufficient balance")
	}
}

func TestDeductBalance(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	testutil.ResetTables(ctx, t)
	_, err := app.DB.ExecContext(ctx, "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 201, 5)
	if err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	txID, err := DeductBalance(ctx, DeductBalanceRequest{CustomerID: 201, Quantity: 1, Type: model.NORMAL})
	if err != nil {
		t.Fatalf("deduct: %v", err)
	}
	if txID == "" {
		t.Fatalf("expected tx id")
	}
	bal, _ := GetUserBalance(ctx, "201")
	if bal != 4 {
		t.Fatalf("expected balance 4, got %d", bal)
	}
	txs, _ := GetUserTransactions(ctx, "201")
	if len(txs) != 1 || txs[0].TransactionID == "" {
		t.Fatalf("expected one withdrawal tx, got %+v", txs)
	}
}

func TestGetUserTransactions(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	testutil.ResetTables(ctx, t)
	_, err := app.DB.ExecContext(ctx, "INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)", 301, 5, "deposit", "desc", "tx1")
	if err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	txs, err := GetUserTransactions(ctx, "301")
	if err != nil {
		t.Fatalf("get tx: %v", err)
	}
	if len(txs) != 1 || txs[0].TransactionID != "tx1" {
		t.Fatalf("unexpected txs: %+v", txs)
	}
}

func TestGetUserBalance(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	testutil.ResetTables(ctx, t)
	_, err := app.DB.ExecContext(ctx, "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 401, 7)
	if err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	bal, err := GetUserBalance(ctx, "401")
	if err != nil {
		t.Fatalf("get balance: %v", err)
	}
	if bal != 7 {
		t.Fatalf("unexpected balance %d", bal)
	}
}

func TestRefund(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	testutil.ResetTables(ctx, t)
	_, err := app.DB.ExecContext(ctx, "INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)", 501, -2, "withdrawal", "charge", "tx123")
	if err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	_, err = app.DB.ExecContext(ctx, "INSERT INTO user_balances (user_id, balance) VALUES (?, ?)", 501, 0)
	if err != nil {
		t.Fatalf("seed balance: %v", err)
	}
	if err := Refund(ctx, model.SMS{CustomerID: 501, TransactionID: "tx123"}); err != nil {
		t.Fatalf("refund: %v", err)
	}
	bal, _ := GetUserBalance(ctx, "501")
	if bal != 2 {
		t.Fatalf("expected balance 2, got %d", bal)
	}
	txs, _ := GetUserTransactions(ctx, "501")
	if len(txs) < 2 {
		t.Fatalf("expected at least 2 transactions, got %+v", txs)
	}
	foundCorrective := false
	for _, tx := range txs {
		if tx.TransactionType == CorrectiveTransaction {
			foundCorrective = true
			break
		}
	}
	if !foundCorrective {
		t.Fatalf("expected corrective transaction, got %+v", txs)
	}
}

func TestAddBalance(t *testing.T) {
	ctx := testutil.EnsureSetup(t)
	testutil.ResetTables(ctx, t)
	if err := AddBalance(ctx, AddBalanceRequest{CustomerID: 601, Amount: 10}); err != nil {
		t.Fatalf("add balance: %v", err)
	}
	bal, _ := GetUserBalance(ctx, "601")
	if bal != 10 {
		t.Fatalf("expected balance 10, got %d", bal)
	}
	txs, _ := GetUserTransactions(ctx, "601")
	if len(txs) != 1 || txs[0].TransactionType != Deposit {
		t.Fatalf("expected one deposit tx, got %+v", txs)
	}
}

func TestCalculatePrice(t *testing.T) {
	if v := calculatePrice(model.EXPRESS, 2); v != 6 {
		t.Fatalf("expected express price 6 got %d", v)
	}
	if v := calculatePrice(model.NORMAL, 3); v != 3 {
		t.Fatalf("expected normal price 3 got %d", v)
	}
}

func TestGetPricePerType(t *testing.T) {
	if getPricePerType(model.EXPRESS) != 3 {
		t.Fatalf("express price mismatch")
	}
	if getPricePerType(model.NORMAL) != 1 {
		t.Fatalf("normal price mismatch")
	}
}
