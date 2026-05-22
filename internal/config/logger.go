package config

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(cfg *Config) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}

	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(level)
	zapCfg.Encoding = "json"
	zapCfg.InitialFields = map[string]any{
		"service": cfg.App.Name,
		"env":     cfg.App.Env,
	}

	if cfg.App.Env == "local" {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.Level = zap.NewAtomicLevelAt(level)
		zapCfg.Encoding = "json"
	}

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	return logger, nil
}
