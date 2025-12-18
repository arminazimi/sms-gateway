package balance

import (
	"net/http"
	"sms-gateway/app"

	"github.com/labstack/echo/v4"
)

func GetBalanceAndHistory(c echo.Context) error {
	userID := c.QueryParam("user_id")
	if userID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}

	balance, err := GetUserBalance(c.Request().Context(), userID)
	if err != nil {
		app.Logger.Error("get balance and history", "user_id", userID, "err", err)
		return err
	}

	Transactions, err := GetUserTransactions(c.Request().Context(), userID)
	if err != nil {
		app.Logger.Error("get balance and history", "user_id", userID, "err", err)
		return err
	}

	out := map[string]any{}
	out["balance"] = balance
	out["transactions"] = Transactions

	return c.JSON(http.StatusOK, out)
}
