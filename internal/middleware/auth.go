package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func APIKeyAuth(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expectedKey == "" {
			c.Next()
			return
		}

		token := c.GetHeader("X-API-Key")
		if token == "" {
			auth := c.GetHeader("Authorization")
			token = strings.TrimPrefix(auth, "Bearer ")
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}

		c.Next()
	}
}
