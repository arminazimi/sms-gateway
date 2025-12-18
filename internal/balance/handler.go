package balance

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func GetBalanceAndHistory(c echo.Context) error {
	return c.JSON(http.StatusOK, nil)
}
