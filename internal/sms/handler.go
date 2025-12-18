package sms

import (
	"encoding/json"
	"net/http"
	"sms-gateway/app"
	"sms-gateway/internal/balance"
	"sms-gateway/internal/model"

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

	app.Logger.Info("", "s", s)

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
		app.Logger.Info("User Has Not Enough Balance ", "user id ", s.CustomerID)
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

	//then send it in quew

	return c.JSON(http.StatusOK, nil)
}
