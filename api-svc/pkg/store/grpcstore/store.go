// Package grpcstore implements repository.DatabaseStore by delegating every read
// to prediction-svc over gRPC (the generic Query RPC). This lets api-svc be
// DB-less: it owns no database connection and reaches market_db only through the
// service that owns it (Phase 2 of the database-per-service work).
//
// Contract: query(method, params) sends a JSON object of named params and
// unmarshals result_json into the Go return type. Because we unmarshal into the
// SAME modelsdb/modelsapi types the handlers already use, JSON keys are the Go
// json tags (= DB columns); Decimal accepts string or number; time is RFC3339.
//
// api-svc only ever calls read methods plus UpsertCronSchedule / UpdateSimBotConfig
// (the two things it writes). Every other interface method is a compile-time stub
// that returns errNotImplemented — they are never invoked from api-svc.
package grpcstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go-stock-prediction/pkg/models/models_config"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	predictionpb "go-stock-prediction/proto/prediction"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var errNotImplemented = errors.New("grpcstore: method not exposed over the read facade (api-svc never calls it)")

// Store is the gRPC-backed DatabaseStore.
type Store struct {
	target string
	conn   *grpc.ClientConn
	client predictionpb.PredictionServiceClient
}

func New(target string) *Store { return &Store{target: target} }

