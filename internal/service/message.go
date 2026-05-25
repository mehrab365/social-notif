package service

import (
	"context"
	"encoding/json"
	"fmt"

	"social-notif/internal/apperror"
	"social-notif/internal/domain"
	"social-notif/internal/queue"
	"social-notif/internal/repository"

	"go.uber.org/zap"
)

type MessageService interface {
	EnqueueWhatsAppMessage(ctx context.Context, input EnqueueWhatsAppMessageInput) (EnqueueWhatsAppMessageResult, error)
}

type EnqueueWhatsAppMessageInput struct {
	PhoneNumber        string
	Body               string
	TemplateName       string
	TemplateLanguage   string
	TemplateBodyParams []string
}

type EnqueueWhatsAppMessageResult struct {
	Message domain.Message
}

type MessageServiceImpl struct {
	repo   repository.MessageRepository
	queue  queue.MessageQueue
	logger *zap.Logger
}

func NewMessageService(repo repository.MessageRepository, q queue.MessageQueue, logger *zap.Logger) MessageService {
	return &MessageServiceImpl{
		repo:   repo,
		queue:  q,
		logger: logger,
	}
}

var errEmptyPhoneNumber = fmt.Errorf("%w: phone number is required", apperror.ErrValidation)
var errEmptyBody = fmt.Errorf("%w: message body is required", apperror.ErrValidation)

func (s *MessageServiceImpl) EnqueueWhatsAppMessage(ctx context.Context, input EnqueueWhatsAppMessageInput) (EnqueueWhatsAppMessageResult, error) {
	if input.PhoneNumber == "" {
		return EnqueueWhatsAppMessageResult{}, errEmptyPhoneNumber
	}
	if input.Body == "" {
		return EnqueueWhatsAppMessageResult{}, errEmptyBody
	}

	msg := &domain.Message{
		PhoneNumber:      input.PhoneNumber,
		Body:             input.Body,
		Status:           domain.MessageStatusQueued,
		TemplateName:     input.TemplateName,
		TemplateLanguage: input.TemplateLanguage,
	}
	msg.EnsureDefaults()
	if msg.Status == "" {
		msg.Status = domain.MessageStatusQueued
	}

	if len(input.TemplateBodyParams) > 0 {
		params := domain.TemplateParams{
			Body: make([]domain.TemplateParam, 0, len(input.TemplateBodyParams)),
		}
		for _, p := range input.TemplateBodyParams {
			params.Body = append(params.Body, domain.TemplateParam{
				Type: "text",
				Text: p,
			})
		}
		raw, _ := json.Marshal(params)
		msg.TemplateParams = raw
	}

	if err := s.repo.Create(ctx, msg); err != nil {
		return EnqueueWhatsAppMessageResult{}, fmt.Errorf("persist message: %w", err)
	}

	if err := s.queue.EnqueueMessageDelivery(ctx, msg.ID); err != nil {
		s.logger.Error("message persisted but enqueue failed, leaving as pending",
			zap.String("message_id", msg.ID.String()),
			zap.Error(err),
		)
		recordErr := s.repo.UpdateStatus(ctx, msg.ID, domain.MessageStatusPending)
		if recordErr != nil {
			s.logger.Error("failed to update message status to pending after enqueue failure",
				zap.String("message_id", msg.ID.String()),
				zap.Error(recordErr),
			)
		}
		return EnqueueWhatsAppMessageResult{}, fmt.Errorf("enqueue delivery task: %w", err)
	}

	return EnqueueWhatsAppMessageResult{Message: *msg}, nil
}
