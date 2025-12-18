package balance

import (
	"encoding/json"
	"net/http"
	"sms-gateway/app"

	"github.com/labstack/echo/v4"
)

func GetBalanceAndHistoryHandler(c echo.Context) error {
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

func AddBalanceHandler(c echo.Context) error {
	var req struct {
		UserID      int64  `json:"user_id"`
		Balance     uint64 `json:"balance"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		app.Logger.Error("invalid input ", "err", err)
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}

	if err := AddBalance(c.Request().Context(), AddBalanceRequest{
		CustomerID:  req.UserID,
		Amount:      req.Balance,
		Description: req.Description,
	}); err != nil {
		app.Logger.Error("add balance", "user_id", req.UserID, "err", err)
		return err
	}

	return c.JSON(http.StatusOK, "done")
}
