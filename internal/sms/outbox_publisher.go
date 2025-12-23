package sms

import (
	"context"
	"encoding/json"
	"time"

	"sms-gateway/app"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"
	amqp "sms-gateway/pkg/queue"

	"github.com/jmoiron/sqlx"
)

type outboxRow struct {
	ID        int64           `db:"id"`
	Payload   json.RawMessage `db:"payload"`
	Attempts  int             `db:"attempts"`
	CreatedAt time.Time       `db:"created_at"`
}

type smsOutboxPayload struct {
	Exchange    string    `json:"exchange"`
	RoutingKey  string    `json:"routing_key"`
	SMS         model.SMS `json:"sms"`
	Transaction string    `json:"transaction_id"`
}

const (
	highPriorityMin = 5

	highPriorityWorkers = 4
	lowPriorityWorkers  = 2

	// Batch sizes: keep transactions reasonably small but high-throughput.
	highBatchSize = 200
	lowBatchSize  = 100

	// Idle sleep when no work was claimed.
	highIdleSleep = 80 * time.Millisecond
	lowIdleSleep  = 250 * time.Millisecond
)

// StartOutboxPublisher polls outbox_events and publishes sms.send events to RabbitMQ.
// Higher priority events are published first.
func StartOutboxPublisher(ctx context.Context) error {
	// High-priority pool (express): more workers and more aggressive polling.
	for i := 0; i < highPriorityWorkers; i++ {
		go func(workerID int) {
			workerLoop(ctx, "high", workerID, highBatchSize, highPriorityMin, nil, highIdleSleep)
		}(i)
	}

	// Low-priority pool (normal): fewer workers; avoids starving high-priority work.
	max := highPriorityMin
	for i := 0; i < lowPriorityWorkers; i++ {
		go func(workerID int) {
			workerLoop(ctx, "low", workerID, lowBatchSize, 0, &max, lowIdleSleep)
		}(i)
	}

	<-ctx.Done()
	return nil
}

func workerLoop(ctx context.Context, pool string, workerID int, batchSize int, minPriority int, maxPriority *int, idleSleep time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		rows, err := claimPending(ctx, batchSize, minPriority, maxPriority)
		if err != nil {
			app.Logger.Error("outbox claim pending", "pool", pool, "worker_id", workerID, "err", err)
			time.Sleep(idleSleep)
			continue
		}
		if len(rows) == 0 {
			time.Sleep(idleSleep)
			continue
		}

		for _, r := range rows {
			if err := publishOne(ctx, r); err != nil {
				app.Logger.Error("outbox publish one", "pool", pool, "worker_id", workerID, "id", r.ID, "err", err)
			}
		}
	}
}

func claimPending(ctx context.Context, limit int, minPriority int, maxPriority *int) ([]outboxRow, error) {
	tx, err := app.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var rows []outboxRow
	const selectQ = `
		SELECT id, payload, attempts, created_at
		FROM outbox_events
		WHERE status = 'pending'
		  AND event_type = 'sms.send'
		  AND priority >= ?
		  AND (? IS NULL OR priority < ?)
		  AND (next_run_at IS NULL OR next_run_at <= CURRENT_TIMESTAMP)
		ORDER BY priority DESC, created_at ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`
	var maxVal any = nil
	var maxVal2 any = nil
	if maxPriority != nil {
		maxVal = *maxPriority
		maxVal2 = *maxPriority
	}

	if err := tx.SelectContext(ctx, &rows, selectQ, minPriority, maxVal, maxVal2, limit); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		_ = tx.Commit()
		return nil, nil
	}

	ids := make([]any, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	// Mark claimed rows as "processing" (soft lock).
	if err := markProcessing(ctx, tx, ids); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows, nil
}

func markProcessing(ctx context.Context, tx *sqlx.Tx, ids []any) error {
	// Build IN (?, ?, ...) safely for small batches.
	in := "?"
	for i := 1; i < len(ids); i++ {
		in += ",?"
	}
	q := `UPDATE outbox_events SET status = 'processing' WHERE id IN (` + in + `)`
	_, err := tx.ExecContext(ctx, q, ids...)
	return err
}

func publishOne(ctx context.Context, r outboxRow) error {
	var p smsOutboxPayload
	if err := json.Unmarshal(r.Payload, &p); err != nil {
		return failOrRetry(ctx, r.ID, r.Attempts, err, nil)
	}

	// Publish to Rabbit.
	msg, err := json.Marshal(p.SMS)
	if err != nil {
		return failOrRetry(ctx, r.ID, r.Attempts, err, &p)
	}

	if err := app.Rabbit.PublishContext(ctx, amqp.PublishRequest{
		Exchange: p.Exchange,
		Key:      p.RoutingKey,
		Msg:      msg,
	}); err != nil {
		return failOrRetry(ctx, r.ID, r.Attempts, err, &p)
	}

	_, err = app.DB.ExecContext(ctx, `UPDATE outbox_events SET status='processed', last_error=NULL WHERE id=?`, r.ID)
	return err
}

func failOrRetry(ctx context.Context, id int64, attempts int, cause error, payload *smsOutboxPayload) error {
	nextAttempts := attempts + 1
	const maxAttempts = 10

	// Exponential backoff (cap at 60s).
	backoff := time.Second * time.Duration(1<<min(nextAttempts, 6))
	nextRun := time.Now().Add(backoff)

	if nextAttempts >= maxAttempts {
		// Permanent failure: refund (best-effort) and mark failed.
		if payload != nil && payload.SMS.TransactionID != "" && payload.SMS.CustomerID != 0 {
			_ = balance.Refund(ctx, model.SMS{CustomerID: payload.SMS.CustomerID, TransactionID: payload.SMS.TransactionID})
		}
		_, err := app.DB.ExecContext(ctx,
			`UPDATE outbox_events SET status='failed', attempts=?, last_error=? WHERE id=?`,
			nextAttempts, cause.Error(), id,
		)
		return err
	}

	_, err := app.DB.ExecContext(ctx,
		`UPDATE outbox_events SET status='pending', attempts=?, next_run_at=?, last_error=? WHERE id=?`,
		nextAttempts, nextRun, cause.Error(), id,
	)
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
