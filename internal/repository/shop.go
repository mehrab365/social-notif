package repository

import (
	"context"
	"errors"
	"fmt"

	"social-notif/internal/domain"
	"social-notif/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrShopNotFound = errors.New("shop not found")

type ShopRepository interface {
	Create(ctx context.Context, shop *domain.Shop) error
	GetByID(ctx context.Context, id string) (*domain.Shop, error)
	GetByDomain(ctx context.Context, domain string) (*domain.Shop, error)
	UpdateWhatsAppConfig(ctx context.Context, id string, config domain.ShopWhatsAppConfig) error
	SetSetupToken(ctx context.Context, id, token string) error
	ClearSetupToken(ctx context.Context, id string) error
	UpdateWebhookID(ctx context.Context, id string, webhookID int64) error
}

type GormShopRepository struct {
	db *gorm.DB
}

func NewShopRepository(db *gorm.DB) ShopRepository {
	return &GormShopRepository{db: db}
}

func (r *GormShopRepository) Create(ctx context.Context, shop *domain.Shop) error {
	if shop.ID == "" {
		shop.ID = uuid.New().String()
	}
	record := model.ShopRecordFromDomain(shop)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create shop: %w", err)
	}
	*shop = *record.ToDomain()
	return nil
}

func (r *GormShopRepository) GetByID(ctx context.Context, id string) (*domain.Shop, error) {
	var record model.ShopRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get shop by id: %w", ErrShopNotFound)
		}
		return nil, fmt.Errorf("get shop by id: %w", err)
	}
	return record.ToDomain(), nil
}

func (r *GormShopRepository) GetByDomain(ctx context.Context, shopDomain string) (*domain.Shop, error) {
	var record model.ShopRecord
	if err := r.db.WithContext(ctx).First(&record, "shop_domain = ?", shopDomain).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get shop by domain: %w", ErrShopNotFound)
		}
		return nil, fmt.Errorf("get shop by domain: %w", err)
	}
	return record.ToDomain(), nil
}

func (r *GormShopRepository) UpdateWhatsAppConfig(ctx context.Context, id string, config domain.ShopWhatsAppConfig) error {
	result := r.db.WithContext(ctx).
		Model(&model.ShopRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"whatsapp_access_token":      config.AccessToken,
			"whatsapp_phone_number_id":   config.PhoneNumberID,
			"whatsapp_template_name":     config.TemplateName,
			"whatsapp_template_language": config.TemplateLanguage,
		})
	if result.Error != nil {
		return fmt.Errorf("update shop whatsapp config: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update shop whatsapp config: %w", ErrShopNotFound)
	}
	return nil
}

func (r *GormShopRepository) SetSetupToken(ctx context.Context, id, token string) error {
	result := r.db.WithContext(ctx).
		Model(&model.ShopRecord{}).
		Where("id = ?", id).
		Update("setup_token", token)
	if result.Error != nil {
		return fmt.Errorf("set setup token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("set setup token: %w", ErrShopNotFound)
	}
	return nil
}

func (r *GormShopRepository) ClearSetupToken(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).
		Model(&model.ShopRecord{}).
		Where("id = ?", id).
		Update("setup_token", "")
	if result.Error != nil {
		return fmt.Errorf("clear setup token: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("clear setup token: %w", ErrShopNotFound)
	}
	return nil
}

func (r *GormShopRepository) UpdateWebhookID(ctx context.Context, id string, webhookID int64) error {
	result := r.db.WithContext(ctx).
		Model(&model.ShopRecord{}).
		Where("id = ?", id).
		Update("webhook_id", webhookID)
	if result.Error != nil {
		return fmt.Errorf("update webhook id: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update webhook id: %w", ErrShopNotFound)
	}
	return nil
}
