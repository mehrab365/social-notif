package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"

	"social-notif/internal/handler"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const ShopifyHMACHeader = "X-Shopify-Hmac-Sha256"

func VerifyShopifyHMAC(secret string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			logger.Warn("shopify webhook secret not configured, skipping verification")
			c.Next()
			return
		}

		signature := c.GetHeader(ShopifyHMACHeader)
		if signature == "" {
			logger.Warn("shopify hmac header missing")
			handler.RespondError(c, http.StatusUnauthorized, "unauthorized", "missing hmac signature")
			c.Abort()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			logger.Warn("failed to read request body", zap.Error(err))
			handler.RespondError(c, http.StatusBadRequest, "bad_request", "failed to read body")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature), []byte(expected)) {
			logger.Warn("shopify hmac verification failed")
			handler.RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid hmac signature")
			c.Abort()
			return
		}

		logger.Debug("shopify hmac verification succeeded")
		c.Next()
	}
}
