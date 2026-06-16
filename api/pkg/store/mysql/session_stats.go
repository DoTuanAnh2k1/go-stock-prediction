package mysql

import (
	"fmt"
	"strings"
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// GetSessionDirAccuracy returns per-algorithm direction accuracy for predictions
// created in [from, to) for the given market. Only reconciled rows (direction_correct
// IS NOT NULL) are counted.
func (c *Client) GetSessionDirAccuracy(market string, from, to time.Time) ([]modelsapi.SessionDirAccRow, error) {
	info, ok := marketTableMap[strings.ToUpper(market)]
	if !ok {
		return nil, fmt.Errorf("unknown market: %q (valid: GOLD, NASDAQ, CRYPTO, SP500)", market)
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS algorithm,
			COUNT(*) AS total,
			SUM(CASE WHEN direction_correct = 1 THEN 1 ELSE 0 END) AS correct
		FROM %s
		WHERE direction_correct IS NOT NULL
		  AND deleted_at IS NULL
		  AND prediction_date >= ?
		  AND prediction_date < ?
		GROUP BY %s
		ORDER BY %s
	`, info.algoCol, info.table, info.algoCol, info.algoCol)

	type rawRow struct {
		Algorithm string
		Total     int64
		Correct   int64
	}
	var rows []rawRow
	if err := c.Db.Raw(query, from, to).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetSessionDirAccuracy(%s): %w", market, err)
	}

	result := make([]modelsapi.SessionDirAccRow, len(rows))
	for i, r := range rows {
		acc := 0.0
		if r.Total > 0 {
			acc = float64(r.Correct) / float64(r.Total)
		}
		result[i] = modelsapi.SessionDirAccRow{
			Algorithm: r.Algorithm,
			Total:     r.Total,
			Correct:   r.Correct,
			Accuracy:  acc,
		}
	}
	return result, nil
}

// GetSessionBotTrades returns per-algorithm bot trade stats for SELL trades
// closed in [from, to) for the given market.
func (c *Client) GetSessionBotTrades(market string, from, to time.Time) ([]modelsapi.SessionBotRow, error) {
	type rawRow struct {
		Algorithm string
		Trades    int64
		Wins      int64
		Losses    int64
		Breakeven int64
		TotalPnL  float64
	}
	var rows []rawRow

	query := `
		SELECT
			b.algorithm,
			COUNT(*) AS trades,
			SUM(CASE WHEN t.pnl > 0 THEN 1 ELSE 0 END) AS wins,
			SUM(CASE WHEN t.pnl < 0 THEN 1 ELSE 0 END) AS losses,
			SUM(CASE WHEN t.pnl = 0 THEN 1 ELSE 0 END) AS breakeven,
			COALESCE(SUM(t.pnl), 0) AS total_pnl
		FROM sim_trades t
		JOIN sim_bots b ON t.bot_id = b.id
		WHERE b.market = ?
		  AND t.action = 'SELL'
		  AND t.pnl IS NOT NULL
		  AND t.trade_date >= ?
		  AND t.trade_date < ?
		GROUP BY b.algorithm
		ORDER BY b.algorithm
	`
	if err := c.Db.Raw(query, strings.ToUpper(market), from, to).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetSessionBotTrades(%s): %w", market, err)
	}

	result := make([]modelsapi.SessionBotRow, len(rows))
	for i, r := range rows {
		avgPnL := 0.0
		winRate := 0.0
		if r.Trades > 0 {
			avgPnL = r.TotalPnL / float64(r.Trades)
			winRate = float64(r.Wins) / float64(r.Trades)
		}
		result[i] = modelsapi.SessionBotRow{
			Algorithm:      r.Algorithm,
			Trades:         r.Trades,
			Wins:           r.Wins,
			Losses:         r.Losses,
			Breakeven:      r.Breakeven,
			TotalPnL:       r.TotalPnL,
			AvgPnLPerTrade: avgPnL,
			WinRate:        winRate,
		}
	}
	return result, nil
}
