package repository

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// SimulationStore — interface for trading simulation DB operations.
type SimulationStore interface {
	// Bots
	GetAllSimBots() ([]modelsdb.SimBot, error)
	GetSimBotByID(id string) (*modelsdb.SimBot, error)
	GetActiveSimBots() ([]modelsdb.SimBot, error)
	// GetSimBotsByMarketAlgo returns all bots for a given market and algorithm key.
	GetSimBotsByMarketAlgo(market, algorithm string) ([]modelsdb.SimBot, error)
	UpdateSimBotConfig(bot *modelsdb.SimBot) error

	// Sessions
	CreateSimSession(s *modelsdb.SimSession) error
	UpdateSimSession(s *modelsdb.SimSession) error
	GetLatestSimSession(botID string) (*modelsdb.SimSession, error)
	GetBestSimSessionForChart(botID string) (*modelsdb.SimSession, error)
	GetLatestLiveSimSession(botID string) (*modelsdb.SimSession, error)
	GetSimSessionsByBot(botID string, limit int) ([]modelsdb.SimSession, error)

	// Trades
	CreateSimTrade(t *modelsdb.SimTrade) error
	// GetSimTrades returns paginated trades for a session. When excludeHold is true,
	// HOLD rows are filtered out at the DB level so total/pagination reflect only BUY/SELL.
	GetSimTrades(sessionID int64, offset, limit int, excludeHold bool) ([]modelsdb.SimTrade, int64, error)

	// Snapshots
	CreateSimPortfolioSnapshot(s *modelsdb.SimPortfolioSnapshot) error
	GetSimPortfolioSnapshots(sessionID int64) ([]modelsdb.SimPortfolioSnapshot, error)

	// Batch methods — replace per-bot N+1 queries in leaderboard/monitoring.
	// GetAllSessionsWithSnapCount returns all sessions with their snapshot count, ordered by bot_id ASC, id DESC.
	GetAllSessionsWithSnapCount() ([]modelsdb.SimSessionWithCount, error)
	// GetSessionTradeStatsBatch returns pre-aggregated trade stats per session (SQL GROUP BY).
	GetSessionTradeStatsBatch(sessionIDs []int64) (map[int64]modelsdb.SimTradeStats, error)
	// GetLastSnapshotsBatch returns the last portfolio snapshot per session.
	GetLastSnapshotsBatch(sessionIDs []int64) (map[int64]*modelsdb.SimPortfolioSnapshot, error)
	// GetLeaderboardEntries returns one row per bot using pre-computed KPI columns.
	// Picks the best session per bot: live+running > most snapshots > latest ID.
	// Results sorted by total_return_pct DESC NULLS LAST.
	GetLeaderboardEntries() ([]modelsdb.LeaderboardEntry, error)
}
