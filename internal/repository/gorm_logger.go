package repository

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/logger"
)

type GormLogger struct {
	logger             *zap.Logger
	level              logger.LogLevel
	slowQueryThreshold time.Duration
}

func NewGormLogger(log *zap.Logger, slowQueryThreshold time.Duration) *GormLogger {
	return &GormLogger{
		logger:             log.Named("gorm"),
		level:              logger.Warn,
		slowQueryThreshold: slowQueryThreshold,
	}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	copy := *l
	copy.level = level
	return &copy
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.level < logger.Info {
		return
	}
	l.logger.Info(msg, zap.Any("data", data))
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.level < logger.Warn {
		return
	}
	l.logger.Warn(msg, zap.Any("data", data))
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.level < logger.Error {
		return
	}
	l.logger.Error(msg, zap.Any("data", data))
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	fields := []zap.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && l.level >= logger.Error && !errors.Is(err, logger.ErrRecordNotFound):
		l.logger.Error("gorm query failed", append(fields, zap.Error(err))...)
	case elapsed > l.slowQueryThreshold && l.level >= logger.Warn:
		l.logger.Warn("gorm slow query", append(fields, zap.Duration("threshold", l.slowQueryThreshold))...)
	case l.level >= logger.Info:
		l.logger.Info("gorm query completed", fields...)
	}
}
