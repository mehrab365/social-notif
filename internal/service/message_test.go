package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"social-notif/internal/apperror"
	"social-notif/internal/domain"
	"social-notif/internal/repository"
	"social-notif/internal/service"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type mockMessageRepo struct {
	createFunc         func(ctx context.Context, msg *domain.Message) error
	getByIDFunc        func(ctx context.Context, id uuid.UUID) (*domain.Message, error)
	updateStatusFunc   func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error
	recordDeliveryFunc func(ctx context.Context, id uuid.UUID, status domain.MessageStatus, resp json.RawMessage) error
	incrementRetryFunc func(ctx context.Context, id uuid.UUID) error
}

func (m *mockMessageRepo) Create(ctx context.Context, msg *domain.Message) error {
	return m.createFunc(ctx, msg)
}
func (m *mockMessageRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	return m.getByIDFunc(ctx, id)
}
func (m *mockMessageRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error {
	return m.updateStatusFunc(ctx, id, status)
}
func (m *mockMessageRepo) RecordDeliveryAttempt(ctx context.Context, id uuid.UUID, status domain.MessageStatus, resp json.RawMessage) error {
	return m.recordDeliveryFunc(ctx, id, status, resp)
}
func (m *mockMessageRepo) IncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	return m.incrementRetryFunc(ctx, id)
}

type mockMessageQueue struct {
	enqueueFunc func(ctx context.Context, messageID uuid.UUID) error
}

func (m *mockMessageQueue) EnqueueMessageDelivery(ctx context.Context, messageID uuid.UUID) error {
	return m.enqueueFunc(ctx, messageID)
}

func TestMessageService_EnqueueWhatsAppMessage(t *testing.T) {
	logger := zap.NewNop()

	t.Run("success", func(t *testing.T) {
		repo := &mockMessageRepo{
			createFunc: func(ctx context.Context, msg *domain.Message) error {
				msg.ID = uuid.New()
				return nil
			},
		}
		q := &mockMessageQueue{
			enqueueFunc: func(ctx context.Context, messageID uuid.UUID) error {
				return nil
			},
		}

		svc := service.NewMessageService(repo, q, logger)
		result, err := svc.EnqueueWhatsAppMessage(context.Background(), service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "+15551234567",
			Body:        "Hello, world!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Message.Status != domain.MessageStatusQueued {
			t.Fatalf("status = %s, want %s", result.Message.Status, domain.MessageStatusQueued)
		}
		if result.Message.ID == uuid.Nil {
			t.Fatal("message ID should not be nil")
		}
	})

	t.Run("empty phone number", func(t *testing.T) {
		svc := service.NewMessageService(nil, nil, logger)
		_, err := svc.EnqueueWhatsAppMessage(context.Background(), service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "",
			Body:        "hello",
		})
		if err == nil {
			t.Fatal("expected error for empty phone number")
		}
		if !errors.Is(err, apperror.ErrValidation) {
			t.Fatalf("error = %v, want ErrValidation", err)
		}
	})

	t.Run("empty body", func(t *testing.T) {
		svc := service.NewMessageService(nil, nil, logger)
		_, err := svc.EnqueueWhatsAppMessage(context.Background(), service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "+15551234567",
			Body:        "",
		})
		if err == nil {
			t.Fatal("expected error for empty body")
		}
		if !errors.Is(err, apperror.ErrValidation) {
			t.Fatalf("error = %v, want ErrValidation", err)
		}
	})

	t.Run("repo create failure", func(t *testing.T) {
		repo := &mockMessageRepo{
			createFunc: func(ctx context.Context, msg *domain.Message) error {
				return errors.New("db connection lost")
			},
		}
		svc := service.NewMessageService(repo, nil, logger)
		_, err := svc.EnqueueWhatsAppMessage(context.Background(), service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "+15551234567",
			Body:        "hello",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("enqueue failure rolls back to pending", func(t *testing.T) {
		var msgID uuid.UUID
		statusUpdated := false
		repo := &mockMessageRepo{
			createFunc: func(ctx context.Context, msg *domain.Message) error {
				msg.ID = uuid.New()
				msgID = msg.ID
				return nil
			},
			updateStatusFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error {
				if id == msgID && status == domain.MessageStatusPending {
					statusUpdated = true
				}
				return nil
			},
		}
		q := &mockMessageQueue{
			enqueueFunc: func(ctx context.Context, messageID uuid.UUID) error {
				return errors.New("redis connection lost")
			},
		}

		svc := service.NewMessageService(repo, q, logger)
		_, err := svc.EnqueueWhatsAppMessage(context.Background(), service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "+15551234567",
			Body:        "hello",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !statusUpdated {
			t.Fatal("expected status to be rolled back to pending")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		repo := &mockMessageRepo{
			createFunc: func(ctx context.Context, msg *domain.Message) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					return nil
				}
			},
		}
		q := &mockMessageQueue{}
		svc := service.NewMessageService(repo, q, logger)
		_, err := svc.EnqueueWhatsAppMessage(ctx, service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "+15551234567",
			Body:        "hello",
		})
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
	})

	t.Run("status rollback failure logged but still returns error", func(t *testing.T) {
		var msgID uuid.UUID
		repo := &mockMessageRepo{
			createFunc: func(ctx context.Context, msg *domain.Message) error {
				msg.ID = uuid.New()
				msgID = msg.ID
				return nil
			},
			updateStatusFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error {
				if id == msgID && status == domain.MessageStatusPending {
					return repository.ErrMessageNotFound
				}
				return nil
			},
		}
		q := &mockMessageQueue{
			enqueueFunc: func(ctx context.Context, messageID uuid.UUID) error {
				return errors.New("enqueue failed")
			},
		}
		svc := service.NewMessageService(repo, q, logger)
		_, err := svc.EnqueueWhatsAppMessage(context.Background(), service.EnqueueWhatsAppMessageInput{
			PhoneNumber: "+15551234567",
			Body:        "hello",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
