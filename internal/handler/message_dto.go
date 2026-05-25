package handler

import (
	"encoding/json"
	"time"

	"social-notif/internal/domain"
)

type CreateMessageRequest struct {
	PhoneNumber        string   `json:"phone_number" binding:"required,e164,max=32"`
	Body               string   `json:"body" binding:"required,min=1,max=4096"`
	TemplateName       string   `json:"template_name,omitempty"`
	TemplateLanguage   string   `json:"template_language,omitempty"`
	TemplateBodyParams []string `json:"template_body_params,omitempty"`
}

type MessageResponse struct {
	ID               string               `json:"id"`
	PhoneNumber      string               `json:"phone_number"`
	Body             string               `json:"body"`
	Status           domain.MessageStatus `json:"status"`
	ProviderResponse json.RawMessage      `json:"provider_response,omitempty"`
	RetryCount       int                  `json:"retry_count"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type MessageStatusResponse struct {
	ID         string               `json:"id"`
	Status     domain.MessageStatus `json:"status"`
	RetryCount int                  `json:"retry_count"`
	UpdatedAt  time.Time            `json:"updated_at"`
}
