package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// GetAllSimBots returns all simulation bot configurations.
func (c *Client) GetAllSimBots() ([]modelsdb.SimBot, error) {
	var bots []modelsdb.SimBot
	err := c.Db.Find(&bots).Error
	return bots, err
}

// GetSimBotByID returns a single bot by its string ID.
func (c *Client) GetSimBotByID(id string) (*modelsdb.SimBot, error) {
	var bot modelsdb.SimBot
	err := c.Db.Where("id = ?", id).First(&bot).Error
	if err != nil {
		return nil, err
	}
	return &bot, nil
}

// GetActiveSimBots returns all bots where is_active = true.
func (c *Client) GetActiveSimBots() ([]modelsdb.SimBot, error) {
	var bots []modelsdb.SimBot
	err := c.Db.Where("is_active = ?", true).Find(&bots).Error
	return bots, err
}

// UpdateSimBotConfig saves (full replace) a bot configuration record.
func (c *Client) UpdateSimBotConfig(bot *modelsdb.SimBot) error {
	return c.Db.Save(bot).Error
}

// CreateSimSession inserts a new simulation session.
func (c *Client) CreateSimSession(s *modelsdb.SimSession) error {
	return c.Db.Create(s).Error
}

// UpdateSimSession saves (full replace) a simulation session.
func (c *Client) UpdateSimSession(s *modelsdb.SimSession) error {
	return c.Db.Save(s).Error
}

// GetLatestSimSession returns the most recently created session for a bot.
func (c *Client) GetLatestSimSession(botID string) (*modelsdb.SimSession, error) {
	var s modelsdb.SimSession
	err := c.Db.Where("bot_id = ?", botID).
		Order("created_at DESC").
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetBestSimSessionForChart returns the session with the most portfolio snapshots.
// This avoids showing a nearly-empty running session when a completed backtest exists.
func (c *Client) GetBestSimSessionForChart(botID string) (*modelsdb.SimSession, error) {
	var s modelsdb.SimSession
	err := c.Db.Raw(`
		SELECT s.* FROM sim_sessions s
		INNER JOIN (
			SELECT session_id, COUNT(*) AS cnt
			FROM sim_portfolio_snapshots
			WHERE bot_id = ?
			GROUP BY session_id
			ORDER BY cnt DESC
			LIMIT 1
		) best ON s.id = best.session_id
	`, botID).Scan(&s).Error
	if err != nil || s.ID == 0 {
		return nil, err
	}
	return &s, nil
}

// GetSimSessionsByBot returns sessions for a bot ordered newest-first, up to limit rows.
func (c *Client) GetSimSessionsByBot(botID string, limit int) ([]modelsdb.SimSession, error) {
	var sessions []modelsdb.SimSession
	err := c.Db.Where("bot_id = ?", botID).
		Order("created_at DESC").
		Limit(limit).
		Find(&sessions).Error
	return sessions, err
}

// CreateSimTrade inserts a new trade record.
func (c *Client) CreateSimTrade(t *modelsdb.SimTrade) error {
	return c.Db.Create(t).Error
}

// GetSimTrades returns paginated trades for a session, ordered by trade_date ASC.
// It also returns the total count (before pagination).
func (c *Client) GetSimTrades(sessionID int64, offset, limit int) ([]modelsdb.SimTrade, int64, error) {
	var trades []modelsdb.SimTrade
	var total int64

	query := c.Db.Model(&modelsdb.SimTrade{}).Where("session_id = ?", sessionID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("trade_date ASC").
		Offset(offset).
		Limit(limit).
		Find(&trades).Error
	return trades, total, err
}

// CreateSimPortfolioSnapshot inserts a daily portfolio snapshot.
func (c *Client) CreateSimPortfolioSnapshot(s *modelsdb.SimPortfolioSnapshot) error {
	return c.Db.Create(s).Error
}

// GetSimPortfolioSnapshots returns all daily snapshots for a session ordered ASC.
func (c *Client) GetSimPortfolioSnapshots(sessionID int64) ([]modelsdb.SimPortfolioSnapshot, error) {
	var snaps []modelsdb.SimPortfolioSnapshot
	err := c.Db.Where("session_id = ?", sessionID).
		Order("snapshot_date ASC").
		Find(&snaps).Error
	return snaps, err
}
