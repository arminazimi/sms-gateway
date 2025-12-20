package metrics

import (
	"net/http"

	"github.com/labstack/echo/v4"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = prom.NewCounterVec(
		prom.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed",
		},
		[]string{"path", "method", "status"},
	)
	httpDuration = prom.NewHistogramVec(
		prom.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prom.DefBuckets,
		},
		[]string{"path", "method"},
	)
)

func init() {
	prom.MustRegister(httpRequests, httpDuration)
}

func EchoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Path()
			method := c.Request().Method
			timer := prom.NewTimer(httpDuration.WithLabelValues(path, method))
			err := next(c)
			timer.ObserveDuration()
			status := c.Response().Status
			httpRequests.WithLabelValues(path, method, http.StatusText(status)).Inc()
			return err
		}
	}
}

func Handler() echo.HandlerFunc {
	h := promhttp.Handler()
	return func(c echo.Context) error {
		h.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
