package app

import (
	"context"
	"fmt"

	"social-notif/internal/config"
	"social-notif/internal/migration"
	"social-notif/internal/repository"

	"go.uber.org/zap"
)

func RunMigrate(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := config.NewLogger(cfg)
	if err != nil {
		return fmt.Errorf("new logger: %w", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	db, err := repository.NewPostgres(ctx, cfg.Database, logger)
	if err != nil {
		return fmt.Errorf("initialize postgres: %w", err)
	}
	defer func() {
		if err := repository.ClosePostgres(db, logger); err != nil {
			logger.Error("database close failed", zap.Error(err))
		}
	}()

	if err := migration.Run(ctx, db); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("database migration completed")
	return nil
}
