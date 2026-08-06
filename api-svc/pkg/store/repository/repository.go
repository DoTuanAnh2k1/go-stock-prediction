package repository

import (
	"context"
	"go-stock-prediction/pkg/models/models_config"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"
)

// DatabaseStore - interface chính cho tất cả DB operations
type DatabaseStore interface {
	Init(cfg models_config.DatabaseConfig) error
	Ping() error

	// Embedded interfaces
	SyncLog
	Utility
	GoldPriceStore
	GoldPredictionStore
	GoldIntradayStore
	MacroIndicatorStore
	TrainingLogStore
	CronScheduleStore
	NasdaqPriceStore
	NasdaqPredictionStore
	NasdaqIntradayStore
	CryptoPriceStore
	CryptoPredictionStore
	CryptoIntradayStore
	SP500PriceStore
	SP500PredictionStore
	SP500IntradayStore
	SimulationStore
	DirectionAccuracyStore
	MonitoringStore
	SessionStatsStore
	PipelineReportStore
}

// CronScheduleStore - interface cho cron schedule operations
type CronScheduleStore interface {
	GetAllCronSchedules(ctx context.Context) ([]modelsdb.CronSchedule, error)
	GetCronScheduleByKey(ctx context.Context, jobKey string) (*modelsdb.CronSchedule, error)
	UpsertCronSchedule(ctx context.Context, s *modelsdb.CronSchedule) error
}

// TrainingLogStore - interface cho training log operations
type TrainingLogStore interface {
	CreateTrainingLog(ctx context.Context, log *modelsdb.TrainingLog) error
	GetTrainingLogByID(ctx context.Context, id uint) (*modelsdb.TrainingLog, error)
	GetTrainingLogsBySessionID(ctx context.Context, sessionID string) ([]modelsdb.TrainingLog, error)
	GetTrainingSessions(ctx context.Context, limit, offset int) ([]modelsdb.TrainingLog, int64, error)
	// GetLatestTrainingLogByAlgorithm returns the most recent TrainingLog for each algorithm.
	// The returned slice has at most one entry per algorithm_name.
	GetLatestTrainingLogByAlgorithm(ctx context.Context) ([]modelsdb.TrainingLog, error)
	// GetTrainingMetricsAggregate returns aggregate stats across all training logs.
	GetTrainingMetricsAggregate(ctx context.Context) (modelsdb.TrainingMetricsAggregate, error)
	// GetTrainingSessionsByMarket returns paginated training logs filtered by market_key.
	GetTrainingSessionsByMarket(ctx context.Context, marketKey string, page, limit int, algorithm, sortBy, sortDir string) ([]modelsdb.TrainingLog, int64, error)
}

// SyncLog - interface cho sync log operations
type SyncLog interface {
	// Read operations
	GetAllSyncLogs(ctx context.Context) ([]modelsdb.SyncLog, error)
	GetSyncLogByID(ctx context.Context, id uint) (*modelsdb.SyncLog, error)
	GetLatestSyncLogs(ctx context.Context, limit int) ([]modelsdb.SyncLog, error)
	GetSyncLogsBySource(ctx context.Context, source string) ([]modelsdb.SyncLog, error)
	GetSyncLogsByDateRange(ctx context.Context, fromDate, toDate time.Time) ([]modelsdb.SyncLog, error)

	// Write operations
	SaveSyncLog(ctx context.Context, syncLog *modelsdb.SyncLog) error
	CreateSyncLog(ctx context.Context, syncLog *modelsdb.SyncLog) error
	UpdateSyncLog(ctx context.Context, syncLog *modelsdb.SyncLog) error
	DeleteSyncLog(ctx context.Context, id uint) error
}

// Utility - interface cho utility operations
type Utility interface {
	TruncateSyncLogs(ctx context.Context) error
}
