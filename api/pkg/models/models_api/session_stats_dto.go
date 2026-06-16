package modelsapi

import "time"

// SessionWindow mô tả phiên giao dịch được truy vấn.
type SessionWindow struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	IsOpen bool      `json:"is_open"`
}

// SessionDirAccRow — direction accuracy của 1 thuật toán trong phiên.
type SessionDirAccRow struct {
	Algorithm string  `json:"algorithm"`
	Total     int64   `json:"total"`
	Correct   int64   `json:"correct"`
	Accuracy  float64 `json:"accuracy"`
}

// SessionBotDetail — per-bot trading stats với portfolio snapshot data trong phiên.
type SessionBotDetail struct {
	BotID          string  `json:"bot_id"`
	DisplayName    string  `json:"display_name"`
	Algorithm      string  `json:"algorithm"`
	Currency       string  `json:"currency"`
	InitialCapital float64 `json:"initial_capital"`
	CurrentValue   float64 `json:"current_value"`
	SessionPnL     float64 `json:"session_pnl"`
	TotalReturnPct float64 `json:"total_return_pct"`
	Trades         int64   `json:"trades"`
	Wins           int64   `json:"wins"`
	Losses         int64   `json:"losses"`
	Breakeven      int64   `json:"breakeven"`
	WinRate        float64 `json:"win_rate"`
}

// SessionStatsResponse — full response của GET /api/markets/{key}/session-stats.
type SessionStatsResponse struct {
	Market            string             `json:"market"`
	Session           SessionWindow      `json:"session"`
	DirectionAccuracy []SessionDirAccRow `json:"direction_accuracy"`
	BotTrades         []SessionBotDetail `json:"bot_trades"`
}