func (s *Store) Init(_ models_config.DatabaseConfig) error {
	conn, err := grpc.NewClient(s.target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	s.conn = conn
	s.client = predictionpb.NewPredictionServiceClient(conn)
	return nil
}

func (s *Store) Ping() error {
	// A lightweight round-trip; GetDistinctPipelineKeys is cheap and always present.
	var out []string
	return s.query(context.Background(), "Ping", nil, &out)
}

func (s *Store) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// query is the single transport primitive: marshal params → Query RPC → unmarshal
// result_json into out. out may be nil for writes that return no body.
func (s *Store) query(ctx context.Context, method string, params map[string]any, out any) error {
	if params == nil {
		params = map[string]any{}
	}
	pj, err := json.Marshal(params)
	if err != nil {
		return err
	}
	resp, err := s.client.Query(ctx, &predictionpb.QueryRequest{Method: method, ParamsJson: string(pj)})
	if err != nil {
		return err
	}
	if resp.GetError() != "" {
		return errors.New(resp.GetError())
	}
	if out == nil || resp.GetResultJson() == "" {
		return nil
	}
	return json.Unmarshal([]byte(resp.GetResultJson()), out)
}

// page is the shared envelope for paginated results (items, total).
type page[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

func rfc(t time.Time) string { return t.Format(time.RFC3339) }

// ─────────────────────────────────────────────────────────────────────────────
// Cron schedules
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetAllCronSchedules(ctx context.Context) ([]modelsdb.CronSchedule, error) {
	var out []modelsdb.CronSchedule
	return out, s.query(ctx, "GetAllCronSchedules", nil, &out)
}

func (s *Store) GetCronScheduleByKey(ctx context.Context, jobKey string) (*modelsdb.CronSchedule, error) {
	var out *modelsdb.CronSchedule
	return out, s.query(ctx, "GetCronScheduleByKey", map[string]any{"job_key": jobKey}, &out)
}

func (s *Store) UpsertCronSchedule(ctx context.Context, sc *modelsdb.CronSchedule) error {
	return s.query(ctx, "UpsertCronSchedule", map[string]any{"schedule": sc}, nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// Training logs
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetTrainingLogByID(ctx context.Context, id uint) (*modelsdb.TrainingLog, error) {
	var out *modelsdb.TrainingLog
	return out, s.query(ctx, "GetTrainingLogByID", map[string]any{"id": id}, &out)
}

func (s *Store) GetTrainingLogsBySessionID(ctx context.Context, sessionID string) ([]modelsdb.TrainingLog, error) {
	var out []modelsdb.TrainingLog
	return out, s.query(ctx, "GetTrainingLogsBySessionID", map[string]any{"session_id": sessionID}, &out)
}

func (s *Store) GetTrainingSessions(ctx context.Context, limit, offset int) ([]modelsdb.TrainingLog, int64, error) {
	var out page[modelsdb.TrainingLog]
	err := s.query(ctx, "GetTrainingSessions", map[string]any{"limit": limit, "offset": offset}, &out)
	return out.Items, out.Total, err
}

func (s *Store) GetLatestTrainingLogByAlgorithm(ctx context.Context) ([]modelsdb.TrainingLog, error) {
	var out []modelsdb.TrainingLog
	return out, s.query(ctx, "GetLatestTrainingLogByAlgorithm", nil, &out)
}

func (s *Store) GetTrainingMetricsAggregate(ctx context.Context) (modelsdb.TrainingMetricsAggregate, error) {
	var out modelsdb.TrainingMetricsAggregate
	return out, s.query(ctx, "GetTrainingMetricsAggregate", nil, &out)
}

func (s *Store) GetTrainingSessionsByMarket(ctx context.Context, marketKey string, page, limit int, algorithm, sortBy, sortDir string) ([]modelsdb.TrainingLog, int64, error) {
	var out pageAlias[modelsdb.TrainingLog]
	err := s.query(ctx, "GetTrainingSessionsByMarket", map[string]any{
		"market_key": marketKey, "page": page, "limit": limit,
		"algorithm": algorithm, "sort_by": sortBy, "sort_dir": sortDir,
	}, &out)
	return out.Items, out.Total, err
}

// pageAlias is identical to page[T]; a second name avoids re-declaring the generic
// inline in every multi-return signature.
type pageAlias[T any] = page[T]

// ─────────────────────────────────────────────────────────────────────────────
// Sync logs
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetLatestSyncLogs(ctx context.Context, limit int) ([]modelsdb.SyncLog, error) {
	var out []modelsdb.SyncLog
	return out, s.query(ctx, "GetLatestSyncLogs", map[string]any{"limit": limit}, &out)
}

// ─────────────────────────────────────────────────────────────────────────────
// GOLD
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetGoldPricesByDateRange(ctx context.Context, source, productType string, from, to time.Time) ([]modelsdb.GoldPrice, error) {
	var out []modelsdb.GoldPrice
	return out, s.query(ctx, "GetGoldPricesByDateRange", map[string]any{"source": source, "product_type": productType, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetLatestGoldPrices(ctx context.Context) ([]modelsdb.GoldPrice, error) {
	var out []modelsdb.GoldPrice
	return out, s.query(ctx, "GetLatestGoldPrices", nil, &out)
}

func (s *Store) GetGoldIntradayByRange(ctx context.Context, source string, from, to time.Time) ([]modelsdb.GoldIntradayPrice, error) {
	var out []modelsdb.GoldIntradayPrice
	return out, s.query(ctx, "GetGoldIntradayByRange", map[string]any{"source": source, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetGoldPredictions(ctx context.Context, source, productType, algorithm string, limit int) ([]modelsdb.GoldPrediction, error) {
	var out []modelsdb.GoldPrediction
	return out, s.query(ctx, "GetGoldPredictions", map[string]any{"source": source, "product_type": productType, "algorithm": algorithm, "limit": limit}, &out)
}

func (s *Store) GetLatestGoldPredictions(ctx context.Context) ([]modelsdb.GoldPrediction, error) {
	var out []modelsdb.GoldPrediction
	return out, s.query(ctx, "GetLatestGoldPredictions", nil, &out)
}

func (s *Store) GetLatestConfirmedGoldPredictions(ctx context.Context) ([]modelsdb.GoldPrediction, error) {
	var out []modelsdb.GoldPrediction
	return out, s.query(ctx, "GetLatestConfirmedGoldPredictions", nil, &out)
}

func (s *Store) GetGoldPredictionsByDateRange(ctx context.Context, source, productType string, from, to time.Time) ([]modelsdb.GoldPrediction, error) {
	var out []modelsdb.GoldPrediction
	return out, s.query(ctx, "GetGoldPredictionsByDateRange", map[string]any{"source": source, "product_type": productType, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetGoldPredictionsPage(ctx context.Context, pageNum, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.GoldPrediction, int64, error) {
	var out page[modelsdb.GoldPrediction]
	err := s.query(ctx, "GetGoldPredictionsPage", predPageParams(pageNum, limit, search, algorithm, status, sortBy, sortDir), &out)
	return out.Items, out.Total, err
}

func predPageParams(pageNum, limit int, search, algorithm, status, sortBy, sortDir string) map[string]any {
	return map[string]any{"page": pageNum, "limit": limit, "search": search, "algorithm": algorithm, "status": status, "sort_by": sortBy, "sort_dir": sortDir}
}

// ─────────────────────────────────────────────────────────────────────────────
// NASDAQ
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetNasdaqPricesByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.NasdaqPrice, error) {
	var out []modelsdb.NasdaqPrice
	return out, s.query(ctx, "GetNasdaqPricesByDateRange", map[string]any{"symbol": symbol, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetLatestNasdaqPrice(ctx context.Context, symbol string) (*modelsdb.NasdaqPrice, error) {
	var out *modelsdb.NasdaqPrice
	return out, s.query(ctx, "GetLatestNasdaqPrice", map[string]any{"symbol": symbol}, &out)
}

func (s *Store) GetNasdaqSymbols(ctx context.Context) ([]string, error) {
	var out []string
	return out, s.query(ctx, "GetNasdaqSymbols", nil, &out)
}

func (s *Store) GetNasdaqIntradayByRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.NasdaqIntradayPrice, error) {
	var out []modelsdb.NasdaqIntradayPrice
	return out, s.query(ctx, "GetNasdaqIntradayByRange", map[string]any{"symbol": symbol, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetNasdaqPredictions(ctx context.Context, symbol, algorithm string, limit int) ([]modelsdb.NasdaqPrediction, error) {
	var out []modelsdb.NasdaqPrediction
	return out, s.query(ctx, "GetNasdaqPredictions", map[string]any{"symbol": symbol, "algorithm": algorithm, "limit": limit}, &out)
}

func (s *Store) GetLatestNasdaqPredictions(ctx context.Context) ([]modelsdb.NasdaqPrediction, error) {
	var out []modelsdb.NasdaqPrediction
	return out, s.query(ctx, "GetLatestNasdaqPredictions", nil, &out)
}

func (s *Store) GetLatestConfirmedNasdaqPredictions(ctx context.Context) ([]modelsdb.NasdaqPrediction, error) {
	var out []modelsdb.NasdaqPrediction
	return out, s.query(ctx, "GetLatestConfirmedNasdaqPredictions", nil, &out)
}

func (s *Store) GetNasdaqPredictionsByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.NasdaqPrediction, error) {
	var out []modelsdb.NasdaqPrediction
	return out, s.query(ctx, "GetNasdaqPredictionsByDateRange", map[string]any{"symbol": symbol, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetNasdaqPredictionsPage(ctx context.Context, pageNum, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.NasdaqPrediction, int64, error) {
	var out page[modelsdb.NasdaqPrediction]
	err := s.query(ctx, "GetNasdaqPredictionsPage", predPageParams(pageNum, limit, search, algorithm, status, sortBy, sortDir), &out)
	return out.Items, out.Total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// CRYPTO
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetCryptoPricesByDateRange(ctx context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoPrice, error) {
	var out []modelsdb.CryptoPrice
	return out, s.query(ctx, "GetCryptoPricesByDateRange", map[string]any{"coin_id": coinID, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetLatestCryptoPrice(ctx context.Context, coinID string) (*modelsdb.CryptoPrice, error) {
	var out *modelsdb.CryptoPrice
	return out, s.query(ctx, "GetLatestCryptoPrice", map[string]any{"coin_id": coinID}, &out)
}

func (s *Store) GetCryptoCoins(ctx context.Context) ([]modelsdb.CryptoPrice, error) {
	var out []modelsdb.CryptoPrice
	return out, s.query(ctx, "GetCryptoCoins", nil, &out)
}

func (s *Store) GetCryptoIntradayByRange(ctx context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoIntradayPrice, error) {
	var out []modelsdb.CryptoIntradayPrice
	return out, s.query(ctx, "GetCryptoIntradayByRange", map[string]any{"coin_id": coinID, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetCryptoPredictions(ctx context.Context, coinID, algorithm string, limit int) ([]modelsdb.CryptoPrediction, error) {
	var out []modelsdb.CryptoPrediction
	return out, s.query(ctx, "GetCryptoPredictions", map[string]any{"coin_id": coinID, "algorithm": algorithm, "limit": limit}, &out)
}

func (s *Store) GetLatestCryptoPredictions(ctx context.Context) ([]modelsdb.CryptoPrediction, error) {
	var out []modelsdb.CryptoPrediction
	return out, s.query(ctx, "GetLatestCryptoPredictions", nil, &out)
}

func (s *Store) GetLatestConfirmedCryptoPredictions(ctx context.Context) ([]modelsdb.CryptoPrediction, error) {
	var out []modelsdb.CryptoPrediction
	return out, s.query(ctx, "GetLatestConfirmedCryptoPredictions", nil, &out)
}

func (s *Store) GetCryptoPredictionsByDateRange(ctx context.Context, coinID string, from, to time.Time) ([]modelsdb.CryptoPrediction, error) {
	var out []modelsdb.CryptoPrediction
	return out, s.query(ctx, "GetCryptoPredictionsByDateRange", map[string]any{"coin_id": coinID, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetCryptoPredictionsPage(ctx context.Context, pageNum, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.CryptoPrediction, int64, error) {
	var out page[modelsdb.CryptoPrediction]
	err := s.query(ctx, "GetCryptoPredictionsPage", predPageParams(pageNum, limit, search, algorithm, status, sortBy, sortDir), &out)
	return out.Items, out.Total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// S&P 500
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetSP500PricesByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.SP500Price, error) {
	var out []modelsdb.SP500Price
	return out, s.query(ctx, "GetSP500PricesByDateRange", map[string]any{"symbol": symbol, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetLatestSP500Price(ctx context.Context, symbol string) (*modelsdb.SP500Price, error) {
	var out *modelsdb.SP500Price
	return out, s.query(ctx, "GetLatestSP500Price", map[string]any{"symbol": symbol}, &out)
}

func (s *Store) GetSP500Symbols(ctx context.Context) ([]string, error) {
	var out []string
	return out, s.query(ctx, "GetSP500Symbols", nil, &out)
}

func (s *Store) GetSP500IntradayByRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.SP500IntradayPrice, error) {
	var out []modelsdb.SP500IntradayPrice
	return out, s.query(ctx, "GetSP500IntradayByRange", map[string]any{"symbol": symbol, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetSP500Predictions(ctx context.Context, symbol, algorithm string, limit int) ([]modelsdb.SP500Prediction, error) {
	var out []modelsdb.SP500Prediction
	return out, s.query(ctx, "GetSP500Predictions", map[string]any{"symbol": symbol, "algorithm": algorithm, "limit": limit}, &out)
}

func (s *Store) GetLatestSP500Predictions(ctx context.Context) ([]modelsdb.SP500Prediction, error) {
	var out []modelsdb.SP500Prediction
	return out, s.query(ctx, "GetLatestSP500Predictions", nil, &out)
}

func (s *Store) GetLatestConfirmedSP500Predictions(ctx context.Context) ([]modelsdb.SP500Prediction, error) {
	var out []modelsdb.SP500Prediction
	return out, s.query(ctx, "GetLatestConfirmedSP500Predictions", nil, &out)
}

func (s *Store) GetSP500PredictionsByDateRange(ctx context.Context, symbol string, from, to time.Time) ([]modelsdb.SP500Prediction, error) {
	var out []modelsdb.SP500Prediction
	return out, s.query(ctx, "GetSP500PredictionsByDateRange", map[string]any{"symbol": symbol, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetSP500PredictionsPage(ctx context.Context, pageNum, limit int, search, algorithm, status, sortBy, sortDir string) ([]modelsdb.SP500Prediction, int64, error) {
	var out page[modelsdb.SP500Prediction]
	err := s.query(ctx, "GetSP500PredictionsPage", predPageParams(pageNum, limit, search, algorithm, status, sortBy, sortDir), &out)
	return out.Items, out.Total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Simulation
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetAllSimBots(ctx context.Context) ([]modelsdb.SimBot, error) {
	var out []modelsdb.SimBot
	return out, s.query(ctx, "GetAllSimBots", nil, &out)
}

func (s *Store) GetSimBotByID(ctx context.Context, id string) (*modelsdb.SimBot, error) {
	var out *modelsdb.SimBot
	return out, s.query(ctx, "GetSimBotByID", map[string]any{"id": id}, &out)
}

func (s *Store) GetSimBotsByMarketAlgo(ctx context.Context, market, algorithm string) ([]modelsdb.SimBot, error) {
	var out []modelsdb.SimBot
	return out, s.query(ctx, "GetSimBotsByMarketAlgo", map[string]any{"market": market, "algorithm": algorithm}, &out)
}

func (s *Store) UpdateSimBotConfig(ctx context.Context, bot *modelsdb.SimBot) error {
	return s.query(ctx, "UpdateSimBotConfig", map[string]any{"bot": bot}, nil)
}

func (s *Store) GetLatestSimSession(ctx context.Context, botID string) (*modelsdb.SimSession, error) {
	var out *modelsdb.SimSession
	return out, s.query(ctx, "GetLatestSimSession", map[string]any{"bot_id": botID}, &out)
}

func (s *Store) GetBestSimSessionForChart(ctx context.Context, botID string) (*modelsdb.SimSession, error) {
	var out *modelsdb.SimSession
	return out, s.query(ctx, "GetBestSimSessionForChart", map[string]any{"bot_id": botID}, &out)
}

func (s *Store) GetLatestLiveSimSession(ctx context.Context, botID string) (*modelsdb.SimSession, error) {
	var out *modelsdb.SimSession
	return out, s.query(ctx, "GetLatestLiveSimSession", map[string]any{"bot_id": botID}, &out)
}

func (s *Store) GetSimTrades(ctx context.Context, sessionID int64, offset, limit int, excludeHold bool) ([]modelsdb.SimTrade, int64, error) {
	var out page[modelsdb.SimTrade]
	err := s.query(ctx, "GetSimTrades", map[string]any{"session_id": sessionID, "offset": offset, "limit": limit, "exclude_hold": excludeHold}, &out)
	return out.Items, out.Total, err
}

func (s *Store) GetSimPortfolioSnapshots(ctx context.Context, sessionID int64) ([]modelsdb.SimPortfolioSnapshot, error) {
	var out []modelsdb.SimPortfolioSnapshot
	return out, s.query(ctx, "GetSimPortfolioSnapshots", map[string]any{"session_id": sessionID}, &out)
}

func (s *Store) GetAllSessionsWithSnapCount(ctx context.Context) ([]modelsdb.SimSessionWithCount, error) {
	var out []modelsdb.SimSessionWithCount
	return out, s.query(ctx, "GetAllSessionsWithSnapCount", nil, &out)
}

func (s *Store) GetSessionTradeStatsBatch(ctx context.Context, sessionIDs []int64) (map[int64]modelsdb.SimTradeStats, error) {
	var out map[int64]modelsdb.SimTradeStats
	return out, s.query(ctx, "GetSessionTradeStatsBatch", map[string]any{"session_ids": sessionIDs}, &out)
}

func (s *Store) GetLastSnapshotsBatch(ctx context.Context, sessionIDs []int64) (map[int64]*modelsdb.SimPortfolioSnapshot, error) {
	var out map[int64]*modelsdb.SimPortfolioSnapshot
	return out, s.query(ctx, "GetLastSnapshotsBatch", map[string]any{"session_ids": sessionIDs}, &out)
}

func (s *Store) GetLeaderboardEntries(ctx context.Context) ([]modelsdb.LeaderboardEntry, error) {
	var out []modelsdb.LeaderboardEntry
	return out, s.query(ctx, "GetLeaderboardEntries", nil, &out)
}

// ─────────────────────────────────────────────────────────────────────────────
// Direction accuracy / monitoring / session stats / pipeline
// ─────────────────────────────────────────────────────────────────────────────

func (s *Store) GetDirectionAccuracy(ctx context.Context, market string) ([]modelsapi.DirectionAccuracyRow, error) {
	var out []modelsapi.DirectionAccuracyRow
	return out, s.query(ctx, "GetDirectionAccuracy", map[string]any{"market": market}, &out)
}

func (s *Store) GetMarketCrawlStats(ctx context.Context, market string) (*modelsapi.MarketCrawlStats, error) {
	var out *modelsapi.MarketCrawlStats
	return out, s.query(ctx, "GetMarketCrawlStats", map[string]any{"market": market}, &out)
}

func (s *Store) GetMarketPredStats(ctx context.Context, market string) ([]modelsapi.AlgoPredStats, error) {
	var out []modelsapi.AlgoPredStats
	return out, s.query(ctx, "GetMarketPredStats", map[string]any{"market": market}, &out)
}

func (s *Store) GetSessionDirAccuracy(ctx context.Context, market string, from, to time.Time) ([]modelsapi.SessionDirAccRow, error) {
	var out []modelsapi.SessionDirAccRow
	return out, s.query(ctx, "GetSessionDirAccuracy", map[string]any{"market": market, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetSessionBotTrades(ctx context.Context, market string, from, to time.Time) ([]modelsapi.SessionBotDetail, error) {
	var out []modelsapi.SessionBotDetail
	return out, s.query(ctx, "GetSessionBotTrades", map[string]any{"market": market, "from": rfc(from), "to": rfc(to)}, &out)
}

func (s *Store) GetPipelineReports(ctx context.Context, pipelineKey string, limit int) ([]modelsdb.PipelineReport, error) {
	var out []modelsdb.PipelineReport
	return out, s.query(ctx, "GetPipelineReports", map[string]any{"pipeline_key": pipelineKey, "limit": limit}, &out)
}

func (s *Store) GetDistinctPipelineKeys(ctx context.Context) ([]string, error) {
	var out []string
	return out, s.query(ctx, "GetDistinctPipelineKeys", nil, &out)
}

// Decimal is referenced only to keep the import when all price fields are pointers;
// used by the unused write stubs below to satisfy signatures without importing lazily.
var _ = decimal.Zero
