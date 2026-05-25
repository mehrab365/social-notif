package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"social-notif/internal/apperror"
	"social-notif/internal/config"
	"social-notif/internal/domain"
)

type WhatsAppProvider interface {
	SendMessage(ctx context.Context, request SendMessageRequest) (SendMessageResponse, error)
}

type SendMessageRequest struct {
	PhoneNumber      string
	Body             string
	TemplateName     string
	TemplateLanguage string
	TemplateParams   json.RawMessage
}

type SendMessageResponse struct {
	MessageID string `json:"message_id"`
	ContactID string `json:"contact_id"`
}

type MetaWhatsAppClient struct {
	baseURL       string
	apiVersion    string
	phoneNumberID string
	accessToken   string
	httpClient    *http.Client
}

func NewMetaWhatsAppClient(cfg config.WhatsAppConfig) *MetaWhatsAppClient {
	return &MetaWhatsAppClient{
		baseURL:       cfg.BaseURL,
		apiVersion:    cfg.APIVersion,
		phoneNumberID: cfg.PhoneNumberID,
		accessToken:   cfg.AccessToken,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

type metaMessageRequest struct {
	MessagingProduct string               `json:"messaging_product"`
	RecipientType    string               `json:"recipient_type"`
	To               string               `json:"to"`
	Type             string               `json:"type"`
	Text             *metaTextContent     `json:"text,omitempty"`
	Template         *metaTemplateContent `json:"template,omitempty"`
}

type metaTemplateContent struct {
	Name       string                  `json:"name"`
	Language   metaTemplateLanguage    `json:"language"`
	Components []metaTemplateComponent `json:"components,omitempty"`
}

type metaTemplateLanguage struct {
	Code string `json:"code"`
}

type metaTemplateComponent struct {
	Type       string              `json:"type"`
	Parameters []metaTemplateParam `json:"parameters"`
}

type metaTemplateParam struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type metaTextContent struct {
	PreviewURL bool   `json:"preview_url"`
	Body       string `json:"body"`
}

type metaMessageResponse struct {
	MessagingProduct string                    `json:"messaging_product"`
	Contacts         []metaContactResponse     `json:"contacts"`
	Messages         []metaMessageResponseItem `json:"messages"`
	Error            *metaErrorResponse        `json:"error,omitempty"`
}

type metaContactResponse struct {
	Input string `json:"input"`
	WaID  string `json:"wa_id"`
}

type metaMessageResponseItem struct {
	ID string `json:"id"`
}

type metaErrorResponse struct {
	Message   string         `json:"message"`
	Type      string         `json:"type"`
	Code      int            `json:"code"`
	ErrorData *metaErrorData `json:"error_data,omitempty"`
}

type metaErrorData struct {
	Details string `json:"details"`
}

func (c *MetaWhatsAppClient) SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResponse, error) {
	body := c.buildRequest(req)

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return SendMessageResponse{}, err
	}

	if len(resp.Messages) == 0 {
		return SendMessageResponse{}, fmt.Errorf("%w: whatsapp api returned no message ids", apperror.ErrProviderPermanent)
	}

	contactID := ""
	if len(resp.Contacts) > 0 {
		contactID = resp.Contacts[0].WaID
	}

	return SendMessageResponse{
		MessageID: resp.Messages[0].ID,
		ContactID: contactID,
	}, nil
}

func (c *MetaWhatsAppClient) buildRequest(req SendMessageRequest) metaMessageRequest {
	if req.TemplateName != "" {
		return c.buildTemplateRequest(req)
	}
	return c.buildTextRequest(req)
}

func (c *MetaWhatsAppClient) buildTextRequest(req SendMessageRequest) metaMessageRequest {
	return metaMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               req.PhoneNumber,
		Type:             "text",
		Text: &metaTextContent{
			PreviewURL: false,
			Body:       req.Body,
		},
	}
}

func (c *MetaWhatsAppClient) buildTemplateRequest(req SendMessageRequest) metaMessageRequest {
	lang := req.TemplateLanguage
	if lang == "" {
		lang = "en_US"
	}

	tmpl := metaTemplateContent{
		Name:     req.TemplateName,
		Language: metaTemplateLanguage{Code: lang},
	}

	if len(req.TemplateParams) > 0 {
		var params domain.TemplateParams
		if err := json.Unmarshal(req.TemplateParams, &params); err == nil && len(params.Body) > 0 {
			components := make([]metaTemplateComponent, 0, 1)
			component := metaTemplateComponent{
				Type:       "body",
				Parameters: make([]metaTemplateParam, 0, len(params.Body)),
			}
			for _, p := range params.Body {
				component.Parameters = append(component.Parameters, metaTemplateParam{
					Type: p.Type,
					Text: p.Text,
				})
			}
			components = append(components, component)
			tmpl.Components = components
		}
	}

	return metaMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               req.PhoneNumber,
		Type:             "template",
		Template:         &tmpl,
	}
}

func (c *MetaWhatsAppClient) doRequest(ctx context.Context, body any) (*metaMessageResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint, err := url.JoinPath(c.baseURL, c.apiVersion, c.phoneNumberID, "messages")
	if err != nil {
		return nil, fmt.Errorf("build endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("whatsapp api request canceled: %w", err)
		}
		return nil, fmt.Errorf("%w: http request: %w", apperror.ErrProviderTemporary, err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if err := c.checkError(httpResp.StatusCode, respBody); err != nil {
		return nil, err
	}

	var result metaMessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *MetaWhatsAppClient) checkError(statusCode int, body []byte) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	var metaErr metaMessageResponse
	if err := json.Unmarshal(body, &metaErr); err == nil && metaErr.Error != nil {
		msg := fmt.Sprintf("whatsapp api error (status=%d, code=%d): %s",
			statusCode, metaErr.Error.Code, metaErr.Error.Message)
		if metaErr.Error.ErrorData != nil {
			msg += ": " + metaErr.Error.ErrorData.Details
		}

		switch {
		case statusCode == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %s", apperror.ErrProviderTemporary, msg)
		case statusCode >= 500:
			return fmt.Errorf("%w: %s", apperror.ErrProviderTemporary, msg)
		default:
			return fmt.Errorf("%w: %s", apperror.ErrProviderPermanent, msg)
		}
	}

	switch {
	case statusCode == http.StatusTooManyRequests:
		return fmt.Errorf("%w: whatsapp api rate limited (status=%d)", apperror.ErrProviderTemporary, statusCode)
	case statusCode >= 500:
		return fmt.Errorf("%w: whatsapp api server error (status=%d)", apperror.ErrProviderTemporary, statusCode)
	default:
		return fmt.Errorf("%w: whatsapp api client error (status=%d, body=%s)", apperror.ErrProviderPermanent, statusCode, string(body))
	}
}
