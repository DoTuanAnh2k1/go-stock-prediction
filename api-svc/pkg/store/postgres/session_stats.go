package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// GetSessionDirAccuracy returns per-algorithm direction accuracy for predictions
// created in [from, to) for the given market. Only reconciled rows (direction_correct
// IS NOT NULL) are counted.
func (c *Client) GetSessionDirAccuracy(ctx context.Context, market string, from, to time.Time) ([]modelsapi.SessionDirAccRow, error) {
	info, ok := marketTableMap[strings.ToUpper(market)]
	if !ok {
		return nil, fmt.Errorf("unknown market: %q (valid: GOLD, NASDAQ, CRYPTO, SP500)", market)
	}

	// PostgreSQL uses FILTER syntax for conditional aggregation instead of CASE WHEN.
	query := fmt.Sprintf(`
		SELECT
			%s AS algorithm,
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE direction_correct = true) AS correct
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
	if err := c.db(ctx).Raw(query, from, to).Scan(&rows).Error; err != nil {
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

// GetSessionBotTrades returns per-bot trade stats with portfolio snapshot data for SELL trades
// closed in [from, to) for the given market. Results are ordered by session_pnl DESC.
func (c *Client) GetSessionBotTrades(ctx context.Context, market string, from, to time.Time) ([]modelsapi.SessionBotDetail, error) {
	type rawRow struct {
		BotID          string
		DisplayName    string
		Algorithm      string
		Currency       string
		InitialCapital float64
		CurrentValue   float64
		TotalReturnPct float64
		Trades         int64
		Wins           int64
		Losses         int64
		Breakeven      int64
		SessionPnL     float64
	}
	var rows []rawRow

	query := `
		SELECT
			b.id                                                     AS bot_id,
			b.display_name,
			b.algorithm,
			b.currency,
			CAST(b.initial_capital AS DECIMAL(20,2))                AS initial_capital,
			COALESCE(CAST(latest.total_value AS DECIMAL(20,2)),
			         CAST(b.initial_capital AS DECIMAL(20,2)))      AS current_value,
			COALESCE(latest.total_return_pct, 0)                    AS total_return_pct,
			COUNT(t.id)                                             AS trades,
			SUM(CASE WHEN t.pnl > 0 THEN 1 ELSE 0 END)             AS wins,
			SUM(CASE WHEN t.pnl < 0 THEN 1 ELSE 0 END)             AS losses,
			SUM(CASE WHEN t.pnl = 0 THEN 1 ELSE 0 END)             AS breakeven,
			COALESCE(SUM(t.pnl), 0)                                 AS session_pnl
		FROM sim_bots b
		LEFT JOIN (
			SELECT s1.bot_id,
			       s1.total_value,
			       s1.total_return_pct
			FROM sim_portfolio_snapshots s1
			WHERE s1.snapshot_date = (
				SELECT MAX(s2.snapshot_date)
				FROM sim_portfolio_snapshots s2
				WHERE s2.bot_id = s1.bot_id
			)
		) latest ON latest.bot_id = b.id
		LEFT JOIN sim_trades t ON t.bot_id = b.id
			AND t.action = 'SELL'
			AND t.pnl IS NOT NULL
			AND t.trade_date >= ?
			AND t.trade_date < ?
		WHERE b.market = ?
		GROUP BY b.id, b.display_name, b.algorithm, b.currency,
		         b.initial_capital, latest.total_value, latest.total_return_pct
		ORDER BY session_pnl DESC
	`

	if err := c.db(ctx).Raw(query, from, to, strings.ToUpper(market)).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetSessionBotTrades(%s): %w", market, err)
	}

	result := make([]modelsapi.SessionBotDetail, len(rows))
	for i, r := range rows {
		winRate := 0.0
		if r.Trades > 0 {
			winRate = float64(r.Wins) / float64(r.Trades)
		}
		result[i] = modelsapi.SessionBotDetail{
			BotID:          r.BotID,
			DisplayName:    r.DisplayName,
			Algorithm:      r.Algorithm,
			Currency:       r.Currency,
			InitialCapital: r.InitialCapital,
			CurrentValue:   r.CurrentValue,
			SessionPnL:     r.SessionPnL,
			TotalReturnPct: r.TotalReturnPct,
			Trades:         r.Trades,
			Wins:           r.Wins,
			Losses:         r.Losses,
			Breakeven:      r.Breakeven,
			WinRate:        winRate,
		}
	}
	return result, nil
}
