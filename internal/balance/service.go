package balance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sms-gateway/app"
	"sms-gateway/internal/model"
	"sms-gateway/pkg/metrics"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type transactionType string

const (
	Withdrawal            transactionType = "withdrawal"
	Deposit               transactionType = "deposit"
	CorrectiveTransaction transactionType = "Corrective"
)

var ErrInsufficientBalance = errors.New("insufficient balance")

type UserHasEnoughBalanceRequest struct {
	CustomerID int64
	Quantity   int
	Type       model.Type
}

func UserHasBalance(ctx context.Context, req UserHasEnoughBalanceRequest) (bool, error) {

	const query = `SELECT balance FROM user_balances WHERE user_id = ?`
	var balance int64
	if err := app.DB.QueryRowxContext(ctx, query, req.CustomerID).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	price := calculatePrice(req.Type, req.Quantity)
	return balance >= price, nil
}

type ChargeRequest struct {
	CustomerID int64
	Quantity   int
	Type       model.Type
}

// ChargeTx atomically checks and deducts user balance and records a withdrawal transaction
// using the provided DB transaction.
func ChargeTx(ctx context.Context, tx *sqlx.Tx, req ChargeRequest) (string, error) {
	if tx == nil {
		return "", errors.New("tx is required")
	}
	price := calculatePrice(req.Type, req.Quantity)

	const updateBalanceQuery = `UPDATE user_balances SET balance = balance - ? WHERE user_id = ? AND balance >= ?`
	res, err := tx.ExecContext(ctx, updateBalanceQuery, price, req.CustomerID, price)
	if err != nil {
		return "", err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return "", err
	}
	if rows == 0 {
		return "", ErrInsufficientBalance
	}

	txID := uuid.NewString()
	const insertTransactionQuery = `INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)`
	if _, err = tx.ExecContext(ctx,
		insertTransactionQuery,
		req.CustomerID,
		-price,
		Withdrawal,
		fmt.Sprintf("بابت خرید %d پیامک تایپ %s", req.Quantity, req.Type),
		txID); err != nil {
		return "", err
	}

	return txID, nil
}

// Charge runs ChargeTx in its own DB transaction.
func Charge(ctx context.Context, req ChargeRequest) (string, error) {
	tx, err := app.DB.BeginTxx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	txID, err := ChargeTx(ctx, tx, req)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return txID, nil
}

type UserTransaction struct {
	UserID          int64           `db:"user_id" json:"-"`
	Amount          int64           `db:"amount"`
	TransactionType transactionType `db:"transaction_type"`
	Description     string          `db:"description"`
	TransactionID   string          `db:"transaction_id" json:"transaction_id"`
}

func GetUserTransactions(ctx context.Context, userID string) ([]UserTransaction, error) {
	const query = `SELECT user_id, amount, transaction_type, description, transaction_id FROM user_transactions WHERE user_id = ?`

	var transactions []UserTransaction
	queryFn := metrics.DBExecObserver("select_user_transactions", func(c context.Context) error {
		return app.DB.SelectContext(c, &transactions, query, userID)
	})
	if err := queryFn(ctx); err != nil {
		return nil, err
	}

	return transactions, nil
}

func GetUserBalance(ctx context.Context, userID string) (int64, error) {

	const query = `SELECT balance FROM user_balances WHERE user_id = ?`
	var balance int64
	queryFn := metrics.DBExecObserver("select_user_balance", func(c context.Context) error {
		return app.DB.QueryRowxContext(c, query, userID).Scan(&balance)
	})
	if err := queryFn(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return balance, nil
}

func Refund(ctx context.Context, s model.SMS) (err error) {
	if s.TransactionID == "" {
		return errors.New("transaction_id is required for refund")
	}
	if s.CustomerID == 0 {
		return errors.New("customer_id is required for refund")
	}

	tx, err := app.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const selectTxn = `SELECT amount FROM user_transactions WHERE transaction_id = ? AND user_id = ? LIMIT 1`
	var amount int64
	queryFn := metrics.DBExecObserver("select_refund_txn", func(c context.Context) error {
		return tx.QueryRowxContext(c, selectTxn, s.TransactionID, s.CustomerID).Scan(&amount)
	})
	if err = queryFn(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("transaction not found")
		}
		return err
	}

	refundAmount := -1 * amount
	const updateBalance = `UPDATE user_balances SET balance = balance + ? WHERE user_id = ?`
	execUpdate := metrics.DBExecObserver("update_balance_refund", func(c context.Context) error {
		_, execErr := tx.ExecContext(c, updateBalance, refundAmount, s.CustomerID)
		return execErr
	})
	if err = execUpdate(ctx); err != nil {
		return err
	}

	const insertTxn = `INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)`
	refundTxID := uuid.NewString()
	desc := fmt.Sprintf("%s :  تراکنش اصلاحی برای  ", s.TransactionID)
	execInsert := metrics.DBExecObserver("insert_refund_txn", func(c context.Context) error {
		_, execErr := tx.ExecContext(c, insertTxn, s.CustomerID, refundAmount, CorrectiveTransaction, desc, refundTxID)
		return execErr
	})
	if err = execInsert(ctx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func calculatePrice(Type model.Type, Quantity int) int64 {
	return int64(getPricePerType(Type) * Quantity)
}

// could read from DB
func getPricePerType(t model.Type) int {
	if t == model.EXPRESS {
		return 3
	}
	return 1
}

type AddBalanceRequest struct {
	CustomerID  int64
	Amount      uint64
	Description string
}

func AddBalance(ctx context.Context, req AddBalanceRequest) (err error) {
	// Retry deadlocks/lock timeouts (common during load test seeding).
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(30*(1<<attempt)) * time.Millisecond)
		}

		tx, txErr := app.DB.BeginTxx(ctx, nil)
		if txErr != nil {
			return txErr
		}

		// Single statement upsert avoids UPDATE+INSERT deadlocks.
		const upsertBalance = `
			INSERT INTO user_balances (user_id, balance)
			VALUES (?, ?)
			ON DUPLICATE KEY UPDATE
				balance = balance + VALUES(balance),
				last_updated = CURRENT_TIMESTAMP
		`
		if lastErr = metrics.DBExecObserver("upsert_balance_add", func(c context.Context) error {
			_, execErr := tx.ExecContext(c, upsertBalance, req.CustomerID, req.Amount)
			return execErr
		})(ctx); lastErr != nil {
			_ = tx.Rollback()
			if isRetryableMySQLError(lastErr) {
				continue
			}
			return lastErr
		}

		description := req.Description
		if description == "" {
			description = fmt.Sprintf("افزایش موجودی به میزان %d", req.Amount)
		}

		txID := uuid.NewString()
		const insertTransactionQuery = `INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)`
		if lastErr = metrics.DBExecObserver("insert_deposit_txn", func(c context.Context) error {
			_, execErr := tx.ExecContext(c, insertTransactionQuery, req.CustomerID, req.Amount, Deposit, description, txID)
			return execErr
		})(ctx); lastErr != nil {
			_ = tx.Rollback()
			if isRetryableMySQLError(lastErr) {
				continue
			}
			return lastErr
		}

		if lastErr = tx.Commit(); lastErr != nil {
			_ = tx.Rollback()
			if isRetryableMySQLError(lastErr) {
				continue
			}
			return lastErr
		}
		return nil
	}
	return lastErr
}

func isRetryableMySQLError(err error) bool {
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		// 1213: deadlock; 1205: lock wait timeout
		return myErr.Number == 1213 || myErr.Number == 1205
	}
	return false
}
