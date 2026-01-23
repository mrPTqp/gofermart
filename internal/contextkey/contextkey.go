package contextkey

import (
	"context"
	"go.uber.org/zap"
)

type key string

const (
	LoggerKey key = "logger"
	UserIDKey key = "user_id"
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

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	uid, ok := ctx.Value(UserIDKey).(int64)
	return uid, ok
}
