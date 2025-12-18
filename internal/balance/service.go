package balance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sms-gateway/app"
	"sms-gateway/internal/model"
)

type transactionType string

const (
	Withdrawal transactionType = "withdrawal"
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

func DeductBalance(ctx context.Context, req DeductBalanceRequest) (err error) {
	price := calculatePrice(req.Type, req.Quantity)

	tx, err := app.DB.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const updateBalanceQuery = `UPDATE user_balances SET balance = balance - ? WHERE user_id = ? AND balance >= ?`
	res, err := tx.ExecContext(ctx, updateBalanceQuery, price, req.CustomerID, price)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("insufficient balance")
	}

	const insertTransactionQuery = `INSERT INTO user_transactions (user_id, amount, transaction_type, description) VALUES (?, ?, ?, ?)`
	if _, err = tx.ExecContext(ctx,
		insertTransactionQuery,
		req.CustomerID,
		-price,
		Withdrawal,
		descriptionGenerator(req.Type, req.Quantity)); err != nil {
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

func descriptionGenerator(t model.Type, q int) string {
	return fmt.Sprintf("بابت خرید %d پیامک تایپ %s", q, t)
}
