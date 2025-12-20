package sms

import (
	"encoding/json"
	"net/http"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"
	amqp "sms-gateway/pkg/queue"

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

	if len(s.Recipients) == 0 {
		app.Logger.Error("zero recipients")
		return echo.NewHTTPError(http.StatusBadRequest, "zero recipients")
	}

	// check user balance
	hasBalance, err := balance.UserHasBalance(c.Request().Context(), balance.UserHasEnoughBalanceRequest{
		CustomerID: s.CustomerID,
		Quantity:   len(s.Recipients),
		Type:       s.Type,
	})
	if err != nil {
		app.Logger.Error("UserHasBalance ", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}
	if !hasBalance {
		app.Logger.Error("User Has Not Enough Balance ", "user id ", s.CustomerID)
		return echo.NewHTTPError(http.StatusPaymentRequired, "dont have Not Enough Balance ")
	}

	transactionID, err := balance.DeductBalance(c.Request().Context(), balance.DeductBalanceRequest{
		CustomerID: s.CustomerID,
		Quantity:   len(s.Recipients),
		Type:       s.Type,
	})
	if err != nil {
		app.Logger.Error("DeductBalance ", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	s.TransactionID = transactionID
	s.SmsIdentifier = uuid.NewString()

	b, err := json.Marshal(s)
	if err != nil {
		app.Logger.Error(" err in marshal", "user id ", s.CustomerID)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	if err = app.Rabbit.PublishContext(c.Request().Context(), amqp.PublishRequest{
		Exchange: config.SmsExchange,
		Key:      getQueue(s.Type),
		Msg:      b,
	}); err != nil {
		app.Logger.Error(" err in publish", "user id ", s.CustomerID)
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
// @Param        status query string false "Filter by status (init|done|failed)"
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
