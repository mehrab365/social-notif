package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"social-notif/internal/config"
	"social-notif/internal/domain"
	"social-notif/internal/repository"
	"social-notif/internal/service"
	"social-notif/internal/shopify"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ShopifyAuthHandler struct {
	cfg      *config.Config
	logger   *zap.Logger
	shopRepo repository.ShopRepository
	msgSvc   service.MessageService
}

func NewShopifyAuthHandler(cfg *config.Config, logger *zap.Logger, shopRepo repository.ShopRepository, msgSvc service.MessageService) *ShopifyAuthHandler {
	return &ShopifyAuthHandler{
		cfg:      cfg,
		logger:   logger,
		shopRepo: shopRepo,
		msgSvc:   msgSvc,
	}
}

// GET /api/v1/shopify/auth?shop={shop}
func (h *ShopifyAuthHandler) Authorize(c *gin.Context) {
	shop := c.Query("shop")
	if shop == "" {
		RespondError(c, http.StatusBadRequest, "validation_error", "shop parameter is required")
		return
	}

	scopes := "write_webhooks,read_webhooks"
	redirectURI := h.cfg.Shopify.AppURL + "/api/v1/shopify/callback"
	authURL := fmt.Sprintf("https://%s/admin/oauth/authorize?client_id=%s&scope=%s&redirect_uri=%s",
		shop, h.cfg.Shopify.APIKey, url.QueryEscape(scopes), url.QueryEscape(redirectURI))

	c.Redirect(http.StatusFound, authURL)
}

