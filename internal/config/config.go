package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Queue    QueueConfig
	Security SecurityConfig
	Logging  LoggingConfig
	WhatsApp WhatsAppConfig
}

type AppConfig struct {
	Name            string
	Env             string
	ShutdownTimeout time.Duration
}

type HTTPConfig struct {
	Port              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	RequestTimeout    time.Duration
	TrustedProxies    []string
}

type DatabaseConfig struct {
	DSN                 string
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetime     time.Duration
	ConnMaxIdleTime     time.Duration
	ConnectTimeout      time.Duration
	ConnectMaxRetries   int
	ConnectRetryBackoff time.Duration
	HealthCheckTimeout  time.Duration
	SlowQueryThreshold  time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type QueueConfig struct {
	Concurrency     int
	DefaultPriority int
}

type SecurityConfig struct {
	APIKey          string
	RateLimitPerMin int
	MaxRequestBytes int64
}

type LoggingConfig struct {
	Level string
}

type WhatsAppConfig struct {
	AccessToken       string
	PhoneNumberID     string
	BusinessAccountID string
	APIVersion        string
	BaseURL           string
	Timeout           time.Duration
}

func Load() (*Config, error) {
	return LoadFromPath(".env")
}

func LoadFromPath(path string) (*Config, error) {
	fileValues, err := readDotEnv(path)
	if err != nil {
		return nil, err
	}

	env := envReader{fileValues: fileValues}
	return loadFromEnv(env)
}

type envReader struct {
	fileValues map[string]string
}

func (e envReader) get(key string) (string, bool) {
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value), true
	}

	value, ok := e.fileValues[key]
	return strings.TrimSpace(value), ok
}

