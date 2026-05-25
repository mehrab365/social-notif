package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		requestID, _ := c.Get(RequestIDKey)
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		statusCode := c.Writer.Status()
		latency := time.Since(startedAt)
		fields := []zap.Field{
			zap.Any("request_id", requestID),
			zap.Duration("latency", latency),
			zap.Int("status_code", statusCode),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("response_bytes", c.Writer.Size()),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
			logger.Error("http request completed with errors", fields...)
			return
		}

		if statusCode >= 500 {
			logger.Error("http request completed with server error", fields...)
			return
		}

		logger.Info("http request completed", fields...)
	}
}
