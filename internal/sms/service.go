package sms

import (
	"context"
	"errors"
	"sms-gateway/app"
	"sms-gateway/internal/model"
	"sms-gateway/internal/operator"
	"strings"
)

type State string

const (
	Init State = "init"
	Done State = "done"
)

func sendSms(ctx context.Context, s model.SMS) error {
	if err := UpdateSMS(ctx, s, Init); err != nil {
		return err
	}

	provider, err := operator.Send(ctx, s)
	if err != nil {
		return err
	}

	if err := UpdateSMS(ctx, s, Done, provider); err != nil {
		return err
	}

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
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)")
		valueArgs = append(valueArgs, s.CustomerID, s.Type, state, recipient, providerName)
	}
	query := ` INSERT INTO sms_status 
				(user_id,type, status, recipient, provider, created_at, updated_at) 
				VALUES ` + strings.Join(valueStrings, ",")
	if _, err := app.DB.ExecContext(ctx, query, valueArgs...); err != nil {
		return err
	}

	return nil
}

type UserHistory struct {
	UserID    int64      `db:"user_id" json:"user_id"`
	Type      model.Type `db:"type" json:"type"`
	Status    State      `db:"status" json:"status"`
	Recipient string     `db:"recipient" json:"recipient"`
	Provider  string     `db:"provider" json:"provider"`
	CreatedAt string     `db:"created_at" json:"created_at"`
	UpdatedAt string     `db:"updated_at" json:"updated_at"`
}

func GetUserHistory(ctx context.Context, userID string) ([]UserHistory, error) {
	const query = `SELECT user_id, type, status, recipient, provider, created_at, updated_at FROM sms_status WHERE user_id = ? ORDER BY created_at DESC`

	var history []UserHistory
	if err := app.DB.SelectContext(ctx, &history, query, userID); err != nil {
		return nil, err
	}

	return history, nil
}
