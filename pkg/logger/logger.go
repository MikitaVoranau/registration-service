package logger

import (
	"context"
	"fmt"
	"go.uber.org/zap"
)

type Logger struct {
	l *zap.Logger
}

func New(ctx context.Context) (context.Context, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("error creating new logger: %w", err)
	}
	ctx = context.WithValue(ctx, "logger", &Logger{logger})

	return ctx, nil
}

func GetLogger(ctx context.Context) *Logger {
	return ctx.Value("logger").(*Logger)
}

func (logger *Logger) Info(msg string, fields ...zap.Field) {
	logger.l.Info(msg, fields...)
}

func (logger *Logger) Fatal(msg string, fields ...zap.Field) {
	logger.l.Fatal(msg, fields...)
}
