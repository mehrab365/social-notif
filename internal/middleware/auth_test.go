package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"social-notif/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAPIKeyAuth_AllowsValidXAPIKey(t *testing.T) {
	router, logs := newAuthRouter("secret")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(middleware.APIKeyHeader, "secret")
	req.Header.Set("X-Request-ID", "req-123")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if logs.FilterMessage("api key authentication failed").Len() != 0 {
		t.Fatalf("unexpected auth failure log")
	}
}

func TestAPIKeyAuth_Returns401ForInvalidKey(t *testing.T) {
	router, logs := newAuthRouter("secret")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(middleware.APIKeyHeader, "wrong")
	req.Header.Set("X-Request-ID", "req-456")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	entries := logs.FilterMessage("api key authentication failed").All()
	if len(entries) != 1 {
		t.Fatalf("auth failure log count = %d, want 1", len(entries))
	}
	if got := entries[0].ContextMap()["request_id"]; got != "req-456" {
		t.Fatalf("request_id log field = %v, want req-456", got)
	}
}

func TestAPIKeyAuth_Returns401ForMissingKey(t *testing.T) {
	router, _ := newAuthRouter("secret")

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func newAuthRouter(expectedKey string) (*gin.Engine, *observer.ObservedLogs) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.APIKeyAuth(expectedKey, logger))
	router.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	return router, logs
}
