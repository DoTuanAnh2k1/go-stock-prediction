package repository

import (
	"context"

	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// SimulationStore — interface for trading simulation DB operations.
type SimulationStore interface {
	// Bots
	GetAllSimBots(ctx context.Context) ([]modelsdb.SimBot, error)
	GetSimBotByID(ctx context.Context, id string) (*modelsdb.SimBot, error)
	GetActiveSimBots(ctx context.Context) ([]modelsdb.SimBot, error)
	// GetSimBotsByMarketAlgo returns all bots for a given market and algorithm key.
	GetSimBotsByMarketAlgo(ctx context.Context, market, algorithm string) ([]modelsdb.SimBot, error)
	UpdateSimBotConfig(ctx context.Context, bot *modelsdb.SimBot) error

	// Sessions
	CreateSimSession(ctx context.Context, s *modelsdb.SimSession) error
	UpdateSimSession(ctx context.Context, s *modelsdb.SimSession) error
	GetLatestSimSession(ctx context.Context, botID string) (*modelsdb.SimSession, error)
	GetBestSimSessionForChart(ctx context.Context, botID string) (*modelsdb.SimSession, error)
	GetLatestLiveSimSession(ctx context.Context, botID string) (*modelsdb.SimSession, error)
	GetSimSessionsByBot(ctx context.Context, botID string, limit int) ([]modelsdb.SimSession, error)

	// Trades
	CreateSimTrade(ctx context.Context, t *modelsdb.SimTrade) error
	// GetSimTrades returns paginated trades for a session. When excludeHold is true,
	// HOLD rows are filtered out at the DB level so total/pagination reflect only BUY/SELL.
	GetSimTrades(ctx context.Context, sessionID int64, offset, limit int, excludeHold bool) ([]modelsdb.SimTrade, int64, error)

	// Snapshots
	CreateSimPortfolioSnapshot(ctx context.Context, s *modelsdb.SimPortfolioSnapshot) error
	GetSimPortfolioSnapshots(ctx context.Context, sessionID int64) ([]modelsdb.SimPortfolioSnapshot, error)

	// Batch methods — replace per-bot N+1 queries in leaderboard/monitoring.
	// GetAllSessionsWithSnapCount returns all sessions with their snapshot count, ordered by bot_id ASC, id DESC.
	GetAllSessionsWithSnapCount(ctx context.Context) ([]modelsdb.SimSessionWithCount, error)
	// GetSessionTradeStatsBatch returns pre-aggregated trade stats per session (SQL GROUP BY).
	GetSessionTradeStatsBatch(ctx context.Context, sessionIDs []int64) (map[int64]modelsdb.SimTradeStats, error)
	// GetLastSnapshotsBatch returns the last portfolio snapshot per session.
	GetLastSnapshotsBatch(ctx context.Context, sessionIDs []int64) (map[int64]*modelsdb.SimPortfolioSnapshot, error)
	// GetLeaderboardEntries returns one row per bot using pre-computed KPI columns.
	// Picks the best session per bot: live+running > most snapshots > latest ID.
	// Results sorted by total_return_pct DESC NULLS LAST.
	GetLeaderboardEntries(ctx context.Context) ([]modelsdb.LeaderboardEntry, error)
}
