package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"social-notif/internal/domain"
	"social-notif/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMessageNotFound = errors.New("message not found")

type MessageRepository interface {
	Create(ctx context.Context, message *domain.Message) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error
	RecordDeliveryAttempt(ctx context.Context, id uuid.UUID, status domain.MessageStatus, providerResponse json.RawMessage) error
	IncrementRetryCount(ctx context.Context, id uuid.UUID) error
}

type GormMessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &GormMessageRepository{db: db}
}

func (r *GormMessageRepository) Create(ctx context.Context, message *domain.Message) error {
	record := model.MessageRecordFromDomain(message)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	*message = *record.ToDomain()
	return nil
}

func (r *GormMessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error {
	result := r.db.WithContext(ctx).
		Model(&model.MessageRecord{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("update message status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("update message status: %w", ErrMessageNotFound)
	}

	return nil
}

func (r *GormMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	var record model.MessageRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get message by id: %w", ErrMessageNotFound)
		}
		return nil, fmt.Errorf("get message by id: %w", err)
	}

	return record.ToDomain(), nil
}

func (r *GormMessageRepository) RecordDeliveryAttempt(ctx context.Context, id uuid.UUID, status domain.MessageStatus, providerResponse json.RawMessage) error {
	var resp any
	if len(providerResponse) > 0 {
		resp = providerResponse
	}

	result := r.db.WithContext(ctx).
		Model(&model.MessageRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":            status,
			"provider_response": resp,
		})
	if result.Error != nil {
		return fmt.Errorf("record delivery attempt: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("record delivery attempt: %w", ErrMessageNotFound)
	}

	return nil
}

func (r *GormMessageRepository) IncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&model.MessageRecord{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1"))
	if result.Error != nil {
		return fmt.Errorf("increment retry count: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("increment retry count: %w", ErrMessageNotFound)
	}
	return nil
}
