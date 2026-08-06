package grpcstore

import (
	"context"
	"time"

	modelsdb "go-stock-prediction/pkg/models/models_db"

	"github.com/shopspring/decimal"
)

// These interface methods are write/mutation or read paths that api-svc never
// invokes (they belong to prediction-svc / the crawlers, which own market_db).
// They exist only so *Store satisfies the full DatabaseStore interface. Calling
// any of them is a bug — return errNotImplemented rather than silently no-op.

// TrainingLog
func (s *Store) CreateTrainingLog(context.Context, *modelsdb.TrainingLog) error { return errNotImplemented }

// SyncLog (reads other than GetLatestSyncLogs + all writes)
func (s *Store) GetAllSyncLogs(context.Context) ([]modelsdb.SyncLog, error) {
	return nil, errNotImplemented
}
func (s *Store) GetSyncLogByID(context.Context, uint) (*modelsdb.SyncLog, error) {
	return nil, errNotImplemented
}
func (s *Store) GetSyncLogsBySource(context.Context, string) ([]modelsdb.SyncLog, error) {
	return nil, errNotImplemented
}
func (s *Store) GetSyncLogsByDateRange(context.Context, time.Time, time.Time) ([]modelsdb.SyncLog, error) {
	return nil, errNotImplemented
}
func (s *Store) SaveSyncLog(context.Context, *modelsdb.SyncLog) error   { return errNotImplemented }
func (s *Store) CreateSyncLog(context.Context, *modelsdb.SyncLog) error { return errNotImplemented }
func (s *Store) UpdateSyncLog(context.Context, *modelsdb.SyncLog) error { return errNotImplemented }
func (s *Store) DeleteSyncLog(context.Context, uint) error             { return errNotImplemented }
func (s *Store) TruncateSyncLogs(context.Context) error                { return errNotImplemented }

// Gold writes + uncalled reads
func (s *Store) GetGoldPrices(context.Context, string, string, int) ([]modelsdb.GoldPrice, error) {
	return nil, errNotImplemented
}
func (s *Store) UpsertGoldPrice(context.Context, *modelsdb.GoldPrice) error      { return errNotImplemented }
func (s *Store) BulkUpsertGoldPrices(context.Context, []modelsdb.GoldPrice) error { return errNotImplemented }
func (s *Store) CreateGoldPrediction(context.Context, *modelsdb.GoldPrediction) error {
	return errNotImplemented
}
func (s *Store) GetPendingGoldPredictions(context.Context, time.Time) ([]modelsdb.GoldPrediction, error) {
	return nil, errNotImplemented
}
func (s *Store) UpdateGoldPredictionActual(context.Context, uint, *decimal.Decimal, *decimal.Decimal) error {
	return errNotImplemented
}
func (s *Store) DeleteGoldPredictionsBeforeDate(context.Context, time.Time) error {
	return errNotImplemented
}
func (s *Store) BulkCreateGoldPredictions(context.Context, []modelsdb.GoldPrediction) error {
	return errNotImplemented
}
func (s *Store) UpsertGoldIntradayPrice(context.Context, *modelsdb.GoldIntradayPrice) error {
	return errNotImplemented
}

// Macro indicators
func (s *Store) GetMacroIndicators(context.Context, string, int) ([]modelsdb.MacroIndicator, error) {
	return nil, errNotImplemented
}
func (s *Store) GetLatestMacroIndicator(context.Context, string) (*modelsdb.MacroIndicator, error) {
	return nil, errNotImplemented
}
func (s *Store) UpsertMacroIndicator(context.Context, *modelsdb.MacroIndicator) error {
	return errNotImplemented
}

// NASDAQ writes + uncalled reads
func (s *Store) CreateNasdaqPrice(context.Context, *modelsdb.NasdaqPrice) error { return errNotImplemented }
func (s *Store) UpsertNasdaqPrice(context.Context, *modelsdb.NasdaqPrice) error { return errNotImplemented }
func (s *Store) GetAllNasdaqPricesForSymbol(context.Context, string) ([]modelsdb.NasdaqPrice, error) {
	return nil, errNotImplemented
}
func (s *Store) UpsertNasdaqIntradayPrice(context.Context, *modelsdb.NasdaqIntradayPrice) error {
	return errNotImplemented
}
func (s *Store) CreateNasdaqPrediction(context.Context, *modelsdb.NasdaqPrediction) error {
	return errNotImplemented
}
func (s *Store) BulkCreateNasdaqPredictions(context.Context, []modelsdb.NasdaqPrediction) error {
	return errNotImplemented
}
func (s *Store) DeleteNasdaqPredictionsBeforeDate(context.Context, time.Time) error {
	return errNotImplemented
}

// CRYPTO writes + uncalled reads
func (s *Store) CreateCryptoPrice(context.Context, *modelsdb.CryptoPrice) error { return errNotImplemented }
func (s *Store) UpsertCryptoPrice(context.Context, *modelsdb.CryptoPrice) error { return errNotImplemented }
func (s *Store) GetAllCryptoPricesForCoin(context.Context, string) ([]modelsdb.CryptoPrice, error) {
	return nil, errNotImplemented
}
func (s *Store) UpsertCryptoIntradayPrice(context.Context, *modelsdb.CryptoIntradayPrice) error {
	return errNotImplemented
}
func (s *Store) CreateCryptoPrediction(context.Context, *modelsdb.CryptoPrediction) error {
	return errNotImplemented
}
func (s *Store) BulkCreateCryptoPredictions(context.Context, []modelsdb.CryptoPrediction) error {
	return errNotImplemented
}
func (s *Store) DeleteCryptoPredictionsBeforeDate(context.Context, time.Time) error {
	return errNotImplemented
}

// S&P 500 writes + uncalled reads
func (s *Store) CreateSP500Price(context.Context, *modelsdb.SP500Price) error { return errNotImplemented }
func (s *Store) UpsertSP500Price(context.Context, *modelsdb.SP500Price) error { return errNotImplemented }
func (s *Store) GetAllSP500PricesForSymbol(context.Context, string) ([]modelsdb.SP500Price, error) {
	return nil, errNotImplemented
}
func (s *Store) UpsertSP500IntradayPrice(context.Context, *modelsdb.SP500IntradayPrice) error {
	return errNotImplemented
}
func (s *Store) CreateSP500Prediction(context.Context, *modelsdb.SP500Prediction) error {
	return errNotImplemented
}
func (s *Store) BulkCreateSP500Predictions(context.Context, []modelsdb.SP500Prediction) error {
	return errNotImplemented
}
func (s *Store) DeleteSP500PredictionsBeforeDate(context.Context, time.Time) error {
	return errNotImplemented
}

// Simulation writes + uncalled reads
func (s *Store) GetActiveSimBots(context.Context) ([]modelsdb.SimBot, error) {
	return nil, errNotImplemented
}
func (s *Store) CreateSimSession(context.Context, *modelsdb.SimSession) error { return errNotImplemented }
func (s *Store) UpdateSimSession(context.Context, *modelsdb.SimSession) error { return errNotImplemented }
func (s *Store) GetSimSessionsByBot(context.Context, string, int) ([]modelsdb.SimSession, error) {
	return nil, errNotImplemented
}
func (s *Store) CreateSimTrade(context.Context, *modelsdb.SimTrade) error { return errNotImplemented }
func (s *Store) CreateSimPortfolioSnapshot(context.Context, *modelsdb.SimPortfolioSnapshot) error {
	return errNotImplemented
}

// PipelineReport write
func (s *Store) DeletePipelineReportsBefore(context.Context, time.Time) (int64, error) {
	return 0, errNotImplemented
}
