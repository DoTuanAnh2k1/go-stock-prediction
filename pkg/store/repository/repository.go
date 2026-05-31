package repository

import (
	"go-stock-prediction/pkg/models/models_config"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"time"

	"github.com/shopspring/decimal"
)

// DatabaseStore - interface chính cho tất cả DB operations
type DatabaseStore interface {
	Init(cfg models_config.DatabaseConfig) error
	Ping() error

	// Embedded interfaces
	Exchange
	Stock
	StockPrices
	Prediction
	SyncLog
	Utility
	GoldPriceStore
	GoldPredictionStore
	MacroIndicatorStore
	TrainingLogStore
	UserStore
	CronScheduleStore
	NasdaqPriceStore
	NasdaqPredictionStore
	CryptoPriceStore
	CryptoPredictionStore
	FuelPriceStore
	FuelPredictionStore
}

// CronScheduleStore - interface cho cron schedule operations
type CronScheduleStore interface {
	GetAllCronSchedules() ([]modelsdb.CronSchedule, error)
	GetCronScheduleByKey(jobKey string) (*modelsdb.CronSchedule, error)
	UpsertCronSchedule(s *modelsdb.CronSchedule) error
}

// Exchange - interface cho exchange operations
type Exchange interface {
	// Read operations
	GetAllExchanges() ([]modelsdb.Exchange, error)
	GetExchangeByID(id uint) (*modelsdb.Exchange, error)
	GetExchangeByCode(code string) (*modelsdb.Exchange, error)

	// Write operations
	SaveExchange(exchange *modelsdb.Exchange) error
	CreateExchange(exchange *modelsdb.Exchange) error
	UpdateExchange(exchange *modelsdb.Exchange) error
	DeleteExchange(id uint) error
}

// Stock - interface cho stock operations
type Stock interface {
	// Read operations
	GetAllStocks() ([]modelsdb.Stock, error)
	GetStockByID(id uint) (*modelsdb.Stock, error)
	GetStockBySymbol(symbol string) (*modelsdb.Stock, error)
	GetVN30Stocks() ([]modelsdb.Stock, error)
	GetVN100Stocks() ([]modelsdb.Stock, error)
	GetStocksByExchange(exchangeID uint) ([]modelsdb.Stock, error)
	GetStocksBySector(sector string) ([]modelsdb.Stock, error)

	// Write operations
	SaveStock(stock *modelsdb.Stock) error
	CreateStock(stock *modelsdb.Stock) error
	UpdateStock(stock *modelsdb.Stock) error
	DeleteStock(id uint) error
	BulkCreateStocks(stocks []modelsdb.Stock) error
}

// StockPrices - interface cho stock price operations
type StockPrices interface {
	// Read operations
	GetAllStockPrices() ([]modelsdb.StockPrice, error)
	GetStockPriceByID(id uint) (*modelsdb.StockPrice, error)
	GetStockPricesByStockID(stockID uint) ([]modelsdb.StockPrice, error)
	GetStockPricesByStockIDAndDateRange(stockID uint, fromDate, toDate time.Time) ([]modelsdb.StockPrice, error)
	GetLatestStockPriceByStockID(stockID uint) (*modelsdb.StockPrice, error)
	GetStockPriceByStockIDAndDate(stockID uint, tradingDate time.Time) (*modelsdb.StockPrice, error)
	GetLatestStockPricesForVN30() ([]modelsdb.StockPrice, error)

	// Write operations
	SaveStockPrice(stockPrice *modelsdb.StockPrice) error
	CreateStockPrice(stockPrice *modelsdb.StockPrice) error
	UpsertStockPrice(stockPrice *modelsdb.StockPrice) error
	UpdateStockPrice(stockPrice *modelsdb.StockPrice) error
	DeleteStockPrice(id uint) error
	BulkCreateStockPrices(stockPrices []modelsdb.StockPrice) error
	BulkUpsertStockPrices(stockPrices []modelsdb.StockPrice) error
}

// Prediction - interface cho prediction operations
type Prediction interface {
	// Read operations
	GetAllPredictions() ([]modelsdb.Prediction, error)
	GetPredictionByID(id uint) (*modelsdb.Prediction, error)
	GetPredictionsByStockID(stockID uint) ([]modelsdb.Prediction, error)
	GetPredictionsByStockIDAndAlgorithm(stockID uint, algorithmName string) ([]modelsdb.Prediction, error)
	GetLatestPredictionsByStockID(stockID uint, limit int) ([]modelsdb.Prediction, error)
	GetPredictionsByDateRange(fromDate, toDate time.Time) ([]modelsdb.Prediction, error)
	GetPredictionsFiltered(stockID *uint, algorithm string, fromDate, toDate time.Time, offset, limit int) ([]modelsdb.Prediction, int64, error)
	// GetConfirmedPredictionsPage returns paginated predictions that have actual_price set.
	GetConfirmedPredictionsPage(stockID *uint, algorithm string, fromDate, toDate time.Time, offset, limit int) ([]modelsdb.Prediction, int64, error)
	// GetPredictionsWithActual returns predictions that have actual_price set (not null),
	// optionally filtered by stockID. Results are ordered by target_date DESC.
	// days = 0 means no date limit.
	GetPredictionsWithActual(stockID *uint, algorithm string, days int) ([]modelsdb.Prediction, error)
	// GetPendingPredictions returns predictions where actual_price IS NULL and target_date <= cutoff.
	GetPendingPredictions(cutoff time.Time) ([]modelsdb.Prediction, error)
	// UpdatePredictionActual updates actual_price, accuracy, and status for a prediction.
	UpdatePredictionActual(id uint, actualPrice, accuracy *decimal.Decimal, status string) error
	// GetPredictionCountByAlgorithm returns total prediction count per algorithm_name.
	GetPredictionCountByAlgorithm() (map[string]int64, error)
	// GetSuccessfulPredictionCountByAlgorithm returns count of predictions with accuracy >= threshold per algorithm.
	GetSuccessfulPredictionCountByAlgorithm(accuracyThreshold float64) (map[string]int64, error)

	// Write operations
	SavePrediction(prediction *modelsdb.Prediction) error
	CreatePrediction(prediction *modelsdb.Prediction) error
	UpdatePrediction(prediction *modelsdb.Prediction) error
	DeletePrediction(id uint) error
	BulkCreatePredictions(predictions []modelsdb.Prediction) error
	// DeletePredictionsBeforeDate deletes all predictions whose target_date < date.
	// Used by historical backtest to clear stale data before re-inserting.
	DeletePredictionsBeforeDate(date time.Time) error
	// GetPredictionsByMarketPage returns paginated predictions filtered by market.
	// marketKey="vn30" filters stocks with is_vn30=true.
	GetPredictionsByMarketPage(marketKey string, page, limit int, search, sortBy, sortDir, algorithm, status string) ([]modelsdb.Prediction, int64, error)
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
	// Count operations
	CountStocks() (int64, error)
	CountVN30Stocks() (int64, error)
	CountStockPrices() (int64, error)
	CountPredictions() (int64, error)

	// Truncate operations (dùng cẩn thận!)
	TruncateStockPrices() error
	TruncatePredictions() error
	TruncateSyncLogs() error
}
