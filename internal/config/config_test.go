package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFromPathLoadsDotEnv(t *testing.T) {
	unsetEnv(t,
		"APP_ENV",
		"DATABASE_URL",
		"HTTP_PORT",
		"HTTP_TRUSTED_PROXIES",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"REDIS_DB",
		"LOG_LEVEL",
	)

	path := writeDotEnv(t, `
APP_ENV=local
DATABASE_URL=postgres://user:pass@localhost:5432/social_notif?sslmode=disable
HTTP_PORT=9090
HTTP_TRUSTED_PROXIES=10.0.0.1, 10.0.0.2
DB_MAX_OPEN_CONNS=12
DB_MAX_IDLE_CONNS=6
REDIS_DB=2
LOG_LEVEL=debug
`)

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	if cfg.HTTP.Port != "9090" {
		t.Fatalf("HTTP.Port = %q, want 9090", cfg.HTTP.Port)
	}
	if cfg.Database.MaxOpenConns != 12 {
		t.Fatalf("Database.MaxOpenConns = %d, want 12", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns != 6 {
		t.Fatalf("Database.MaxIdleConns = %d, want 6", cfg.Database.MaxIdleConns)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("Redis.DB = %d, want 2", cfg.Redis.DB)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if len(cfg.HTTP.TrustedProxies) != 2 {
		t.Fatalf("HTTP.TrustedProxies = %v, want 2 entries", cfg.HTTP.TrustedProxies)
	}
}

func TestLoadFromPathProcessEnvOverridesDotEnv(t *testing.T) {
	unsetEnv(t, "APP_ENV", "DATABASE_URL", "HTTP_PORT")
	t.Setenv("HTTP_PORT", "7070")

	path := writeDotEnv(t, `
APP_ENV=local
DATABASE_URL=postgres://user:pass@localhost:5432/social_notif?sslmode=disable
HTTP_PORT=9090
`)

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	if cfg.HTTP.Port != "7070" {
		t.Fatalf("HTTP.Port = %q, want process env override 7070", cfg.HTTP.Port)
	}
}

func TestLoadFromPathReturnsParseErrors(t *testing.T) {
	unsetEnv(t, "APP_ENV", "DATABASE_URL", "HTTP_READ_TIMEOUT", "REDIS_DB")

	path := writeDotEnv(t, `
APP_ENV=local
DATABASE_URL=postgres://user:pass@localhost:5432/social_notif?sslmode=disable
HTTP_READ_TIMEOUT=soon
REDIS_DB=cache
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("LoadFromPath() error = nil, want parse error")
	}

	for _, want := range []string{"HTTP_READ_TIMEOUT must be a valid duration", "REDIS_DB must be an integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadFromPath() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateRequiresProductionSecrets(t *testing.T) {
	cfg := validConfig()
	cfg.App.Env = "production"
	cfg.Security.APIKey = ""
	cfg.WhatsApp.AccessToken = ""
	cfg.WhatsApp.PhoneNumberID = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing secret errors")
	}

	for _, want := range []string{
		"API_KEY is required outside local environment",
		"WHATSAPP_ACCESS_TOKEN is required outside local environment",
		"WHATSAPP_PHONE_NUMBER_ID is required outside local environment",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateRejectsInvalidSectionValues(t *testing.T) {
	cfg := validConfig()
	cfg.HTTP.Port = "99999"
	cfg.Database.MaxIdleConns = cfg.Database.MaxOpenConns + 1
	cfg.Logging.Level = "verbose"
	cfg.WhatsApp.BaseURL = "://bad-url"

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}

	for _, want := range []string{
		"HTTP_PORT must be a valid TCP port",
		"DB_MAX_IDLE_CONNS must be less than or equal to DB_MAX_OPEN_CONNS",
		"LOG_LEVEL must be one of",
		"WHATSAPP_BASE_URL must be a valid absolute URL",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestLoadFromPathRejectsMalformedDotEnv(t *testing.T) {
	path := writeDotEnv(t, "DATABASE_URL\n")

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("LoadFromPath() error = nil, want dotenv parse error")
	}
	if !strings.Contains(err.Error(), "expected KEY=VALUE") {
		t.Fatalf("LoadFromPath() error = %q, want dotenv parse error", err.Error())
	}
}

func validConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:            "social-notif",
			Env:             "local",
			ShutdownTimeout: 15 * time.Second,
		},
		HTTP: HTTPConfig{
			Port:              "8080",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       time.Minute,
			RequestTimeout:    10 * time.Second,
		},
		Database: DatabaseConfig{
			DSN:             "postgres://user:pass@localhost:5432/social_notif?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
			DB:   0,
		},
		Queue: QueueConfig{
			Concurrency:     10,
			DefaultPriority: 1,
		},
		Security: SecurityConfig{
			APIKey:          "test-key",
			RateLimitPerMin: 120,
			MaxRequestBytes: 1048576,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		WhatsApp: WhatsAppConfig{
			AccessToken:       "test-token",
			PhoneNumberID:     "test-phone-id",
			BusinessAccountID: "test-business-id",
			APIVersion:        "v20.0",
			BaseURL:           "https://graph.facebook.com",
			Timeout:           10 * time.Second,
		},
	}
}

func writeDotEnv(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}
	return path
}

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		previous, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}

		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, previous)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}