func loadFromEnv(env envReader) (*Config, error) {
	cfg := &Config{
		App: AppConfig{
			Name: getStringEnv(env, "APP_NAME", "social-notif"),
			Env:  getStringEnv(env, "APP_ENV", "local"),
		},
		HTTP: HTTPConfig{
			Port:           getStringEnv(env, "HTTP_PORT", "8080"),
			TrustedProxies: getCSVEnv(env, "HTTP_TRUSTED_PROXIES"),
		},
		Database: DatabaseConfig{
			DSN: getStringEnv(env, "DATABASE_URL", ""),
		},
		Redis: RedisConfig{
			Addr:     getStringEnv(env, "REDIS_ADDR", "localhost:6379"),
			Password: getStringEnv(env, "REDIS_PASSWORD", ""),
		},
		Security: SecurityConfig{
			APIKey: getStringEnv(env, "API_KEY", ""),
		},
		Logging: LoggingConfig{
			Level: getStringEnv(env, "LOG_LEVEL", "info"),
		},
		WhatsApp: WhatsAppConfig{
			AccessToken:       getStringEnv(env, "WHATSAPP_ACCESS_TOKEN", ""),
			PhoneNumberID:     getStringEnv(env, "WHATSAPP_PHONE_NUMBER_ID", ""),
			BusinessAccountID: getStringEnv(env, "WHATSAPP_BUSINESS_ACCOUNT_ID", ""),
			APIVersion:        getStringEnv(env, "WHATSAPP_API_VERSION", "v20.0"),
			BaseURL:           getStringEnv(env, "WHATSAPP_BASE_URL", "https://graph.facebook.com"),
		},
	}

	var parseErrs validationErrors
	cfg.App.ShutdownTimeout, parseErrs = appendDurationEnv(parseErrs, env, "APP_SHUTDOWN_TIMEOUT", 15*time.Second)
	cfg.HTTP.ReadHeaderTimeout, parseErrs = appendDurationEnv(parseErrs, env, "HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	cfg.HTTP.ReadTimeout, parseErrs = appendDurationEnv(parseErrs, env, "HTTP_READ_TIMEOUT", 15*time.Second)
	cfg.HTTP.WriteTimeout, parseErrs = appendDurationEnv(parseErrs, env, "HTTP_WRITE_TIMEOUT", 15*time.Second)
	cfg.HTTP.IdleTimeout, parseErrs = appendDurationEnv(parseErrs, env, "HTTP_IDLE_TIMEOUT", 60*time.Second)
	cfg.HTTP.RequestTimeout, parseErrs = appendDurationEnv(parseErrs, env, "HTTP_REQUEST_TIMEOUT", 10*time.Second)
	cfg.Database.MaxOpenConns, parseErrs = appendIntEnv(parseErrs, env, "DB_MAX_OPEN_CONNS", 25)
	cfg.Database.MaxIdleConns, parseErrs = appendIntEnv(parseErrs, env, "DB_MAX_IDLE_CONNS", 10)
	cfg.Database.ConnMaxLifetime, parseErrs = appendDurationEnv(parseErrs, env, "DB_CONN_MAX_LIFETIME", 30*time.Minute)
	cfg.Database.ConnMaxIdleTime, parseErrs = appendDurationEnv(parseErrs, env, "DB_CONN_MAX_IDLE_TIME", 10*time.Minute)
	cfg.Database.ConnectTimeout, parseErrs = appendDurationEnv(parseErrs, env, "DB_CONNECT_TIMEOUT", 5*time.Second)
	cfg.Database.ConnectMaxRetries, parseErrs = appendIntEnv(parseErrs, env, "DB_CONNECT_MAX_RETRIES", 5)
	cfg.Database.ConnectRetryBackoff, parseErrs = appendDurationEnv(parseErrs, env, "DB_CONNECT_RETRY_BACKOFF", 2*time.Second)
	cfg.Database.HealthCheckTimeout, parseErrs = appendDurationEnv(parseErrs, env, "DB_HEALTH_CHECK_TIMEOUT", 2*time.Second)
	cfg.Database.SlowQueryThreshold, parseErrs = appendDurationEnv(parseErrs, env, "DB_SLOW_QUERY_THRESHOLD", 500*time.Millisecond)
	cfg.Redis.DB, parseErrs = appendIntEnv(parseErrs, env, "REDIS_DB", 0)
	cfg.Queue.Concurrency, parseErrs = appendIntEnv(parseErrs, env, "QUEUE_CONCURRENCY", 10)
	cfg.Queue.DefaultPriority, parseErrs = appendIntEnv(parseErrs, env, "QUEUE_DEFAULT_PRIORITY", 1)
	cfg.Security.RateLimitPerMin, parseErrs = appendIntEnv(parseErrs, env, "RATE_LIMIT_PER_MIN", 120)
	maxRequestBytes, nextErrs := appendIntEnv(parseErrs, env, "MAX_REQUEST_BYTES", 1048576)
	parseErrs = nextErrs
	cfg.Security.MaxRequestBytes = int64(maxRequestBytes)
	cfg.WhatsApp.Timeout, parseErrs = appendDurationEnv(parseErrs, env, "WHATSAPP_TIMEOUT", 10*time.Second)
	if len(parseErrs) > 0 {
		return nil, parseErrs
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs validationErrors

	if c.Database.DSN == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	if c.App.Name == "" {
		errs = append(errs, "APP_NAME is required")
	}
	if c.App.Env == "" {
		errs = append(errs, "APP_ENV is required")
	}

	if c.App.ShutdownTimeout <= 0 {
		errs = append(errs, "APP_SHUTDOWN_TIMEOUT must be greater than zero")
	}

	port, err := strconv.Atoi(c.HTTP.Port)
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, "HTTP_PORT must be a valid TCP port")
	}
	if c.HTTP.ReadHeaderTimeout <= 0 {
		errs = append(errs, "HTTP_READ_HEADER_TIMEOUT must be greater than zero")
	}
	if c.HTTP.ReadTimeout <= 0 {
		errs = append(errs, "HTTP_READ_TIMEOUT must be greater than zero")
	}
	if c.HTTP.WriteTimeout <= 0 {
		errs = append(errs, "HTTP_WRITE_TIMEOUT must be greater than zero")
	}
	if c.HTTP.IdleTimeout <= 0 {
		errs = append(errs, "HTTP_IDLE_TIMEOUT must be greater than zero")
	}
	if c.HTTP.RequestTimeout <= 0 {
		errs = append(errs, "HTTP_REQUEST_TIMEOUT must be greater than zero")
	}

	if c.Database.MaxOpenConns <= 0 {
		errs = append(errs, "DB_MAX_OPEN_CONNS must be greater than zero")
	}
	if c.Database.MaxIdleConns < 0 {
		errs = append(errs, "DB_MAX_IDLE_CONNS must be zero or greater")
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, "DB_MAX_IDLE_CONNS must be less than or equal to DB_MAX_OPEN_CONNS")
	}
	if c.Database.ConnMaxLifetime <= 0 {
		errs = append(errs, "DB_CONN_MAX_LIFETIME must be greater than zero")
	}
	if c.Database.ConnMaxIdleTime <= 0 {
		errs = append(errs, "DB_CONN_MAX_IDLE_TIME must be greater than zero")
	}
	if c.Database.ConnectTimeout <= 0 {
		errs = append(errs, "DB_CONNECT_TIMEOUT must be greater than zero")
	}
	if c.Database.ConnectMaxRetries < 0 {
		errs = append(errs, "DB_CONNECT_MAX_RETRIES must be zero or greater")
	}
	if c.Database.ConnectRetryBackoff <= 0 {
		errs = append(errs, "DB_CONNECT_RETRY_BACKOFF must be greater than zero")
	}
	if c.Database.HealthCheckTimeout <= 0 {
		errs = append(errs, "DB_HEALTH_CHECK_TIMEOUT must be greater than zero")
	}
	if c.Database.SlowQueryThreshold <= 0 {
		errs = append(errs, "DB_SLOW_QUERY_THRESHOLD must be greater than zero")
	}

	if c.Redis.Addr == "" {
		errs = append(errs, "REDIS_ADDR is required")
	}
	if c.Redis.DB < 0 {
		errs = append(errs, "REDIS_DB must be zero or greater")
	}

	if c.Queue.Concurrency <= 0 {
		errs = append(errs, "QUEUE_CONCURRENCY must be greater than zero")
	}
	if c.Queue.DefaultPriority <= 0 {
		errs = append(errs, "QUEUE_DEFAULT_PRIORITY must be greater than zero")
	}

	if c.Security.APIKey == "" && c.App.Env != "local" {
		errs = append(errs, "API_KEY is required outside local environment")
	}
	if c.Security.RateLimitPerMin < 0 {
		errs = append(errs, "RATE_LIMIT_PER_MIN must be zero or greater")
	}
	if c.Security.MaxRequestBytes <= 0 {
		errs = append(errs, "MAX_REQUEST_BYTES must be greater than zero")
	}

	if !isValidLogLevel(c.Logging.Level) {
		errs = append(errs, "LOG_LEVEL must be one of debug, info, warn, error, dpanic, panic, fatal")
	}

	if c.WhatsApp.APIVersion == "" {
		errs = append(errs, "WHATSAPP_API_VERSION is required")
	}
	if c.WhatsApp.BaseURL == "" {
		errs = append(errs, "WHATSAPP_BASE_URL is required")
	} else if parsed, err := url.ParseRequestURI(c.WhatsApp.BaseURL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		errs = append(errs, "WHATSAPP_BASE_URL must be a valid absolute URL")
	}
	if c.WhatsApp.Timeout <= 0 {
		errs = append(errs, "WHATSAPP_TIMEOUT must be greater than zero")
	}
	if c.App.Env != "local" {
		if c.WhatsApp.AccessToken == "" {
			errs = append(errs, "WHATSAPP_ACCESS_TOKEN is required outside local environment")
		}
		if c.WhatsApp.PhoneNumberID == "" {
			errs = append(errs, "WHATSAPP_PHONE_NUMBER_ID is required outside local environment")
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

type validationErrors []string

func (e validationErrors) Error() string {
	return "invalid config: " + strings.Join(e, "; ")
}

func getStringEnv(env envReader, key, fallback string) string {
	value, ok := env.get(key)
	if !ok {
		return fallback
	}
	if value == "" {
		return fallback
	}
	return value
}

func appendIntEnv(errs validationErrors, env envReader, key string, fallback int) (int, validationErrors) {
	value, ok := env.get(key)
	if !ok || value == "" {
		return fallback, errs
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback, append(errs, fmt.Sprintf("%s must be an integer", key))
	}
	return parsed, errs
}

func appendDurationEnv(errs validationErrors, env envReader, key string, fallback time.Duration) (time.Duration, validationErrors) {
	value, ok := env.get(key)
	if !ok || value == "" {
		return fallback, errs
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback, append(errs, fmt.Sprintf("%s must be a valid duration", key))
	}
	return parsed, errs
}

func getCSVEnv(env envReader, key string) []string {
	value, ok := env.get(key)
	if !ok {
		return nil
	}
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func isValidLogLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error", "dpanic", "panic", "fatal":
		return true
	default:
		return false
	}
}

func readDotEnv(path string) (map[string]string, error) {
	values := make(map[string]string)
	if path == "" {
		return values, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return values, nil
		}
		return nil, fmt.Errorf("read dotenv file %s: %w", path, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("parse dotenv file %s:%d: expected KEY=VALUE", path, lineNo)
		}

		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("parse dotenv file %s:%d: invalid key", path, lineNo)
		}

		parsedValue, err := parseDotEnvValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("parse dotenv file %s:%d: %w", path, lineNo, err)
		}
		values[key] = parsedValue
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan dotenv file %s: %w", path, err)
	}

	return values, nil
}

func parseDotEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	if strings.HasPrefix(value, `"`) {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", err
		}
		return parsed, nil
	}

	if strings.HasPrefix(value, "'") {
		if !strings.HasSuffix(value, "'") || len(value) == 1 {
			return "", errors.New("unterminated quoted value")
		}
		return strings.TrimSuffix(strings.TrimPrefix(value, "'"), "'"), nil
	}

	if before, _, ok := strings.Cut(value, " #"); ok {
		value = before
	}

	return strings.TrimSpace(value), nil
}
