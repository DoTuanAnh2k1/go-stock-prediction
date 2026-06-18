package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
)

// SimBot represents a trading bot configuration (market × algorithm pair).
type SimBot struct {
	ID             string          `gorm:"primaryKey;size:50" json:"id"`
	Market         string          `gorm:"size:20;not null;index" json:"market"`
	Algorithm      string          `gorm:"size:50;not null" json:"algorithm"`
	DisplayName    string          `gorm:"size:100;not null" json:"display_name"`
	InitialCapital decimal.Decimal `gorm:"type:decimal(20,2);not null" json:"initial_capital"`
	Currency       string          `gorm:"size:5;not null" json:"currency"`
	BuyThreshold   decimal.Decimal `gorm:"type:decimal(5,2);default:1.50" json:"buy_threshold"`
	SellThreshold  decimal.Decimal `gorm:"type:decimal(5,2);default:1.00" json:"sell_threshold"`
	MinConfidence  decimal.Decimal `gorm:"type:decimal(4,2);default:0.60" json:"min_confidence"`
	StopLoss       decimal.Decimal `gorm:"type:decimal(5,2);default:5.00" json:"stop_loss"`
	TakeProfit     decimal.Decimal `gorm:"type:decimal(5,2);default:8.00" json:"take_profit"`
	MaxPositionPct decimal.Decimal `gorm:"type:decimal(5,2);default:15.00" json:"max_position_pct"`
	MaxPositions   int             `gorm:"default:5" json:"max_positions"`
	IsActive       bool            `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func (SimBot) TableName() string { return "sim_bots" }

// SimSession represents a backtest or live simulation run for a bot.
type SimSession struct {
	ID             int64            `gorm:"primaryKey;autoIncrement" json:"id"`
	BotID          string           `gorm:"size:50;not null;index" json:"bot_id"`
	StartDate      time.Time        `gorm:"not null" json:"start_date"`
	EndDate        *time.Time       `json:"end_date,omitempty"`
	Status         string           `gorm:"size:20;default:'running'" json:"status"` // running, completed, paused
	Mode           string           `gorm:"size:20;default:'backtest'" json:"mode"`  // backtest, live
	CreatedAt      time.Time        `json:"created_at"`
	// Pre-computed KPIs — set by Python after session end
	TotalTrades    int              `gorm:"default:0" json:"total_trades"`
	Wins           int              `gorm:"default:0" json:"wins"`
	Losses         int              `gorm:"default:0" json:"losses"`
	Breakeven      int              `gorm:"default:0" json:"breakeven"`
	TotalPnl       decimal.Decimal  `gorm:"type:decimal(20,2);default:0" json:"total_pnl"`
	TotalReturnPct *decimal.Decimal `gorm:"type:decimal(8,4)" json:"total_return_pct,omitempty"`
	WinRate        *decimal.Decimal `gorm:"type:decimal(5,4)" json:"win_rate,omitempty"`
	ProfitFactor   *decimal.Decimal `gorm:"type:decimal(8,4)" json:"profit_factor,omitempty"`
	MaxDrawdownPct *decimal.Decimal `gorm:"type:decimal(8,4)" json:"max_drawdown_pct,omitempty"`
	KpiUpdatedAt   *time.Time       `json:"kpi_updated_at,omitempty"`
}

func (SimSession) TableName() string { return "sim_sessions" }

// SimTrade records a single BUY or SELL trade in a simulation.
type SimTrade struct {
	ID             int64            `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID      int64            `gorm:"not null;index" json:"session_id"`
	BotID          string           `gorm:"size:50;not null;index" json:"bot_id"`
	Symbol         string           `gorm:"size:20;not null" json:"symbol"`
	Action         string           `gorm:"size:5;not null" json:"action"` // BUY, SELL
	Quantity       decimal.Decimal  `gorm:"type:decimal(20,6);not null" json:"quantity"`
	Price          decimal.Decimal  `gorm:"type:decimal(20,4);not null" json:"price"`
	TradeValue     decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"trade_value"`
	SignalStrength *decimal.Decimal `gorm:"type:decimal(8,4)" json:"signal_strength,omitempty"`
	Confidence     *decimal.Decimal `gorm:"type:decimal(4,3)" json:"confidence,omitempty"`
	TradeDate      time.Time        `gorm:"not null;index" json:"trade_date"`
	CloseReason    string           `gorm:"size:20" json:"close_reason,omitempty"` // signal, stop_loss, take_profit
	EntryTradeID   *int64           `json:"entry_trade_id,omitempty"`
	PnL            *decimal.Decimal `gorm:"type:decimal(20,2);column:pnl" json:"pnl,omitempty"`
	PnLPct         *decimal.Decimal `gorm:"type:decimal(8,4);column:pnl_pct" json:"pnl_pct,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

func (SimTrade) TableName() string { return "sim_trades" }

// SimPortfolioSnapshot is a daily snapshot of portfolio state.
type SimPortfolioSnapshot struct {
	ID             int64            `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID      int64            `gorm:"not null;index:idx_sim_snap_session_date,priority:1" json:"session_id"`
	BotID          string           `gorm:"size:50;not null;index:idx_sim_snap_bot_date,priority:1" json:"bot_id"`
	SnapshotDate   time.Time        `gorm:"not null;index:idx_sim_snap_session_date,priority:2;index:idx_sim_snap_bot_date,priority:2" json:"snapshot_date"`
	CashBalance    decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"cash_balance"`
	PositionsValue decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"positions_value"`
	TotalValue     decimal.Decimal  `gorm:"type:decimal(20,2);not null" json:"total_value"`
	TotalReturnPct *decimal.Decimal `gorm:"type:decimal(8,4)" json:"total_return_pct,omitempty"`
	OpenPositions  int              `gorm:"default:0" json:"open_positions"`
}

func (SimPortfolioSnapshot) TableName() string { return "sim_portfolio_snapshots" }

// SimSessionWithCount extends SimSession with the number of portfolio snapshots.
// Used by batch leaderboard/monitoring queries.
type SimSessionWithCount struct {
	SimSession
	SnapCount int
}

// SimTradeStats holds pre-aggregated trade metrics from SQL for a session.
type SimTradeStats struct {
	TotalTrades int
	Wins        int
	Losses      int
	Breakeven   int
	TotalPnl    float64
	WinPnl      float64
	LossPnl     float64
}

// LeaderboardEntry is the result of a single JOIN query for the leaderboard.
type LeaderboardEntry struct {
	SessionID       int64            `gorm:"column:session_id"`
	BotID           string           `gorm:"column:bot_id"`
	Market          string           `gorm:"column:market"`
	Algorithm       string           `gorm:"column:algorithm"`
	DisplayName     string           `gorm:"column:display_name"`
	InitialCapital  decimal.Decimal  `gorm:"column:initial_capital"`
	Currency        string           `gorm:"column:currency"`
	StartDate       time.Time        `gorm:"column:start_date"`
	EndDate         *time.Time       `gorm:"column:end_date"`
	Mode            string           `gorm:"column:mode"`
	Status          string           `gorm:"column:status"`
	TotalTrades     int              `gorm:"column:total_trades"`
	Wins            int              `gorm:"column:wins"`
	Losses          int              `gorm:"column:losses"`
	Breakeven       int              `gorm:"column:breakeven"`
	TotalPnl        decimal.Decimal  `gorm:"column:total_pnl"`
	TotalReturnPct  *decimal.Decimal `gorm:"column:total_return_pct"`
	WinRate         *decimal.Decimal `gorm:"column:win_rate"`
	ProfitFactor    *decimal.Decimal `gorm:"column:profit_factor"`
	MaxDrawdownPct  *decimal.Decimal `gorm:"column:max_drawdown_pct"`
	CurrentValue    *decimal.Decimal `gorm:"column:current_value"`
}
