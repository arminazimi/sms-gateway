package sms

import (
	"context"
	"errors"
	"fmt"
	"sms-gateway/app"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"
	"sms-gateway/internal/operator"
	"sms-gateway/pkg/metrics"
	"sms-gateway/pkg/tracing"
	"strings"

	"github.com/jmoiron/sqlx"
)

type State string

const (
	Pending State = "pending"
	Sending State = "sending"
	Done    State = "done"
	Failed  State = "failed"
)

func sendSms(ctx context.Context, s model.SMS) error {
	ctx = tracing.WithUser(ctx, fmt.Sprint(s.CustomerID))
	ctx, span := tracing.Start(ctx, "sms.send",
		tracing.Attr("customer_id", fmt.Sprint(s.CustomerID)),
		tracing.Attr("type", string(s.Type)),
	)
	defer span.End()

	if err := UpdateSMSStatus(ctx, s, Sending); err != nil {
		app.Logger.Error("err in update sms status to sending", "err", err)
		return err
	}

	provider, err := operator.Send(ctx, s)
	if err != nil {
		app.Logger.Error("err in sending msg to provider", "err", err)
		if err := UpdateSMSStatus(ctx, s, Failed); err != nil {
			app.Logger.Error("err in update sms status to failed", "err", err)
			return err
		}
		if err := balance.Refund(ctx, s); err != nil {
			app.Logger.Error("err in Refund ", "err", err)
			return err
		}
		return err
	}

	if err := UpdateSMSStatus(ctx, s, Done, provider); err != nil {
		app.Logger.Error("err in update sms status to done", "err", err)
		return err
	}

	app.Logger.Info("sms processed successfully", "user_id", s.CustomerID, "type", s.Type)

	return nil
}

// InsertPendingTx inserts PENDING rows for each recipient inside the given DB transaction.
// This should be called from the API flow when inserting the outbox event.
func InsertPendingTx(ctx context.Context, tx *sqlx.Tx, s model.SMS) error {
	if tx == nil {
		return errors.New("tx is required")
	}
	if len(s.Recipients) == 0 {
		return errors.New("no recipients")
	}
	if s.SmsIdentifier == "" {
		return errors.New("sms_identifier is required")
	}

	// Batch insert to reduce roundtrips. Idempotent via unique(sms_identifier,recipient).
	// If the row already exists, keep it unchanged.
	const prefix = `INSERT INTO sms_status (user_id,type,status,recipient,provider,sms_identifier,created_at,updated_at) VALUES `
	const suffix = ` ON DUPLICATE KEY UPDATE updated_at = updated_at`
	execFn := metrics.DBExecObserver("insert_sms_pending", func(c context.Context) error {
		valueStrings := make([]string, 0, len(s.Recipients))
		args := make([]any, 0, len(s.Recipients)*6)
		for _, recipient := range s.Recipients {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, '', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)")
			args = append(args, s.CustomerID, s.Type, Pending, recipient, s.SmsIdentifier)
		}
		q := prefix + strings.Join(valueStrings, ",") + suffix
		_, err := tx.ExecContext(c, q, args...)
		return err
	})
	return execFn(ctx)
}

// InsertPending inserts PENDING rows for each recipient using the global DB connection.
func InsertPending(ctx context.Context, s model.SMS) error {
	tx, err := app.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := InsertPendingTx(ctx, tx, s); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateSMSStatus updates existing sms_status rows for each recipient.
// This is used by the consumer/worker: PENDING -> SENDING -> DONE/FAILED.
func UpdateSMSStatus(ctx context.Context, s model.SMS, state State, provider ...string) error {
	if len(s.Recipients) == 0 {
		return errors.New("no recipients")
	}
	if s.SmsIdentifier == "" {
		return errors.New("sms_identifier is required")
	}

	providerName := ""
	if len(provider) > 0 {
		providerName = provider[0]
	}

	// Batch update recipients in one query.
	const qPrefix = `UPDATE sms_status SET status = ?, provider = ?, updated_at = CURRENT_TIMESTAMP WHERE sms_identifier = ? AND recipient IN (`
	execFn := metrics.DBExecObserver("update_sms_status", func(c context.Context) error {
		placeholders := make([]string, 0, len(s.Recipients))
		args := make([]any, 0, 3+len(s.Recipients))
		args = append(args, state, providerName, s.SmsIdentifier)
		for _, recipient := range s.Recipients {
			placeholders = append(placeholders, "?")
			args = append(args, recipient)
		}
		q := qPrefix + strings.Join(placeholders, ",") + `)`
		_, err := app.DB.ExecContext(c, q, args...)
		return err
	})
	return execFn(ctx)
}

type UserHistory struct {
	UserID        int64      `db:"user_id" json:"user_id"`
	Type          model.Type `db:"type" json:"type"`
	Status        State      `db:"status" json:"status"`
	Recipient     string     `db:"recipient" json:"recipient"`
	Provider      string     `db:"provider" json:"provider"`
	SmsIdentifier string     `db:"sms_identifier" json:"sms_identifier"`
	CreatedAt     string     `db:"created_at" json:"created_at"`
	UpdatedAt     string     `db:"updated_at" json:"updated_at"`
}

func GetUserHistory(ctx context.Context, userID string, status string, smsIdentifier string) ([]UserHistory, error) {
	query := `SELECT user_id, type, status, recipient, provider, sms_identifier, created_at, updated_at FROM sms_status WHERE user_id = ?`
	args := []any{userID}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	if smsIdentifier != "" {
		query += ` AND sms_identifier = ?`
		args = append(args, smsIdentifier)
	}

	query += ` ORDER BY created_at DESC`

	var history []UserHistory
	queryFn := metrics.DBExecObserver("select_sms_history", func(c context.Context) error {
		return app.DB.SelectContext(c, &history, query, args...)
	})
	if err := queryFn(ctx); err != nil {
		return nil, err
	}

	return history, nil
}
