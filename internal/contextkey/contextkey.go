package contextkey

import (
	"context"

	"go.uber.org/zap"
)

type key string

const (
	LoggerKey key = "logger"
)

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

func LoggerFromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(LoggerKey).(*zap.Logger); ok {
		return logger
	}
	return zap.L()
}
