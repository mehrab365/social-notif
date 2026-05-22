package middleware

import (
	"net/http"

	"social-notif/internal/handler"

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
		handler.RespondError(c, http.StatusInternalServerError, "internal_server_error", "internal server error")
		c.Abort()
	})
}
