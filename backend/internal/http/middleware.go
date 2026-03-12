package http

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
)

const requestIDHeader = "X-Request-Id"

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = ulid.Make().String()
		}

		c.Header(requestIDHeader, requestID)
		c.Set(requestIDHeader, requestID)

		start := time.Now()
		c.Next()

		logger.Info(
			"http request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
