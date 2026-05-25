package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"social-notif/internal/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogger_LogsStructuredRequestFields(t *testing.T) {
	router, logs := newLoggerRouter()
	router.POST("/ok", func(c *gin.Context) {
		c.String(http.StatusAccepted, "accepted")
	})

	req := httptest.NewRequest(http.MethodPost, "/ok", nil)
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("log count = %d, want 1", len(entries))
	}

	fields := entries[0].ContextMap()
	assertLogField(t, fields, "request_id", "req-123")
	assertLogField(t, fields, "status_code", int64(http.StatusAccepted))
	assertLogField(t, fields, "method", http.MethodPost)
	assertLogField(t, fields, "path", "/ok")
	assertLogField(t, fields, "client_ip", "203.0.113.10")

	if _, ok := fields["latency"]; !ok {
		t.Fatalf("latency field missing")
	}
}

func TestLogger_LogsGinErrorsAtErrorLevel(t *testing.T) {
	router, logs := newLoggerRouter()
	router.GET("/error", func(c *gin.Context) {
		_ = c.Error(errors.New("handler failed"))
		c.Status(http.StatusBadRequest)
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	req.Header.Set("X-Request-ID", "req-456")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	entries := logs.FilterMessage("http request completed with errors").All()
	if len(entries) != 1 {
		t.Fatalf("error log count = %d, want 1", len(entries))
	}
	if entries[0].Level.String() != "error" {
		t.Fatalf("log level = %s, want error", entries[0].Level.String())
	}
	if _, ok := entries[0].ContextMap()["errors"]; !ok {
		t.Fatalf("errors field missing")
	}
}

func TestLogger_LogsServerErrorsAtErrorLevel(t *testing.T) {
	router, logs := newLoggerRouter()
	router.GET("/server-error", func(c *gin.Context) {
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/server-error", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	entries := logs.FilterMessage("http request completed with server error").All()
	if len(entries) != 1 {
		t.Fatalf("server error log count = %d, want 1", len(entries))
	}
	if entries[0].Level.String() != "error" {
		t.Fatalf("log level = %s, want error", entries[0].Level.String())
	}
}

func newLoggerRouter() (*gin.Engine, *observer.ObservedLogs) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger(logger))
	return router, logs
}

func assertLogField(t *testing.T, fields map[string]any, key string, want any) {
	t.Helper()

	got, ok := fields[key]
	if !ok {
		t.Fatalf("%s field missing", key)
	}
	if got != want {
		t.Fatalf("%s field = %v, want %v", key, got, want)
	}
}
