package worker

import (
	"context"
	"fmt"

	"social-notif/internal/config"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewRedisClient(ctx context.Context, cfg config.RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	logger.Info("redis initialized", zap.String("addr", cfg.Addr), zap.Int("db", cfg.DB))
	return client, nil
}

func AsynqRedisOptions(cfg config.RedisConfig) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
}
