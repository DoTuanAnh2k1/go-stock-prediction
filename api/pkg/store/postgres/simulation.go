package postgres

import (
	"time"

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

// GetSimBotsByMarketAlgo returns all bots with the given market and algorithm,
// ordered by ID for consistent display.
func (c *Client) GetSimBotsByMarketAlgo(market, algorithm string) ([]modelsdb.SimBot, error) {
	var bots []modelsdb.SimBot
	err := c.Db.Where("market = ? AND algorithm = ?", market, algorithm).
		Order("id ASC").
		Find(&bots).Error
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

// GetLatestLiveSimSession returns the most recent running live session for a bot.
// Returns nil error + nil session if none exists.
func (c *Client) GetLatestLiveSimSession(botID string) (*modelsdb.SimSession, error) {
	var s modelsdb.SimSession
	err := c.Db.Where("bot_id = ? AND mode = 'live' AND status = 'running'", botID).
		Order("id DESC").
		First(&s).Error
	if err != nil {
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

// GetAllSessionsWithSnapCount returns all sim sessions with their portfolio snapshot counts.
func (c *Client) GetAllSessionsWithSnapCount() ([]modelsdb.SimSessionWithCount, error) {
	type rawRow struct {
		ID        int64      `gorm:"column:id"`
		BotID     string     `gorm:"column:bot_id"`
		StartDate time.Time  `gorm:"column:start_date"`
		EndDate   *time.Time `gorm:"column:end_date"`
		Status    string     `gorm:"column:status"`
		Mode      string     `gorm:"column:mode"`
		CreatedAt time.Time  `gorm:"column:created_at"`
		SnapCount int        `gorm:"column:snap_count"`
	}
	var rows []rawRow
	err := c.Db.Raw(`
		SELECT s.id, s.bot_id, s.start_date, s.end_date, s.status, s.mode, s.created_at,
		       COALESCE(sc.cnt, 0) AS snap_count
		FROM sim_sessions s
		LEFT JOIN (
		    SELECT session_id, COUNT(*) AS cnt
		    FROM sim_portfolio_snapshots
		    GROUP BY session_id
		) sc ON s.id = sc.session_id
		ORDER BY s.bot_id ASC, s.id DESC
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]modelsdb.SimSessionWithCount, len(rows))
	for i, r := range rows {
		result[i] = modelsdb.SimSessionWithCount{
			SimSession: modelsdb.SimSession{
				ID:        r.ID,
				BotID:     r.BotID,
				StartDate: r.StartDate,
				EndDate:   r.EndDate,
				Status:    r.Status,
				Mode:      r.Mode,
				CreatedAt: r.CreatedAt,
			},
			SnapCount: r.SnapCount,
		}
	}
	return result, nil
}

// GetSessionTradeStatsBatch returns pre-aggregated trade stats per session via SQL GROUP BY.
func (c *Client) GetSessionTradeStatsBatch(sessionIDs []int64) (map[int64]modelsdb.SimTradeStats, error) {
	if len(sessionIDs) == 0 {
		return map[int64]modelsdb.SimTradeStats{}, nil
	}
	type rawRow struct {
		SessionID   int64   `gorm:"column:session_id"`
		TotalTrades int     `gorm:"column:total_trades"`
		Wins        int     `gorm:"column:wins"`
		Losses      int     `gorm:"column:losses"`
		Breakeven   int     `gorm:"column:breakeven"`
		TotalPnl    float64 `gorm:"column:total_pnl"`
		WinPnl      float64 `gorm:"column:win_pnl"`
		LossPnl     float64 `gorm:"column:loss_pnl"`
	}
	var rows []rawRow
	err := c.Db.Raw(`
		SELECT
		    session_id,
		    COUNT(CASE WHEN action = 'SELL' THEN 1 END)                                             AS total_trades,
		    COUNT(CASE WHEN action = 'SELL' AND COALESCE(pnl_pct, pnl) > 0 THEN 1 END)             AS wins,
		    COUNT(CASE WHEN action = 'SELL' AND COALESCE(pnl_pct, pnl) < 0 THEN 1 END)             AS losses,
		    COUNT(CASE WHEN action = 'SELL' AND COALESCE(pnl_pct, pnl) = 0 THEN 1 END)             AS breakeven,
		    COALESCE(SUM(CASE WHEN action = 'SELL' THEN COALESCE(pnl, 0) END), 0)                  AS total_pnl,
		    COALESCE(SUM(CASE WHEN action = 'SELL' AND pnl > 0 THEN pnl ELSE 0 END), 0)            AS win_pnl,
		    COALESCE(SUM(CASE WHEN action = 'SELL' AND pnl < 0 THEN ABS(pnl) ELSE 0 END), 0)       AS loss_pnl
		FROM sim_trades
		WHERE session_id IN ?
		GROUP BY session_id
	`, sessionIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64]modelsdb.SimTradeStats, len(rows))
	for _, r := range rows {
		result[r.SessionID] = modelsdb.SimTradeStats{
			TotalTrades: r.TotalTrades,
			Wins:        r.Wins,
			Losses:      r.Losses,
			Breakeven:   r.Breakeven,
			TotalPnl:    r.TotalPnl,
			WinPnl:      r.WinPnl,
			LossPnl:     r.LossPnl,
		}
	}
	return result, nil
}

// GetLastSnapshotsBatch returns the last portfolio snapshot per session.
func (c *Client) GetLastSnapshotsBatch(sessionIDs []int64) (map[int64]*modelsdb.SimPortfolioSnapshot, error) {
	if len(sessionIDs) == 0 {
		return map[int64]*modelsdb.SimPortfolioSnapshot{}, nil
	}
	var snaps []modelsdb.SimPortfolioSnapshot
	err := c.Db.Raw(`
		SELECT sp.*
		FROM sim_portfolio_snapshots sp
		INNER JOIN (
		    SELECT session_id, MAX(snapshot_date) AS last_date
		    FROM sim_portfolio_snapshots
		    WHERE session_id IN ?
		    GROUP BY session_id
		) latest ON sp.session_id = latest.session_id AND sp.snapshot_date = latest.last_date
	`, sessionIDs).Scan(&snaps).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*modelsdb.SimPortfolioSnapshot, len(snaps))
	for i := range snaps {
		s := snaps[i]
		result[s.SessionID] = &s
	}
	return result, nil
}

// GetAllSnapshotsBatch returns all snapshots for the given sessions, grouped by session_id.
func (c *Client) GetAllSnapshotsBatch(sessionIDs []int64) (map[int64][]modelsdb.SimPortfolioSnapshot, error) {
	if len(sessionIDs) == 0 {
		return map[int64][]modelsdb.SimPortfolioSnapshot{}, nil
	}
	var snaps []modelsdb.SimPortfolioSnapshot
	err := c.Db.Where("session_id IN ?", sessionIDs).
		Order("session_id ASC, snapshot_date ASC").
		Find(&snaps).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]modelsdb.SimPortfolioSnapshot)
	for _, s := range snaps {
		result[s.SessionID] = append(result[s.SessionID], s)
	}
	return result, nil
}

// GetAllTradesBatch returns all trades for the given sessions, grouped by session_id.
func (c *Client) GetAllTradesBatch(sessionIDs []int64) (map[int64][]modelsdb.SimTrade, error) {
	if len(sessionIDs) == 0 {
		return map[int64][]modelsdb.SimTrade{}, nil
	}
	var trades []modelsdb.SimTrade
	err := c.Db.Where("session_id IN ?", sessionIDs).
		Order("session_id ASC, trade_date ASC").
		Find(&trades).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]modelsdb.SimTrade)
	for _, t := range trades {
		result[t.SessionID] = append(result[t.SessionID], t)
	}
	return result, nil
}
