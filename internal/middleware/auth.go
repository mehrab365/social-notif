package middleware

import (
	"crypto/subtle"
	"net/http"

	"social-notif/internal/handler"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const APIKeyHeader = "X-API-KEY"

func APIKeyAuth(expectedKey string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedKey == "" {
			logger.Warn("api key authentication disabled", requestLogFields(c)...)
			c.Next()
			return
		}

		token := c.GetHeader(APIKeyHeader)

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedKey)) != 1 {
			logger.Warn("api key authentication failed", requestLogFields(c)...)
			handler.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid api key")
			c.Abort()
			return
		}

		logger.Debug("api key authentication succeeded", requestLogFields(c)...)
		c.Next()
	}
}

func requestLogFields(c *gin.Context) []zap.Field {
	requestID, _ := c.Get(RequestIDKey)
	return []zap.Field{
		zap.Any("request_id", requestID),
		zap.String("method", c.Request.Method),
		zap.String("path", c.FullPath()),
		zap.String("client_ip", c.ClientIP()),
	}
}
