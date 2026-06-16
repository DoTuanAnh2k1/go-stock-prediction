package repository

import (
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"time"
)

// SessionStatsStore provides session-window aggregation queries.
type SessionStatsStore interface {
	// GetSessionDirAccuracy returns direction accuracy per algorithm for predictions
	// created within [from, to) for the given market.
	// market must be one of: GOLD, NASDAQ, CRYPTO, SP500.
	GetSessionDirAccuracy(market string, from, to time.Time) ([]modelsapi.SessionDirAccRow, error)

	// GetSessionBotTrades returns per-bot trade stats with portfolio snapshot data for trades
	// executed within [from, to) for the given market.
	GetSessionBotTrades(market string, from, to time.Time) ([]modelsapi.SessionBotDetail, error)
}
