package middleware

import (
	"context"
	"net/http"
	"time"

	"social-notif/internal/handler"

	"github.com/gin-gonic/gin"
)

func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			handler.RespondError(c, http.StatusGatewayTimeout, "request_timeout", "request timed out")
			c.Abort()
		}
	}
}
