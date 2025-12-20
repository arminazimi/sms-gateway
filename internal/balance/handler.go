package balance

import (
	"encoding/json"
	"net/http"
	"sms-gateway/app"

	"github.com/labstack/echo/v4"
)

// AddBalancePayload represents the request body for adding balance.
type AddBalancePayload struct {
	UserID      int64  `json:"user_id"`
	Balance     uint64 `json:"balance"`
	Description string `json:"description"`
}

// GetBalanceAndHistoryHandler godoc
// @Summary      Get user balance and transactions
// @Description  Returns current balance and transaction history
// @Tags         balance
// @Produce      json
// @Param        user_id query string true "User ID"
// @Success      200 {object} map[string]any
// @Failure      400 {string} string "user_id is required"
// @Failure      500 {string} string "internal error"
// @Router       /balance [get]
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

// AddBalanceHandler godoc
// @Summary      Add balance for user
// @Description  Increases user balance and records transaction
// @Tags         balance
// @Accept       json
// @Produce      json
// @Param        request body AddBalancePayload true "Add balance request"
// @Success      200 {string} string "done"
// @Failure      400 {string} string "invalid input"
// @Failure      500 {string} string "internal error"
// @Router       /balance/add [post]
func AddBalanceHandler(c echo.Context) error {
	var req AddBalancePayload
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
