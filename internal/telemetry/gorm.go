package telemetry

import (
	"context"
	"time"

	"gorm.io/gorm/logger"
)

type GORMLogger struct {
	delegate logger.Interface
	metrics  *Metrics
}

func NewGORMLogger(delegate logger.Interface, metrics *Metrics) logger.Interface {
	return &GORMLogger{delegate: delegate, metrics: metrics}
}

func (l *GORMLogger) LogMode(level logger.LogLevel) logger.Interface {
	return &GORMLogger{delegate: l.delegate.LogMode(level), metrics: l.metrics}
}

func (l *GORMLogger) Info(ctx context.Context, message string, data ...any) {
	l.delegate.Info(ctx, message, data...)
}

func (l *GORMLogger) Warn(ctx context.Context, message string, data ...any) {
	l.delegate.Warn(ctx, message, data...)
}

func (l *GORMLogger) Error(ctx context.Context, message string, data ...any) {
	l.delegate.Error(ctx, message, data...)
}

func (l *GORMLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.delegate.Trace(ctx, begin, fc, err)
	l.metrics.ObserveDBQuery(time.Since(begin), err)
}
