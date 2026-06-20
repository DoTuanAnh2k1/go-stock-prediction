package postgres

import (
	"fmt"
	"strings"
	"time"

	modelsapi "go-stock-prediction/pkg/models/models_api"
)

// marketMonitorMeta maps canonical market keys to their daily price table,
// intraday price table, and the timestamp column used for "last crawl at".
//
// All timestamps are stored in Asia/Ho_Chi_Minh wallclock (ICT-at-rest), and the
// Go PostgreSQL driver reads them with TimeZone=Asia/Ho_Chi_Minh, so time math here is
// straightforward against time.Now() (also ICT).
type marketMonitorMeta struct {
	dailyTable    string
	dailyTsCol    string
	intradayTable string
	intradayTsCol string
}

var monitorMeta = map[string]marketMonitorMeta{
	"GOLD":   {dailyTable: "gold_prices", dailyTsCol: "created_at", intradayTable: "gold_intraday_prices", intradayTsCol: "created_at"},
	"NASDAQ": {dailyTable: "nasdaq_prices", dailyTsCol: "created_at", intradayTable: "nasdaq_intraday_prices", intradayTsCol: "created_at"},
	"CRYPTO": {dailyTable: "crypto_prices", dailyTsCol: "created_at", intradayTable: "crypto_intraday_prices", intradayTsCol: "created_at"},
	"SP500":  {dailyTable: "sp500_prices", dailyTsCol: "created_at", intradayTable: "sp500_intraday_prices", intradayTsCol: "created_at"},
}

// marketPredMeta maps market keys to their prediction table name.
var marketPredMeta = map[string]string{
	"GOLD":   "gold_predictions",
	"NASDAQ": "nasdaq_predictions",
	"CRYPTO": "crypto_predictions",
	"SP500":  "sp500_predictions",
}

// GetMarketCrawlStats returns crawl freshness stats for the given market.
func (c *Client) GetMarketCrawlStats(market string) (*modelsapi.MarketCrawlStats, error) {
	meta, ok := monitorMeta[strings.ToUpper(market)]
	if !ok {
		return nil, fmt.Errorf("unknown market: %q (valid: GOLD, NASDAQ, CRYPTO, SP500)", market)
	}

	// Start-of-today in the server's local timezone (Asia/Ho_Chi_Minh).
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats := &modelsapi.MarketCrawlStats{}

	type tsRow struct {
		Ts time.Time
	}
	type cntRow struct {
		Cnt int64
	}

	// ── last daily crawl timestamp ──────────────────────────────────────────
	var dailyLast tsRow
	err := c.Db.Raw(
		fmt.Sprintf(`SELECT %s AS ts FROM %s WHERE deleted_at IS NULL ORDER BY %s DESC LIMIT 1`,
			meta.dailyTsCol, meta.dailyTable, meta.dailyTsCol),
	).Scan(&dailyLast).Error
	if err == nil && !dailyLast.Ts.IsZero() {
		t := dailyLast.Ts
		stats.LastDailyAt = &t
	}

	// ── daily count today ───────────────────────────────────────────────────
	var dailyCnt cntRow
	_ = c.Db.Raw(
		fmt.Sprintf(`SELECT COUNT(*) AS cnt FROM %s WHERE deleted_at IS NULL AND %s >= ?`,
			meta.dailyTable, meta.dailyTsCol),
		todayStart,
	).Scan(&dailyCnt).Error
	stats.DailyToday = dailyCnt.Cnt

	// ── last intraday crawl timestamp ───────────────────────────────────────
	var intradayLast tsRow
	err = c.Db.Raw(
		fmt.Sprintf(`SELECT %s AS ts FROM %s ORDER BY %s DESC LIMIT 1`,
			meta.intradayTsCol, meta.intradayTable, meta.intradayTsCol),
	).Scan(&intradayLast).Error
	if err == nil && !intradayLast.Ts.IsZero() {
		t := intradayLast.Ts
		stats.LastIntradayAt = &t
	}

	// ── intraday count today ────────────────────────────────────────────────
	var intradayCnt cntRow
	_ = c.Db.Raw(
		fmt.Sprintf(`SELECT COUNT(*) AS cnt FROM %s WHERE %s >= ?`,
			meta.intradayTable, meta.intradayTsCol),
		todayStart,
	).Scan(&intradayCnt).Error
	stats.IntradayToday = intradayCnt.Cnt

	return stats, nil
}

// GetMarketPredStats returns per-algorithm prediction counts for today for the given market.
func (c *Client) GetMarketPredStats(market string) ([]modelsapi.AlgoPredStats, error) {
	table, ok := marketPredMeta[strings.ToUpper(market)]
	if !ok {
		return nil, fmt.Errorf("unknown market: %q (valid: GOLD, NASDAQ, CRYPTO, SP500)", market)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	type rawRow struct {
		AlgorithmName string
		TodayCount    int64
		LastPredictAt *time.Time
	}

	// Group by algorithm, count today's rows, pick max created_at as last_predict_at.
	query := fmt.Sprintf(`
		SELECT
			algorithm_name,
			SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END) AS today_count,
			MAX(created_at) AS last_predict_at
		FROM %s
		WHERE deleted_at IS NULL
		GROUP BY algorithm_name
		ORDER BY algorithm_name
	`, table)

	var rows []rawRow
	if err := c.Db.Raw(query, todayStart).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("GetMarketPredStats(%s): %w", market, err)
	}

	result := make([]modelsapi.AlgoPredStats, len(rows))
	for i, r := range rows {
		result[i] = modelsapi.AlgoPredStats{
			AlgorithmName: r.AlgorithmName,
			TodayCount:    r.TodayCount,
			LastPredictAt: r.LastPredictAt,
		}
	}
	return result, nil
}
