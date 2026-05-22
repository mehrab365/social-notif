package service

import "context"

type MessageService interface {
	EnqueueWhatsAppMessage(ctx context.Context, input EnqueueWhatsAppMessageInput) (EnqueueWhatsAppMessageResult, error)
}

type EnqueueWhatsAppMessageInput struct{}

type EnqueueWhatsAppMessageResult struct{}
