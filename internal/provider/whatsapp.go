package provider

import "context"

type WhatsAppProvider interface {
	SendMessage(ctx context.Context, request SendMessageRequest) (SendMessageResponse, error)
}

type SendMessageRequest struct{}

type SendMessageResponse struct{}
