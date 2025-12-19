package sms

import (
	"context"
	"errors"
	"sms-gateway/app"
	"sms-gateway/internal/model"
	"strings"
)

type State string

const (
	Init           State = "init"
	SendToProvider State = "send-to-provider"
	Done           State = "done"
)

func sendSmsToProvider(ctx context.Context, s model.SMS) error {
	if err := UpdateSMS(ctx, s, Init); err != nil {
		return err
	}

	return nil
}

func UpdateSMS(ctx context.Context, s model.SMS, state State) error {
	if len(s.Recipients) == 0 {
		return errors.New("no  Recipients")
	}

	valueStrings := make([]string, 0, len(s.Recipients))
	valueArgs := make([]any, 0, len(s.Recipients)*4)
	for _, recipient := range s.Recipients {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)")
		valueArgs = append(valueArgs, s.CustomerID, s.Type, state, recipient)
	}
	query := `	INSERT INTO sms_status 
				(user_id,type, status, recipient, created_at, updated_at) 
				VALUES ` + strings.Join(valueStrings, ",")
	if _, err := app.DB.ExecContext(ctx, query, valueArgs...); err != nil {
		return err
	}

	return nil
}
