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

// SessionBotRow — bot trading stats của 1 thuật toán trong phiên.
type SessionBotRow struct {
	Algorithm      string  `json:"algorithm"`
	Trades         int64   `json:"trades"`
	Wins           int64   `json:"wins"`
	Losses         int64   `json:"losses"`
	Breakeven      int64   `json:"breakeven"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnLPerTrade float64 `json:"avg_pnl_per_trade"`
	WinRate        float64 `json:"win_rate"`
}

// SessionStatsResponse — full response của GET /api/markets/{key}/session-stats.
type SessionStatsResponse struct {
	Market            string             `json:"market"`
	Session           SessionWindow      `json:"session"`
	DirectionAccuracy []SessionDirAccRow `json:"direction_accuracy"`
	BotTrades         []SessionBotRow    `json:"bot_trades"`
}
