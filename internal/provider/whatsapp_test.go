package provider_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-notif/internal/apperror"
	"social-notif/internal/config"
	"social-notif/internal/provider"
)

func TestMetaWhatsAppClient_SendMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusOK, map[string]any{
			"messaging_product": "whatsapp",
			"contacts": []map[string]string{
				{"input": "+15551234567", "wa_id": "15551234567"},
			},
			"messages": []map[string]string{
				{"id": "wamid.HBgzNTUxMjM0NTY3NxUC"},
			},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		resp, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
			PhoneNumber: "+15551234567",
			Body:        "Hello!",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.MessageID != "wamid.HBgzNTUxMjM0NTY3NxUC" {
			t.Fatalf("message_id = %s, want wamid.HBgzNTUxMjM0NTY3NxUC", resp.MessageID)
		}
		if resp.ContactID != "15551234567" {
			t.Fatalf("contact_id = %s, want 15551234567", resp.ContactID)
		}
	})

	t.Run("permanent error - 401", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"message": "Invalid access token",
				"type":    "OAuthException",
				"code":    190,
			},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
			PhoneNumber: "+15551234567",
			Body:        "Hello!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !isPermanentError(err) {
			t.Fatalf("error should be permanent, got: %v", err)
		}
	})

	t.Run("retryable error - 429", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusTooManyRequests, map[string]any{
			"error": map[string]any{
				"message": "Rate limit hit",
				"type":    "RateLimit",
				"code":    80004,
			},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
			PhoneNumber: "+15551234567",
			Body:        "Hello!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !isTemporaryError(err) {
			t.Fatalf("error should be temporary, got: %v", err)
		}
	})

	t.Run("retryable error - 500", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"message": "Internal server error",
				"type":    "Server",
				"code":    2,
			},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
			PhoneNumber: "+15551234567",
			Body:        "Hello!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !isTemporaryError(err) {
			t.Fatalf("error should be temporary, got: %v", err)
		}
	})

	t.Run("permanent error - 400 invalid phone", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusBadRequest, map[string]any{
			"error": map[string]any{
				"message": "Invalid parameter",
				"type":    "GraphMethodException",
				"code":    100,
				"error_data": map[string]string{
					"details": "Phone number is not valid",
				},
			},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
			PhoneNumber: "invalid",
			Body:        "Hello!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !isPermanentError(err) {
			t.Fatalf("error should be permanent, got: %v", err)
		}
	})

	t.Run("no message ids in response", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusOK, map[string]any{
			"messaging_product": "whatsapp",
			"contacts":          []map[string]string{},
			"messages":          []map[string]string{},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		_, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
			PhoneNumber: "+15551234567",
			Body:        "Hello!",
		})
		if err == nil {
			t.Fatal("expected error")
		}
		if !isPermanentError(err) {
			t.Fatalf("error should be permanent, got: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		srv := newMetaTestServer(t, http.StatusOK, map[string]any{
			"messaging_product": "whatsapp",
			"contacts":          []map[string]string{{"input": "+15551234567", "wa_id": "15551234567"}},
			"messages":          []map[string]string{{"id": "wamid.123"}},
		})
		defer srv.Close()

		client := newTestClient(t, srv.URL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.SendMessage(ctx, provider.SendMessageRequest{
			PhoneNumber: "+15551234567",
			Body:        "Hello!",
		})
		if err == nil {
			t.Fatal("expected error for cancelled context")
		}
	})
}

func newMetaTestServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func newTestClient(t *testing.T, baseURL string) provider.WhatsAppProvider {
	t.Helper()
	cfg := config.WhatsAppConfig{
		BaseURL:       baseURL,
		APIVersion:    "v20.0",
		PhoneNumberID: "1234567890",
		AccessToken:   "test-token",
		Timeout:       0,
	}
	return provider.NewMetaWhatsAppClient(cfg)
}

func isPermanentError(err error) bool {
	return isSentinelError(err, apperror.ErrProviderPermanent)
}

func isTemporaryError(err error) bool {
	return isSentinelError(err, apperror.ErrProviderTemporary)
}

func isSentinelError(err error, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		err = unwrap(err)
	}
	return false
}

func unwrap(err error) error {
	type unwrapper interface {
		Unwrap() error
	}
	u, ok := err.(unwrapper)
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func TestNewMetaWhatsAppClient_SendsCorrectRequest(t *testing.T) {
	var capturedPath, capturedAuth string
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"messaging_product":"whatsapp","contacts":[{"input":"+15551234567","wa_id":"15551234567"}],"messages":[{"id":"wamid.123"}]}`)
	}))
	defer srv.Close()

	client := provider.NewMetaWhatsAppClient(config.WhatsAppConfig{
		BaseURL:       srv.URL,
		APIVersion:    "v21.0",
		PhoneNumberID: "PHONE_ID_123",
		AccessToken:   "secret-token",
		Timeout:       0,
	})

	_, err := client.SendMessage(context.Background(), provider.SendMessageRequest{
		PhoneNumber: "+15551234567",
		Body:        "Test message body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedPath != "/v21.0/PHONE_ID_123/messages" {
		t.Fatalf("path = %s, want /v21.0/PHONE_ID_123/messages", capturedPath)
	}
	if capturedAuth != "Bearer secret-token" {
		t.Fatalf("auth = %s, want Bearer secret-token", capturedAuth)
	}
	if capturedBody["to"] != "+15551234567" {
		t.Fatalf("to = %v, want +15551234567", capturedBody["to"])
	}
	text, ok := capturedBody["text"].(map[string]any)
	if !ok {
		t.Fatal("missing text field in request body")
	}
	if text["body"] != "Test message body" {
		t.Fatalf("text.body = %v, want Test message body", text["body"])
	}
}
