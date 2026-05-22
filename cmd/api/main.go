package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"social-notif/internal/api"
	"social-notif/internal/config"
	"social-notif/internal/repository"
	"social-notif/internal/worker"

	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger, err := config.NewLogger(cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	db, err := repository.NewPostgres(ctx, cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to initialize database", zap.Error(err))
	}

	redisClient, err := worker.NewRedisClient(ctx, cfg.Redis, logger)
	if err != nil {
		logger.Fatal("failed to initialize redis", zap.Error(err))
	}

	router := api.NewRouter(api.Dependencies{
		Config: cfg,
		Logger: logger,
		DB:     db,
		Redis:  redisClient,
	})

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	go func() {
		logger.Info("api server started", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("api server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.App.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server shutdown failed", zap.Error(err))
	}

	if err := repository.ClosePostgres(db, logger); err != nil {
		logger.Error("database close failed", zap.Error(err))
	}

	if err := redisClient.Close(); err != nil {
		logger.Error("redis close failed", zap.Error(err))
	}

	time.Sleep(100 * time.Millisecond)
	logger.Info("api server stopped")
}
