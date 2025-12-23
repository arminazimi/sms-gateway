package sms

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"
	"sms-gateway/internal/outbox"
	"sms-gateway/pkg/tracing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// SendHandler godoc
// @Summary      Send SMS request
// @Description  Deducts balance, enqueues SMS for processing, returns processing ack
// @Tags         sms
// @Accept       json
// @Produce      json
// @Param        request body model.SMS true "SMS request"
// @Success      200 {object} map[string]any "ack with sms_identifier"
// @Failure      400 {string} string "invalid input"
// @Failure      402 {string} string "dont have Not Enough Balance"
// @Failure      500 {string} string "internal error"
// @Router       /sms/send [post]
func SendHandler(c echo.Context) error {
	var s model.SMS
	if err := json.NewDecoder(c.Request().Body).Decode(&s); err != nil {
		app.Logger.Error("invalid input ", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}

	ctxWithUser := tracing.WithUser(c.Request().Context(), fmt.Sprint(s.CustomerID))
	c.SetRequest(c.Request().WithContext(ctxWithUser))

	if len(s.Recipients) == 0 {
		app.Logger.Error("zero recipients")
		return echo.NewHTTPError(http.StatusBadRequest, "zero recipients")
	}

	s.SmsIdentifier = uuid.NewString()
	// Atomic: deduct balance (user_transactions) + insert outbox (pending) in ONE DB transaction.
	tx, err := app.DB.BeginTxx(c.Request().Context(), nil)
	if err != nil {
		app.Logger.Error("begin tx", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	defer func() {
		_ = tx.Rollback()
	}()

	transactionID, err := balance.ChargeTx(c.Request().Context(), tx, balance.ChargeRequest{
		CustomerID: s.CustomerID,
		Quantity:   len(s.Recipients),
		Type:       s.Type,
	})
	if err != nil {
		if errors.Is(err, balance.ErrInsufficientBalance) {
			app.Logger.Error("User Has Not Enough Balance ", "user id ", s.CustomerID)
			return echo.NewHTTPError(http.StatusPaymentRequired, "dont have Not Enough Balance ")
		}
		app.Logger.Error("ChargeTx ", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	s.TransactionID = transactionID

	priority := 0
	if s.Type == model.EXPRESS {
		priority = 10
	}

	// Initial state: PENDING (inserted with the outbox record)
	if err := InsertPendingTx(c.Request().Context(), tx, s); err != nil {
		app.Logger.Error("insert sms pending", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	// Store SMS message in outbox for the job to publish to Rabbit.
	if err := outbox.InsertTx(c.Request().Context(), tx, outbox.Event{
		AggregateType: "sms",
		AggregateID:   s.SmsIdentifier,
		EventType:     "sms.send",
		Priority:      priority,
		Status:        outbox.StatusPending,
		Payload: map[string]any{
			"exchange":       config.SmsExchange,
			"routing_key":    getQueue(s.Type),
			"sms":            s,
			"transaction_id": s.TransactionID,
		},
	}); err != nil {
		app.Logger.Error("insert outbox", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	if err := tx.Commit(); err != nil {
		app.Logger.Error("commit tx", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status":         "processing",
		"sms_identifier": s.SmsIdentifier,
	})
}

// HistoryHandler godoc
// @Summary      Get SMS history for user
// @Description  Returns sent SMS history for a user
// @Tags         sms
// @Accept       json
// @Produce      json
// @Param        user_id query string true "User ID"
// @Param        status query string false "Filter by status (pending|sending|done|failed)"
// @Param        sms_identifier query string false "Filter by sms_identifier"
// @Success      200 {object} map[string]any
// @Failure      400 {string} string "user_id is required"
// @Failure      500 {string} string "internal error"
// @Router       /sms/history [get]
func HistoryHandler(c echo.Context) error {
	userID := c.QueryParam("user_id")
	if userID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}

	status := c.QueryParam("status")
	smsIdentifier := c.QueryParam("sms_identifier")

	history, err := GetUserHistory(c.Request().Context(), userID, status, smsIdentifier)
	if err != nil {
		app.Logger.Error("get sms history", "user_id", userID, "status", status, "sms_identifier", smsIdentifier, "err", err)
		return err
	}

	out := map[string]any{}
	out["history"] = history

	return c.JSON(http.StatusOK, out)
}

func getQueue(s model.Type) string {
	switch s {
	case model.NORMAL:
		return config.NormalQueue
	case model.EXPRESS:
		return config.ExpressQueue
	default:
		return config.NormalQueue
	}
}
