package app

import (
	"github.com/labstack/echo/v4"
)

var (
	Echo *echo.Echo
)

func Init() {
	InitEcho()
}

func InitEcho() {
	Echo = echo.New()
}
