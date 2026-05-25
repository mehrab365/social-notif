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
	TemplateName     string               `gorm:"column:template_name;type:varchar(128)"`
	TemplateLanguage string               `gorm:"column:template_language;type:varchar(16);not null;default:'en_US'"`
	TemplateParams   json.RawMessage      `gorm:"column:template_params;type:jsonb"`
	ShopID           uuid.UUID            `gorm:"column:shop_id;type:uuid"`
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
	var shopID uuid.UUID
	if message.ShopID != "" {
		shopID = uuid.MustParse(message.ShopID)
	}
	return &MessageRecord{
		ID:               message.ID,
		PhoneNumber:      message.PhoneNumber,
		Body:             message.Body,
		Status:           message.Status,
		ProviderResponse: message.ProviderResponse,
		RetryCount:       message.RetryCount,
		CreatedAt:        message.CreatedAt,
		UpdatedAt:        message.UpdatedAt,
		TemplateName:     message.TemplateName,
		TemplateLanguage: message.TemplateLanguage,
		TemplateParams:   message.TemplateParams,
		ShopID:           shopID,
	}
}

func (m *MessageRecord) ToDomain() *domain.Message {
	if m == nil {
		return nil
	}

	var shopID string
	if m.ShopID != uuid.Nil {
		shopID = m.ShopID.String()
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
		TemplateName:     m.TemplateName,
		TemplateLanguage: m.TemplateLanguage,
		TemplateParams:   m.TemplateParams,
		ShopID:           shopID,
	}
}
