package repository

import (
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
	GetAllCronSchedules() ([]modelsdb.CronSchedule, error)
	GetCronScheduleByKey(jobKey string) (*modelsdb.CronSchedule, error)
	UpsertCronSchedule(s *modelsdb.CronSchedule) error
}

// TrainingLogStore - interface cho training log operations
type TrainingLogStore interface {
	CreateTrainingLog(log *modelsdb.TrainingLog) error
	GetTrainingLogByID(id uint) (*modelsdb.TrainingLog, error)
	GetTrainingLogsBySessionID(sessionID string) ([]modelsdb.TrainingLog, error)
	GetTrainingSessions(limit, offset int) ([]modelsdb.TrainingLog, int64, error)
	// GetLatestTrainingLogByAlgorithm returns the most recent TrainingLog for each algorithm.
	// The returned slice has at most one entry per algorithm_name.
	GetLatestTrainingLogByAlgorithm() ([]modelsdb.TrainingLog, error)
	// GetTrainingMetricsAggregate returns aggregate stats across all training logs.
	GetTrainingMetricsAggregate() (modelsdb.TrainingMetricsAggregate, error)
	// GetTrainingSessionsByMarket returns paginated training logs filtered by market_key.
	GetTrainingSessionsByMarket(marketKey string, page, limit int, algorithm, sortBy, sortDir string) ([]modelsdb.TrainingLog, int64, error)
}

// SyncLog - interface cho sync log operations
type SyncLog interface {
	// Read operations
	GetAllSyncLogs() ([]modelsdb.SyncLog, error)
	GetSyncLogByID(id uint) (*modelsdb.SyncLog, error)
	GetLatestSyncLogs(limit int) ([]modelsdb.SyncLog, error)
	GetSyncLogsBySource(source string) ([]modelsdb.SyncLog, error)
	GetSyncLogsByDateRange(fromDate, toDate time.Time) ([]modelsdb.SyncLog, error)

	// Write operations
	SaveSyncLog(syncLog *modelsdb.SyncLog) error
	CreateSyncLog(syncLog *modelsdb.SyncLog) error
	UpdateSyncLog(syncLog *modelsdb.SyncLog) error
	DeleteSyncLog(id uint) error
}

// Utility - interface cho utility operations
type Utility interface {
	TruncateSyncLogs() error
}