// GET /api/v1/shopify/callback?code={code}&shop={shop}&hmac={hmac}&timestamp={ts}
func (h *ShopifyAuthHandler) Callback(c *gin.Context) {
	code := c.Query("code")
	shop := c.Query("shop")
	hmacHeader := c.Query("hmac")

	if !h.verifyCallbackHMAC(c.Request.URL.RawQuery, hmacHeader) {
		RespondError(c, http.StatusUnauthorized, "unauthorized", "invalid hmac")
		return
	}

	accessToken, err := h.exchangeCode(shop, code)
	if err != nil {
		h.logger.Error("failed to exchange code for token", zap.Error(err))
		RespondError(c, http.StatusInternalServerError, "internal_error", "failed to complete installation")
		return
	}

	existingShop, lookupErr := h.shopRepo.GetByDomain(c.Request.Context(), shop)
	if lookupErr == nil && existingShop != nil {
		_ = h.shopRepo.SetSetupToken(c.Request.Context(), existingShop.ID, "")
		html := h.configFormHTML(existingShop.ID, shop, existingShop.WebhookID > 0)
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
		return
	}

	newShop := &domain.Shop{
		ShopDomain:         shop,
		ShopifyAccessToken: accessToken,
	}
	if err := h.shopRepo.Create(c.Request.Context(), newShop); err != nil {
		h.logger.Error("failed to save shop", zap.Error(err))
		RespondError(c, http.StatusInternalServerError, "internal_error", "failed to save shop")
		return
	}

	setupToken, _ := generateToken()
	if err := h.shopRepo.SetSetupToken(c.Request.Context(), newShop.ID, setupToken); err != nil {
		h.logger.Error("failed to set setup token", zap.Error(err))
	}

	html := h.configFormHTML(newShop.ID, shop, false)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// GET /api/v1/shopify/setup-token?shop_id={id}
func (h *ShopifyAuthHandler) GetSetupToken(c *gin.Context) {
	shopID := c.Query("shop_id")
	if shopID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "shop_id required"})
		return
	}

	shop, err := h.shopRepo.GetByID(c.Request.Context(), shopID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		return
	}

	token := shop.SetupToken
	if token == "" {
		newToken, _ := generateToken()
		if err := h.shopRepo.SetSetupToken(c.Request.Context(), shopID, newToken); err == nil {
			token = newToken
		}
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// GET /api/v1/shopify/setup-complete?shop={shop}
func (h *ShopifyAuthHandler) SetupComplete(c *gin.Context) {
	shopDomain := c.Query("shop")
	html := h.successHTML(shopDomain)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// POST /api/v1/shopify/configure
func (h *ShopifyAuthHandler) Configure(c *gin.Context) {
	shopID := c.PostForm("shop_id")
	setupToken := c.PostForm("setup_token")
	phoneNumberID := c.PostForm("phone_number_id")
	accessToken := c.PostForm("access_token")
	templateName := c.PostForm("template_name")
	templateLanguage := c.PostForm("template_language")

	if shopID == "" || setupToken == "" || phoneNumberID == "" || accessToken == "" || templateName == "" {
		RespondError(c, http.StatusBadRequest, "validation_error", "all fields are required")
		return
	}
	if templateLanguage == "" {
		templateLanguage = "en_US"
	}

	shop, err := h.shopRepo.GetByID(c.Request.Context(), shopID)
	if err != nil {
		RespondError(c, http.StatusNotFound, "not_found", "shop not found")
		return
	}
	if shop.SetupToken == "" || shop.SetupToken != setupToken {
		RespondError(c, http.StatusForbidden, "forbidden", "invalid setup token")
		return
	}

	if err := h.shopRepo.UpdateWhatsAppConfig(c.Request.Context(), shopID, domain.ShopWhatsAppConfig{
		AccessToken:      accessToken,
		PhoneNumberID:    phoneNumberID,
		TemplateName:     templateName,
		TemplateLanguage: templateLanguage,
	}); err != nil {
		h.logger.Error("failed to save whatsapp config", zap.Error(err))
		RespondError(c, http.StatusInternalServerError, "internal_error", "failed to save configuration")
		return
	}

	_ = h.shopRepo.ClearSetupToken(c.Request.Context(), shopID)

	webhookID, err := h.registerWebhook(shop.ShopDomain, shop.ShopifyAccessToken)
	if err != nil {
		h.logger.Error("failed to register webhook", zap.Error(err))
	} else if webhookID > 0 {
		_ = h.shopRepo.UpdateWebhookID(c.Request.Context(), shopID, webhookID)
	}

	shopDomain := shop.ShopDomain
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(h.successHTML(shopDomain)))
}

// POST /api/v1/shopify/reconfigure (requires X-API-KEY)
func (h *ShopifyAuthHandler) Reconfigure(c *gin.Context) {
	var req struct {
		ShopDomain       string `json:"shop_domain" binding:"required"`
		PhoneNumberID    string `json:"phone_number_id" binding:"required"`
		AccessToken      string `json:"access_token" binding:"required"`
		TemplateName     string `json:"template_name" binding:"required"`
		TemplateLanguage string `json:"template_language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	shop, err := h.shopRepo.GetByDomain(c.Request.Context(), req.ShopDomain)
	if err != nil {
		RespondError(c, http.StatusNotFound, "not_found", "shop not found")
		return
	}

	lang := req.TemplateLanguage
	if lang == "" {
		lang = "en_US"
	}

	if err := h.shopRepo.UpdateWhatsAppConfig(c.Request.Context(), shop.ID, domain.ShopWhatsAppConfig{
		AccessToken:      req.AccessToken,
		PhoneNumberID:    req.PhoneNumberID,
		TemplateName:     req.TemplateName,
		TemplateLanguage: lang,
	}); err != nil {
		h.logger.Error("failed to update whatsapp config", zap.Error(err))
		RespondError(c, http.StatusInternalServerError, "internal_error", "failed to update config")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "whatsapp config updated"})
}

// POST /api/v1/webhooks/shopify/order-create
func (h *ShopifyAuthHandler) HandleOrderCreate(c *gin.Context) {
	shopDomain := c.GetHeader("X-Shopify-Shop-Domain")
	if shopDomain == "" {
		RespondError(c, http.StatusBadRequest, "validation_error", "missing shop domain")
		return
	}

	shop, err := h.shopRepo.GetByDomain(c.Request.Context(), shopDomain)
	if err != nil {
		h.logger.Warn("unknown shop", zap.String("shop_domain", shopDomain))
		RespondError(c, http.StatusNotFound, "not_found", "shop not found")
		return
	}

	if shop.WhatsAppAccessToken == "" || shop.WhatsAppPhoneNumberID == "" {
		h.logger.Warn("shop has no whatsapp config", zap.String("shop_domain", shopDomain))
		RespondError(c, http.StatusPreconditionFailed, "not_configured", "whatsapp not configured")
		return
	}

	var order struct {
		ID          int64  `json:"id"`
		OrderNumber int64  `json:"order_number"`
		Name        string `json:"name"`
		CreatedAt   string `json:"created_at"`
		Customer    *struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Phone     string `json:"phone"`
		} `json:"customer"`
		BillingAddress *struct {
			Phone string `json:"phone"`
		} `json:"billing_address"`
		ShippingAddress *struct {
			Phone string `json:"phone"`
		} `json:"shipping_address"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&order); err != nil {
		RespondError(c, http.StatusBadRequest, "validation_error", "invalid webhook payload")
		return
	}

	phone := ""
	if order.Customer != nil && order.Customer.Phone != "" {
		phone = order.Customer.Phone
	} else if order.BillingAddress != nil && order.BillingAddress.Phone != "" {
		phone = order.BillingAddress.Phone
	} else if order.ShippingAddress != nil && order.ShippingAddress.Phone != "" {
		phone = order.ShippingAddress.Phone
	} else {
		phone = order.Phone
	}
	phone = normalizePhone(phone)
	if phone == "" {
		h.logger.Warn("order has no customer phone", zap.Int64("order_id", order.ID))
		RespondError(c, http.StatusBadRequest, "validation_error", "customer phone is required")
		return
	}

	customerName := ""
	if order.Customer != nil {
		name := strings.TrimSpace(order.Customer.FirstName + " " + order.Customer.LastName)
		if name != "" {
			customerName = name
		}
	}

	orderID := order.Name
	if orderID == "" {
		orderID = fmt.Sprintf("#%d", order.OrderNumber)
	}

	templateParams := []string{customerName, orderID, extractDate(order.CreatedAt)}

	if _, err := h.msgSvc.EnqueueWhatsAppMessageForShop(c.Request.Context(),
		shop, phone, "Your order "+orderID+" has been confirmed!", templateParams); err != nil {
		h.logger.Error("failed to enqueue message", zap.Error(err))
		RespondError(c, http.StatusInternalServerError, "internal_error", "failed to enqueue message")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "ok"})
}

