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
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid input"})
	}

	if len(s.Recipients) == 0 {
		app.Logger.Error("zero recipients")
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "zero recipients"})
	}

	app.Logger.Info("", "s", s)

	// check user balance
	hasBalance, err := balance.UserHasEnoughBalance(c.Request().Context(), balance.UserHasEnoughBalanceRequest{
		CustomerID: s.CustomerID,
		Quantity:   len(s.Recipients),
		Type:       s.Type,
	})
	if err != nil {
		app.Logger.Error("UserHasEnoughBalance ", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	if !hasBalance {
		app.Logger.Info("User Has Not Enough Balance ", "user id ", s.CustomerID)
		return c.JSON(http.StatusPaymentRequired, map[string]string{"error": "dont have "})
	}

	app.Logger.Info("user has Enough Balance", "s", s)

	// if not ok return err
	//else cut down the balance
	//then send it in quew

	return c.JSON(http.StatusOK, nil)
}
