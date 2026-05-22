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
		fields := []zap.Field{
			zap.Any("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", c.FullPath()),
			zap.Int("status", c.Writer.Status()),
			zap.Int("bytes", c.Writer.Size()),
			zap.Duration("duration", time.Since(startedAt)),
			zap.String("client_ip", c.ClientIP()),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
			logger.Warn("http request completed with errors", fields...)
			return
		}

		logger.Info("http request completed", fields...)
	}
}