// --- helpers ---

func (h *ShopifyAuthHandler) verifyCallbackHMAC(rawQuery, hmacHeader string) bool {
	values, _ := url.ParseQuery(rawQuery)
	delete(values, "hmac")
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		for _, v := range values[k] {
			parts = append(parts, k+"="+v)
		}
	}
	message := strings.Join(parts, "&")

	mac := hmac.New(sha256.New, []byte(h.cfg.Shopify.APISecret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(hmacHeader), []byte(expected))
}

func (h *ShopifyAuthHandler) exchangeCode(shop, code string) (string, error) {
	endpoint := fmt.Sprintf("https://%s/admin/oauth/access_token", shop)
	body := map[string]string{
		"client_id":     h.cfg.Shopify.APIKey,
		"client_secret": h.cfg.Shopify.APISecret,
		"code":          code,
	}
	payload, _ := json.Marshal(body)

	resp, err := http.Post(endpoint, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed (status=%d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	return result.AccessToken, nil
}

func (h *ShopifyAuthHandler) registerWebhook(shopDomain, accessToken string) (int64, error) {
	client := shopify.NewAdminClient(shopDomain, accessToken)
	webhookURL := h.cfg.Shopify.AppURL + "/api/v1/webhooks/shopify/order-create"
	return client.RegisterOrderCreateWebhook(webhookURL)
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	if strings.HasPrefix(phone, "00") {
		return "+" + strings.TrimPrefix(phone, "00")
	}
	return phone
}

func extractDate(createdAt string) string {
	if createdAt == "" {
		return ""
	}
	if len(createdAt) >= 10 {
		return createdAt[:10]
	}
	return createdAt
}

func (h *ShopifyAuthHandler) configFormHTML(shopID, shopDomain string, isReconfig bool) string {
	title := "Configure WhatsApp"
	subtitle := "Connect your WhatsApp Business account to send order notifications."
	btnText := "Save Configuration"
	if isReconfig {
		title = "Update WhatsApp Configuration"
		subtitle = "Your shop is already configured. Update your WhatsApp credentials below."
		btnText = "Update Configuration"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s - Social Notif</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; display: flex; justify-content: center; align-items: center; min-height: 100vh; }
    .card { background: white; border-radius: 12px; padding: 40px; width: 100%%%%; max-width: 520px; box-shadow: 0 2px 12px rgba(0,0,0,0.08); }
    h1 { font-size: 24px; margin-bottom: 8px; color: #1a1a1a; }
    p { color: #666; margin-bottom: 24px; font-size: 14px; }
    label { display: block; font-size: 14px; font-weight: 600; color: #333; margin-bottom: 6px; }
    input { width: 100%%%%; padding: 10px 12px; border: 1px solid #ddd; border-radius: 8px; font-size: 14px; margin-bottom: 16px; }
    input:focus { outline: none; border-color: #2563eb; box-shadow: 0 0 0 3px rgba(37,99,235,0.1); }
    button { width: 100%%%%; padding: 12px; background: #2563eb; color: white; border: none; border-radius: 8px; font-size: 16px; font-weight: 600; cursor: pointer; }
    button:hover { background: #1d4ed8; }
    .hint { font-size: 12px; color: #888; margin-top: -12px; margin-bottom: 16px; }
    #status { margin-top: 16px; padding: 12px; border-radius: 8px; display: none; }
    .success { background: #dcfce7; color: #166534; }
    .error { background: #fef2f2; color: #991b1b; }
  </style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    <p>%s</p>
    <form id="configForm">
      <input type="hidden" name="shop_id" value="%s">
      <input type="hidden" name="setup_token" value="">
      <label for="phone_number_id">WhatsApp Phone Number ID</label>
      <input type="text" id="phone_number_id" name="phone_number_id" placeholder="e.g. 123456789012345" required>
      <label for="access_token">WhatsApp Access Token</label>
      <input type="password" id="access_token" name="access_token" placeholder="Paste your permanent access token" required>
      <label for="template_name">Template Name</label>
      <input type="text" id="template_name" name="template_name" placeholder="e.g. order_confirmation" value="jaspers_market_order_confirmation_v1" required>
      <label for="template_language">Template Language</label>
      <input type="text" id="template_language" name="template_language" placeholder="en_US" value="en_US">
      <button type="submit">%s</button>
    </form>
    <div id="status"></div>
  </div>
  <script>
    document.getElementById('configForm').addEventListener('submit', async function(e) {
      e.preventDefault();
      const status = document.getElementById('status');
      const btn = this.querySelector('button');
      btn.disabled = true;
      btn.textContent = 'Saving...';
      status.style.display = 'none';

      const data = new FormData(this);
      const body = new URLSearchParams(data);

      try {
        const res = await fetch('/api/v1/shopify/configure', {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: body.toString()
        });
        if (res.ok) {
          window.location.href = '/api/v1/shopify/setup-complete?shop=%s';
        } else {
          const json = await res.json();
          status.className = 'error';
          status.textContent = json?.error?.message || 'Failed to save configuration';
          status.style.display = 'block';
        }
      } catch (err) {
        status.className = 'error';
        status.textContent = 'Network error. Please try again.';
        status.style.display = 'block';
      } finally {
        btn.disabled = false;
        btn.textContent = '%s';
      }
    });

    fetch('/api/v1/shopify/setup-token?shop_id=%s')
      .then(r => r.json())
      .then(d => { document.querySelector('input[name="setup_token"]').value = d.token; })
      .catch(() => {});
  </script>
</body>
</html>`, title, title, subtitle, shopID, btnText, shopDomain, btnText, shopID)
}

func (h *ShopifyAuthHandler) successHTML(shopDomain string) string {
	shopifyAdminURL := fmt.Sprintf("https://%s/admin", shopDomain)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Setup Complete - Social Notif</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; display: flex; justify-content: center; align-items: center; min-height: 100vh; }
    .card { background: white; border-radius: 12px; padding: 48px; width: 100%%%%; max-width: 480px; box-shadow: 0 2px 12px rgba(0,0,0,0.08); text-align: center; }
    .checkmark { width: 64px; height: 64px; background: #dcfce7; border-radius: 50%%; display: flex; align-items: center; justify-content: center; margin: 0 auto 24px; font-size: 32px; color: #166534; }
    h1 { font-size: 24px; margin-bottom: 8px; color: #1a1a1a; }
    p { color: #666; margin-bottom: 8px; font-size: 14px; }
    .btn { display: inline-block; margin-top: 24px; padding: 12px 24px; background: #2563eb; color: white; text-decoration: none; border-radius: 8px; font-weight: 600; }
    .btn:hover { background: #1d4ed8; }
  </style>
</head>
<body>
  <div class="card">
    <div class="checkmark">&#10003;</div>
    <h1>Setup Complete!</h1>
    <p>Social Notif is now connected to your Shopify store.</p>
    <p>Customers will receive WhatsApp order confirmations automatically.</p>
    <a href="%s" class="btn">Return to Shopify Admin</a>
  </div>
</body>
</html>`, shopifyAdminURL)
}
