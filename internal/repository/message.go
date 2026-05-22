package repository

import (
	"context"
	"errors"
	"fmt"

	"social-notif/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrMessageNotFound = errors.New("message not found")

type MessageRepository interface {
	Create(ctx context.Context, message *model.Message) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.MessageStatus) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error)
}

type GormMessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) MessageRepository {
	return &GormMessageRepository{db: db}
}

func (r *GormMessageRepository) Create(ctx context.Context, message *model.Message) error {
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		return fmt.Errorf("create message: %w", err)
	}

	return nil
}

func (r *GormMessageRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.MessageStatus) error {
	result := r.db.WithContext(ctx).
		Model(&model.Message{}).
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

func (r *GormMessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	var message model.Message
	if err := r.db.WithContext(ctx).First(&message, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get message by id: %w", ErrMessageNotFound)
		}
		return nil, fmt.Errorf("get message by id: %w", err)
	}

	return &message, nil
}
