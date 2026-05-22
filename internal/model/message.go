package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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

type Message struct {
	ID               uuid.UUID       `gorm:"type:uuid;primaryKey"`
	PhoneNumber      string          `gorm:"column:phone_number;type:varchar(32);not null;index"`
	Body             string          `gorm:"column:body;type:text;not null"`
	Status           MessageStatus   `gorm:"column:status;type:varchar(32);not null;default:'pending';index"`
	ProviderResponse json.RawMessage `gorm:"column:provider_response;type:jsonb"`
	RetryCount       int             `gorm:"column:retry_count;not null;default:0"`
	CreatedAt        time.Time       `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt        time.Time       `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (Message) TableName() string {
	return "messages"
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Status == "" {
		m.Status = MessageStatusPending
	}
	return nil
}
