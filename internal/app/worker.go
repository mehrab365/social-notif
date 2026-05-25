package app

import (
	"context"
	"fmt"

	"social-notif/internal/config"
	"social-notif/internal/provider"
	"social-notif/internal/repository"
	"social-notif/internal/worker"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func RunWorker(ctx context.Context) error {
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
		logger.Fatal("failed to initialize database", zap.Error(err))
	}

	msgRepo := repository.NewMessageRepository(db)
	whatsAppClient := provider.NewMetaWhatsAppClient(cfg.WhatsApp)

	redisOpt := worker.AsynqRedisOptions(cfg.Redis)
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.Queue.Concurrency,
		Queues: map[string]int{
			"default": cfg.Queue.DefaultPriority,
		},
		ErrorHandler: worker.NewErrorHandler(logger),
	})

	mux := asynq.NewServeMux()
	worker.RegisterHandlers(mux, worker.Dependencies{
		Config:      cfg,
		Logger:      logger,
		DB:          db,
		MessageRepo: msgRepo,
		Provider:    whatsAppClient,
	})

	go func() {
		logger.Info("worker started", zap.Int("concurrency", cfg.Queue.Concurrency))
		if err := server.Run(mux); err != nil {
			logger.Fatal("worker failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("worker shutdown signal received")
	server.Shutdown()

	if err := repository.ClosePostgres(db, logger); err != nil {
		logger.Error("database close failed", zap.Error(err))
	}

	logger.Info("worker stopped")
	return nil
}
