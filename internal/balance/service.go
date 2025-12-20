package balance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sms-gateway/app"
	"sms-gateway/internal/model"

	"github.com/google/uuid"
)

type transactionType string

const (
	Withdrawal            transactionType = "withdrawal"
	Deposit               transactionType = "deposit"
	CorrectiveTransaction transactionType = "Corrective"
)

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

type DeductBalanceRequest struct {
	CustomerID int64
	Quantity   int
	Type       model.Type
}

func DeductBalance(ctx context.Context, req DeductBalanceRequest) (string, error) {
	price := calculatePrice(req.Type, req.Quantity)

	tx, err := app.DB.BeginTxx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

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
		return "", errors.New("insufficient balance")
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

	if err = tx.Commit(); err != nil {
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
	if err := app.DB.SelectContext(ctx, &transactions, query, userID); err != nil {
		return nil, err
	}

	return transactions, nil
}

func GetUserBalance(ctx context.Context, userID string) (int64, error) {

	const query = `SELECT balance FROM user_balances WHERE user_id = ?`
	var balance int64
	if err := app.DB.QueryRowxContext(ctx, query, userID).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return balance, nil
}

func Refund(ctx context.Context, s model.SMS) error {
	if s.TransactionID == "" {
		return errors.New("transaction_id is required for refund")
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
	if err = tx.QueryRowxContext(ctx, selectTxn, s.TransactionID, s.CustomerID).Scan(&amount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("transaction not found")
		}
		return err
	}

	refundAmount := -1 * amount
	const updateBalance = `UPDATE user_balances SET balance = balance + ? WHERE user_id = ?`
	res, execErr := tx.ExecContext(ctx, updateBalance, refundAmount, s.CustomerID)
	if execErr != nil {
		err = execErr
		return err
	}

	rows, rowsErr := res.RowsAffected()
	if rowsErr != nil {
		err = rowsErr
		return err
	}
	if rows == 0 {
		const insertBalance = `INSERT INTO user_balances (user_id, balance) VALUES (?, ?)`
		if _, err = tx.ExecContext(ctx, insertBalance, s.CustomerID, refundAmount); err != nil {
			return err
		}
	}

	refundTxID := uuid.NewString()
	const insertTxn = `INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)`
	desc := fmt.Sprintf("%s :  تراکنش اصلاحی برای  ", s.TransactionID)
	if _, err = tx.ExecContext(ctx, insertTxn, s.CustomerID, refundAmount, CorrectiveTransaction, desc, refundTxID); err != nil {
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
	tx, err := app.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const increaseBalanceQuery = `UPDATE user_balances SET balance = balance + ? WHERE user_id = ?`
	res, err := tx.ExecContext(ctx, increaseBalanceQuery, req.Amount, req.CustomerID)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		const insertBalanceQuery = `INSERT INTO user_balances (user_id, balance) VALUES (?, ?)`
		if _, err = tx.ExecContext(ctx, insertBalanceQuery, req.CustomerID, req.Amount); err != nil {
			return err
		}
	}

	description := req.Description
	if description == "" {
		description = fmt.Sprintf("افزایش موجودی به میزان %d", req.Amount)
	}

	txID := uuid.NewString()
	const insertTransactionQuery = `INSERT INTO user_transactions (user_id, amount, transaction_type, description, transaction_id) VALUES (?, ?, ?, ?, ?)`
	if _, err = tx.ExecContext(ctx, insertTransactionQuery, req.CustomerID, req.Amount, Deposit, description, txID); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}
