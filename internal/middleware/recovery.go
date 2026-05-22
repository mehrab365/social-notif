package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		requestID, _ := c.Get(RequestIDKey)
		logger.Error("panic recovered",
			zap.Any("request_id", requestID),
			zap.Any("panic", recovered),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "internal_server_error",
		})
	})
}
