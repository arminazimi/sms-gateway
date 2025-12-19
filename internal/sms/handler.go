package sms

import (
	"encoding/json"
	"net/http"
	"sms-gateway/app"
	"sms-gateway/config"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"
	amqp "sms-gateway/pkg/queue"

	"github.com/labstack/echo/v4"
)

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

	if err := balance.DeductBalance(c.Request().Context(), balance.DeductBalanceRequest{
		CustomerID: s.CustomerID,
		Quantity:   len(s.Recipients),
		Type:       s.Type,
	}); err != nil {
		app.Logger.Error("DeductBalance ", "err", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

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

	return c.JSON(http.StatusOK, "your msg is processing")
}

func HistoryHandler(c echo.Context) error {
	userID := c.QueryParam("user_id")
	if userID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}

	history, err := GetUserHistory(c.Request().Context(), userID)
	if err != nil {
		app.Logger.Error("get sms history", "user_id", userID, "err", err)
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
