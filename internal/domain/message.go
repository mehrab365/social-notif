package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type MessageStatus string

const (
	MessageStatusPending         MessageStatus = "pending"
	MessageStatusQueued          MessageStatus = "queued"
	MessageStatusProcessing      MessageStatus = "processing"
	MessageStatusSent            MessageStatus = "sent"
	MessageStatusFailedRetryable MessageStatus = "failed_retryable"
	MessageStatusFailedPermanent MessageStatus = "failed_permanent"
)

type TemplateParams struct {
	Body []TemplateParam `json:"body,omitempty"`
}

type TemplateParam struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Message struct {
	ID               uuid.UUID
	PhoneNumber      string
	Body             string
	Status           MessageStatus
	ProviderResponse json.RawMessage
	RetryCount       int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	TemplateName     string
	TemplateLanguage string
	TemplateParams   json.RawMessage
}

func (m *Message) EnsureDefaults() {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Status == "" {
		m.Status = MessageStatusPending
	}
}
