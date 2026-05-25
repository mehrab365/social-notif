package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-notif/internal/domain"
	"social-notif/internal/handler"
	"social-notif/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type mockMessageService struct {
	enqueueFunc func(ctx context.Context, input service.EnqueueWhatsAppMessageInput) (service.EnqueueWhatsAppMessageResult, error)
}

func (m *mockMessageService) EnqueueWhatsAppMessage(ctx context.Context, input service.EnqueueWhatsAppMessageInput) (service.EnqueueWhatsAppMessageResult, error) {
	return m.enqueueFunc(ctx, input)
}

func setupTestRouter(h *handler.MessageHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/messages/whatsapp", h.CreateWhatsApp)
	return r
}

func TestMessageHandler_CreateWhatsApp(t *testing.T) {
	logger := zap.NewNop()

	t.Run("success", func(t *testing.T) {
		svc := &mockMessageService{
			enqueueFunc: func(ctx context.Context, input service.EnqueueWhatsAppMessageInput) (service.EnqueueWhatsAppMessageResult, error) {
				return service.EnqueueWhatsAppMessageResult{
					Message: domain.Message{
						ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
						PhoneNumber: "+15551234567",
						Body:        "Hello!",
						Status:      domain.MessageStatusQueued,
					},
				}, nil
			},
		}
		h := handler.NewMessageHandler(svc, logger)
		r := setupTestRouter(h)

		body := map[string]string{
			"phone_number": "+15551234567",
			"body":         "Hello!",
		}
		resp := sendRequest(t, r, body)

		if resp.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusAccepted)
		}

		var result struct {
			Data handler.MessageStatusResponse `json:"data"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if result.Data.Status != domain.MessageStatusQueued {
			t.Fatalf("status = %s, want %s", result.Data.Status, domain.MessageStatusQueued)
		}
	})

	t.Run("validation error - missing phone", func(t *testing.T) {
		h := handler.NewMessageHandler(&mockMessageService{}, logger)
		r := setupTestRouter(h)

		body := map[string]string{
			"body": "Hello!",
		}
		resp := sendRequest(t, r, body)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})

	t.Run("validation error - empty body", func(t *testing.T) {
		h := handler.NewMessageHandler(&mockMessageService{}, logger)
		r := setupTestRouter(h)

		body := map[string]string{
			"phone_number": "+15551234567",
			"body":         "",
		}
		resp := sendRequest(t, r, body)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})

	t.Run("service error returns 500", func(t *testing.T) {
		svc := &mockMessageService{
			enqueueFunc: func(ctx context.Context, input service.EnqueueWhatsAppMessageInput) (service.EnqueueWhatsAppMessageResult, error) {
				return service.EnqueueWhatsAppMessageResult{}, errors.New("internal error")
			},
		}
		h := handler.NewMessageHandler(svc, logger)
		r := setupTestRouter(h)

		body := map[string]string{
			"phone_number": "+15551234567",
			"body":         "Hello!",
		}
		resp := sendRequest(t, r, body)

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
		}
	})

	t.Run("bad JSON body", func(t *testing.T) {
		h := handler.NewMessageHandler(&mockMessageService{}, logger)
		r := setupTestRouter(h)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/whatsapp", bytes.NewReader([]byte("{invalid")))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})
}

func sendRequest(t *testing.T, r *gin.Engine, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/whatsapp", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	return resp
}
