package model

import (
	"encoding/json"
	"time"

	"social-notif/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageRecord struct {
	ID               uuid.UUID            `gorm:"type:uuid;primaryKey"`
	PhoneNumber      string               `gorm:"column:phone_number;type:varchar(32);not null;index"`
	Body             string               `gorm:"column:body;type:text;not null"`
	Status           domain.MessageStatus `gorm:"column:status;type:varchar(32);not null;default:'pending';index"`
	ProviderResponse json.RawMessage      `gorm:"column:provider_response;type:jsonb"`
	RetryCount       int                  `gorm:"column:retry_count;not null;default:0"`
	CreatedAt        time.Time            `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt        time.Time            `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (MessageRecord) TableName() string {
	return "messages"
}

func (m *MessageRecord) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Status == "" {
		m.Status = domain.MessageStatusPending
	}
	return nil
}

func MessageRecordFromDomain(message *domain.Message) *MessageRecord {
	if message == nil {
		return nil
	}

	message.EnsureDefaults()
	return &MessageRecord{
		ID:               message.ID,
		PhoneNumber:      message.PhoneNumber,
		Body:             message.Body,
		Status:           message.Status,
		ProviderResponse: message.ProviderResponse,
		RetryCount:       message.RetryCount,
		CreatedAt:        message.CreatedAt,
		UpdatedAt:        message.UpdatedAt,
	}
}

func (m *MessageRecord) ToDomain() *domain.Message {
	if m == nil {
		return nil
	}

	return &domain.Message{
		ID:               m.ID,
		PhoneNumber:      m.PhoneNumber,
		Body:             m.Body,
		Status:           m.Status,
		ProviderResponse: m.ProviderResponse,
		RetryCount:       m.RetryCount,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
