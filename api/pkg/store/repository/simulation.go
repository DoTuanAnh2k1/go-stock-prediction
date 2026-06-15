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
	GetSimTrades(sessionID int64, offset, limit int) ([]modelsdb.SimTrade, int64, error)

	// Snapshots
	CreateSimPortfolioSnapshot(s *modelsdb.SimPortfolioSnapshot) error
	GetSimPortfolioSnapshots(sessionID int64) ([]modelsdb.SimPortfolioSnapshot, error)
}
