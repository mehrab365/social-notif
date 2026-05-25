package model

import (
	"time"

	"social-notif/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShopRecord struct {
	ID                       string    `gorm:"type:uuid;primaryKey"`
	ShopDomain               string    `gorm:"column:shop_domain;type:varchar(255);not null;uniqueIndex"`
	ShopifyAccessToken       string    `gorm:"column:shopify_access_token;type:varchar(512);not null;default:''"`
	WhatsAppAccessToken      string    `gorm:"column:whatsapp_access_token;type:varchar(512);not null;default:''"`
	WhatsAppPhoneNumberID    string    `gorm:"column:whatsapp_phone_number_id;type:varchar(64);not null;default:''"`
	WhatsAppTemplateName     string    `gorm:"column:whatsapp_template_name;type:varchar(128);not null;default:''"`
	WhatsAppTemplateLanguage string    `gorm:"column:whatsapp_template_language;type:varchar(16);not null;default:'en_US'"`
	SetupToken               string    `gorm:"column:setup_token;type:varchar(64)"`
	WebhookID                int64     `gorm:"column:webhook_id;not null;default:0"`
	CreatedAt                time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt                time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (ShopRecord) TableName() string {
	return "shops"
}

func (r *ShopRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

func ShopRecordFromDomain(s *domain.Shop) *ShopRecord {
	if s == nil {
		return nil
	}
	id := s.ID
	if id == "" {
		id = uuid.New().String()
	}
	return &ShopRecord{
		ID:                       id,
		ShopDomain:               s.ShopDomain,
		ShopifyAccessToken:       s.ShopifyAccessToken,
		WhatsAppAccessToken:      s.WhatsAppAccessToken,
		WhatsAppPhoneNumberID:    s.WhatsAppPhoneNumberID,
		WhatsAppTemplateName:     s.WhatsAppTemplateName,
		WhatsAppTemplateLanguage: s.WhatsAppTemplateLanguage,
		SetupToken:               s.SetupToken,
		WebhookID:                s.WebhookID,
		CreatedAt:                s.CreatedAt,
		UpdatedAt:                s.UpdatedAt,
	}
}

func (r *ShopRecord) ToDomain() *domain.Shop {
	if r == nil {
		return nil
	}
	return &domain.Shop{
		ID:                       r.ID,
		ShopDomain:               r.ShopDomain,
		ShopifyAccessToken:       r.ShopifyAccessToken,
		WhatsAppAccessToken:      r.WhatsAppAccessToken,
		WhatsAppPhoneNumberID:    r.WhatsAppPhoneNumberID,
		WhatsAppTemplateName:     r.WhatsAppTemplateName,
		WhatsAppTemplateLanguage: r.WhatsAppTemplateLanguage,
		SetupToken:               r.SetupToken,
		WebhookID:                r.WebhookID,
		CreatedAt:                r.CreatedAt,
		UpdatedAt:                r.UpdatedAt,
	}
}
