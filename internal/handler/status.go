package handler

import (
	"net/http"

	"social-notif/internal/config"

	"github.com/gin-gonic/gin"
)

func GetStatus(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(statusPageHTML(cfg)))
	}
}

func statusPageHTML(cfg *config.Config) string {
	appURL := cfg.Shopify.AppURL
	apiKey := cfg.Shopify.APIKey

	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1.0">
  <title>Social Notif</title>
  <style>
    *{box-sizing:border-box;margin:0;padding:0}
    body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f5f5f5;display:flex;justify-content:center;align-items:center;min-height:100vh}
    .card{background:white;border-radius:12px;padding:48px;width:100%;max-width:540px;box-shadow:0 2px 12px rgba(0,0,0,0.08);text-align:center}
    .badge{display:inline-block;padding:4px 12px;border-radius:20px;font-size:12px;font-weight:600;margin-bottom:16px}
    .badge.on{background:#dcfce7;color:#166534}
    h1{font-size:28px;margin-bottom:8px;color:#1a1a1a}
    p{color:#666;margin-bottom:6px;font-size:14px;line-height:1.5}
    .detail{margin-top:20px;padding:16px;background:#fafafa;border-radius:8px;text-align:left;font-size:13px}
    .detail dt{font-weight:600;color:#333;margin-top:8px}
    .detail dt:first-child{margin-top:0}
    .detail dd{color:#666;margin-left:0;word-break:break-all}
  </style>
</head>
<body>
  <div class="card">
    <div class="badge on">&#10003; Running</div>
    <h1>Social Notif</h1>
    <p>WhatsApp order notification service for Shopify.</p>
    <dl class="detail">
      <dt>App URL</dt>
      <dd>` + appURL + `</dd>
      <dt>API Key</dt>
      <dd>` + apiKey + `</dd>
      <dt>Status</dt>
      <dd>Ready to install</dd>
    </dl>
  </div>
</body>
</html>`
}
