package shopify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AdminClient struct {
	shopDomain  string
	accessToken string
	apiVersion  string
	httpClient  *http.Client
}

func NewAdminClient(shopDomain, accessToken string) *AdminClient {
	return &AdminClient{
		shopDomain:  shopDomain,
		accessToken: accessToken,
		apiVersion:  "2026-04",
		httpClient:  &http.Client{},
	}
}

type webhookInput struct {
	Webhook webhookData `json:"webhook"`
}

type webhookData struct {
	Address string `json:"address"`
	Topic   string `json:"topic"`
	Format  string `json:"format"`
}

type webhookResponse struct {
	Webhook webhookResult `json:"webhook"`
}

type webhookResult struct {
	ID int64 `json:"id"`
}

func (c *AdminClient) RegisterOrderCreateWebhook(webhookURL string) (int64, error) {
	body := webhookInput{
		Webhook: webhookData{
			Address: webhookURL,
			Topic:   "orders/create",
			Format:  "json",
		},
	}

	payload, _ := json.Marshal(body)
	endpoint := fmt.Sprintf("https://%s/admin/api/%s/webhooks.json", c.shopDomain, c.apiVersion)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Shopify-Access-Token", c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("shopify api error (status=%d): %s", resp.StatusCode, string(respBody))
	}

	var result webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}

	return result.Webhook.ID, nil
}
