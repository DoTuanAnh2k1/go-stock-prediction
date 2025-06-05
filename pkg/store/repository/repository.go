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
	Exchange
	Stock
	StockPrices
	Prediction
	SyncLog
	Utility
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

	// Write operations
	SavePrediction(prediction *modelsdb.Prediction) error
	CreatePrediction(prediction *modelsdb.Prediction) error
	UpdatePrediction(prediction *modelsdb.Prediction) error
	DeletePrediction(id uint) error
	BulkCreatePredictions(predictions []modelsdb.Prediction) error
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
