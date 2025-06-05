package logger

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type GormLogger struct {
	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
	Debug                 bool
}

func NewGormLogger(debug bool) *GormLogger {
	return &GormLogger{
		Debug: debug,
	}
}

func (l *GormLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return l
}

func (l *GormLogger) Info(ctx context.Context, s string, args ...interface{}) {
	Logger.Info(s, args...)
}

func (l *GormLogger) Warn(ctx context.Context, s string, args ...interface{}) {
	Logger.Warn(s, args...)
}

func (l *GormLogger) Error(ctx context.Context, s string, args ...interface{}) {
	Logger.Error(s, args...)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, _ := fc()

	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && l.SkipErrRecordNotFound) {
		Logger.Errorf("%s [%s]", sql, elapsed)
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold {
		Logger.Warnf("%s [%s]", sql, elapsed)
		return
	}

	if l.Debug {
		Logger.Warnf("%s [%s]", sql, elapsed)
	}
}
