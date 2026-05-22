package repository

import (
	"context"
	"fmt"
	"time"

	"social-notif/internal/config"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewPostgres(ctx context.Context, cfg config.DatabaseConfig, log *zap.Logger) (*gorm.DB, error) {
	attempts := cfg.ConnectMaxRetries + 1
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		db, err := openPostgres(ctx, cfg, log)
		if err == nil {
			log.Info("postgres initialized",
				zap.Int("max_open_conns", cfg.MaxOpenConns),
				zap.Int("max_idle_conns", cfg.MaxIdleConns),
				zap.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
				zap.Duration("conn_max_idle_time", cfg.ConnMaxIdleTime),
			)
			return db, nil
		}

		lastErr = err
		if attempt == attempts {
			break
		}

		log.Warn("postgres connection attempt failed",
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", attempts),
			zap.Duration("retry_backoff", cfg.ConnectRetryBackoff),
			zap.Error(err),
		)

		timer := time.NewTimer(cfg.ConnectRetryBackoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("postgres initialization canceled: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf("initialize postgres after %d attempt(s): %w", attempts, lastErr)
}

func openPostgres(ctx context.Context, cfg config.DatabaseConfig, log *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: NewGormLogger(log, cfg.SlowQueryThreshold),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

func CheckPostgres(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("postgres db is nil")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get postgres sql db: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	return nil
}

func ClosePostgres(db *gorm.DB, log *zap.Logger) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get postgres sql db: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close postgres: %w", err)
	}

	log.Info("postgres connection pool closed")
	return nil
}
