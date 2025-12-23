package outbox

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusProcessed Status = "processed"
	StatusFailed    Status = "failed"
)

type Event struct {
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       any
	Priority      int
	Status        Status
}

func InsertTx(ctx context.Context, tx *sqlx.Tx, evt Event) error {
	if tx == nil {
		return errors.New("tx is required")
	}
	if evt.AggregateType == "" || evt.AggregateID == "" || evt.EventType == "" {
		return errors.New("aggregate_type, aggregate_id, and event_type are required")
	}
	if evt.Status == "" {
		evt.Status = StatusPending
	}

	b, err := json.Marshal(evt.Payload)
	if err != nil {
		return err
	}

	const q = `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload, priority, status)
		VALUES (?, ?, ?, CAST(? AS JSON), ?, ?)
	`
	_, err = tx.ExecContext(ctx, q, evt.AggregateType, evt.AggregateID, evt.EventType, string(b), evt.Priority, evt.Status)
	return err
}


