package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"social-notif/internal/apperror"
	"social-notif/internal/config"
	"social-notif/internal/domain"
	"social-notif/internal/provider"
	"social-notif/internal/repository"
	"social-notif/internal/worker"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
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

type mockProvider struct {
	sendMessageFunc func(ctx context.Context, req provider.SendMessageRequest) (provider.SendMessageResponse, error)
}

func (m *mockProvider) SendMessage(ctx context.Context, req provider.SendMessageRequest) (provider.SendMessageResponse, error) {
	return m.sendMessageFunc(ctx, req)
}

func TestDeliveryHandler_Handle(t *testing.T) {
	logger := zap.NewNop()
	cfg := testConfig()

	t.Run("successful delivery", func(t *testing.T) {
		msgID := uuid.New()

		repo := &mockMessageRepo{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
				return &domain.Message{
					ID:          msgID,
					PhoneNumber: "+15551234567",
					Body:        "Hello!",
					Status:      domain.MessageStatusQueued,
				}, nil
			},
			updateStatusFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error {
				return nil
			},
			incrementRetryFunc: func(ctx context.Context, id uuid.UUID) error {
				return nil
			},
			recordDeliveryFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus, resp json.RawMessage) error {
				if status != domain.MessageStatusSent {
					t.Fatalf("status = %s, want %s", status, domain.MessageStatusSent)
				}
				return nil
			},
		}
		p := &mockProvider{
			sendMessageFunc: func(ctx context.Context, req provider.SendMessageRequest) (provider.SendMessageResponse, error) {
				return provider.SendMessageResponse{
					MessageID: "wamid.123",
				}, nil
			},
		}

		deps := worker.Dependencies{Config: cfg, Logger: logger, DB: nil, MessageRepo: repo, Provider: p}
		h := worker.NewDeliveryHandler(deps)
		task := newTask(t, msgID)

		err := h.Handle(context.Background(), task)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("message not found skips retry", func(t *testing.T) {
		repo := &mockMessageRepo{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
				return nil, repository.ErrMessageNotFound
			},
		}
		p := &mockProvider{}

		deps := worker.Dependencies{Config: cfg, Logger: logger, DB: nil, MessageRepo: repo, Provider: p}
		h := worker.NewDeliveryHandler(deps)
		task := newTask(t, uuid.New())

		err := h.Handle(context.Background(), task)
		if !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("error = %v, want SkipRetry", err)
		}
	})

	t.Run("permanent provider error skips retry", func(t *testing.T) {
		msgID := uuid.New()

		repo := &mockMessageRepo{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
				return &domain.Message{ID: msgID, PhoneNumber: "invalid", Body: "test"}, nil
			},
			updateStatusFunc:   func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error { return nil },
			incrementRetryFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
			recordDeliveryFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus, resp json.RawMessage) error {
				if status != domain.MessageStatusFailedPermanent {
					t.Fatalf("status = %s, want %s", status, domain.MessageStatusFailedPermanent)
				}
				return nil
			},
		}
		p := &mockProvider{
			sendMessageFunc: func(ctx context.Context, req provider.SendMessageRequest) (provider.SendMessageResponse, error) {
				return provider.SendMessageResponse{}, apperror.ErrProviderPermanent
			},
		}

		deps := worker.Dependencies{Config: cfg, Logger: logger, DB: nil, MessageRepo: repo, Provider: p}
		h := worker.NewDeliveryHandler(deps)
		task := newTask(t, msgID)

		err := h.Handle(context.Background(), task)
		if !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("error = %v, want SkipRetry", err)
		}
	})

	t.Run("temporary provider error returns error for retry", func(t *testing.T) {
		msgID := uuid.New()

		repo := &mockMessageRepo{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
				return &domain.Message{ID: msgID, PhoneNumber: "+15551234567", Body: "test"}, nil
			},
			updateStatusFunc:   func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error { return nil },
			incrementRetryFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
			recordDeliveryFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus, resp json.RawMessage) error {
				if status != domain.MessageStatusFailedRetryable {
					t.Fatalf("status = %s, want %s", status, domain.MessageStatusFailedRetryable)
				}
				return nil
			},
		}
		p := &mockProvider{
			sendMessageFunc: func(ctx context.Context, req provider.SendMessageRequest) (provider.SendMessageResponse, error) {
				return provider.SendMessageResponse{}, apperror.ErrProviderTemporary
			},
		}

		deps := worker.Dependencies{Config: cfg, Logger: logger, DB: nil, MessageRepo: repo, Provider: p}
		h := worker.NewDeliveryHandler(deps)
		task := newTask(t, msgID)

		err := h.Handle(context.Background(), task)
		if err == nil {
			t.Fatal("expected retryable error")
		}
		if errors.Is(err, asynq.SkipRetry) {
			t.Fatal("expected retry, not SkipRetry")
		}
	})

	t.Run("invalid payload returns error", func(t *testing.T) {
		deps := worker.Dependencies{Config: cfg, Logger: logger}
		h := worker.NewDeliveryHandler(deps)
		task := asynq.NewTask("whatsapp:deliver_message", []byte("{invalid"))
		err := h.Handle(context.Background(), task)
		if err == nil {
			t.Fatal("expected error for invalid payload")
		}
	})

	t.Run("invalid message id in payload", func(t *testing.T) {
		deps := worker.Dependencies{Config: cfg, Logger: logger}
		payload, _ := json.Marshal(worker.MessageDeliveryPayload{MessageID: "not-a-uuid"})
		task := asynq.NewTask("whatsapp:deliver_message", payload)
		h := worker.NewDeliveryHandler(deps)
		err := h.Handle(context.Background(), task)
		if err == nil {
			t.Fatal("expected error for invalid uuid")
		}
	})

	t.Run("context cancellation during provider call", func(t *testing.T) {
		msgID := uuid.New()
		getByIDCalled := false

		repo := &mockMessageRepo{
			getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
				getByIDCalled = true
				return &domain.Message{ID: msgID, PhoneNumber: "+15551234567", Body: "test"}, nil
			},
			updateStatusFunc:   func(ctx context.Context, id uuid.UUID, status domain.MessageStatus) error { return nil },
			incrementRetryFunc: func(ctx context.Context, id uuid.UUID) error { return nil },
			recordDeliveryFunc: func(ctx context.Context, id uuid.UUID, status domain.MessageStatus, resp json.RawMessage) error {
				return nil
			},
		}
		p := &mockProvider{
			sendMessageFunc: func(ctx context.Context, req provider.SendMessageRequest) (provider.SendMessageResponse, error) {
				<-ctx.Done()
				return provider.SendMessageResponse{}, ctx.Err()
			},
		}
		deps := worker.Dependencies{Config: cfg, Logger: logger, DB: nil, MessageRepo: repo, Provider: p}
		h := worker.NewDeliveryHandler(deps)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		task := newTask(t, msgID)
		err := h.Handle(ctx, task)
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
		if !getByIDCalled {
			t.Fatal("expected GetByID to be called")
		}
	})
}

func newTask(t *testing.T, id uuid.UUID) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(worker.MessageDeliveryPayload{MessageID: id.String()})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return asynq.NewTask(worker.TaskDeliverWhatsAppMessage, payload)
}

func testConfig() *config.Config {
	return &config.Config{
		Queue: config.QueueConfig{Concurrency: 10, DefaultPriority: 1},
	}
}
