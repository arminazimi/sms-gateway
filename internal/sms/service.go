package sms

import (
	"context"
	"errors"
	"sms-gateway/app"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"
	"sms-gateway/internal/operator"
	"sms-gateway/pkg/metrics"
	"strings"
)

type State string

const (
	Init   State = "init"
	Done   State = "done"
	Failed State = "failed"
)

func sendSms(ctx context.Context, s model.SMS) error {
	if err := UpdateSMS(ctx, s, Init); err != nil {
		app.Logger.Error("err in update sms ", "err", err)
		return err
	}

	provider, err := operator.Send(ctx, s)
	if err != nil {
		app.Logger.Error("err in sending msg to provider", "err", err)
		if err := UpdateSMS(ctx, s, Failed); err != nil {
			app.Logger.Error("err in update sms ", "err", err)
			return err
		}
		if err := balance.Refund(ctx, s); err != nil {
			app.Logger.Error("err in Refund ", "err", err)
			return err
		}
		return err
	}

	if err := UpdateSMS(ctx, s, Done, provider); err != nil {
		app.Logger.Error("err in update sms ", "err", err)
		return err
	}

	app.Logger.Info("sms processed successfully", "user_id", s.CustomerID, "type", s.Type)

	return nil
}

func UpdateSMS(ctx context.Context, s model.SMS, state State, provider ...string) error {
	if len(s.Recipients) == 0 {
		return errors.New("no  Recipients")
	}

	var providerName string
	if len(provider) > 0 {
		providerName = provider[0]
	}

	valueStrings := make([]string, 0, len(s.Recipients))
	valueArgs := make([]any, 0, len(s.Recipients)*5)
	for _, recipient := range s.Recipients {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)")
		valueArgs = append(valueArgs, s.CustomerID, s.Type, state, recipient, providerName, s.SmsIdentifier)
	}
	query := ` INSERT INTO sms_status 
				(user_id,type, status, recipient, provider, sms_identifier, created_at, updated_at) 
				VALUES ` + strings.Join(valueStrings, ",")

	execFn := metrics.DBExecObserver("insert_sms_status", func(c context.Context) error {
		_, err := app.DB.ExecContext(c, query, valueArgs...)
		return err
	})
	if err := execFn(ctx); err != nil {
		return err
	}

	return nil
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
