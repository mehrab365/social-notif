package service

import (
	"context"

	"social-notif/internal/domain"
)

type MessageService interface {
	EnqueueWhatsAppMessage(ctx context.Context, input EnqueueWhatsAppMessageInput) (EnqueueWhatsAppMessageResult, error)
}

type EnqueueWhatsAppMessageInput struct {
	PhoneNumber string
	Body        string
}

type EnqueueWhatsAppMessageResult struct {
	Message domain.Message
}
