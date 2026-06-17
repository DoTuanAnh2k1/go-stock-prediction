package server

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/service/predict/registry"
	"go-stock-prediction/pkg/store/repository"
	market_calendar "go-stock-prediction/pkg/utils/market_calendar"
)

// ─── Response DTOs ────────────────────────────────────────────────────────────

type monitoringCrawl struct {
	LastCrawlAt   *string `json:"last_crawl_at"`  // RFC3339 or null
	Staleness     string  `json:"staleness"`       // "35m ago" | "2h ago" | "never"
	Stale         bool    `json:"stale"`
	MarketOpen    bool    `json:"market_open"`     // true if market is currently in a trading session (ET-based)
	DailyToday    int64   `json:"daily_today"`
	IntradayToday int64   `json:"intraday_today"`
}

type monitoringAlgoStat struct {
	Algorithm         string  `json:"algorithm"`
	TodayCount        int64   `json:"today_count"`
	DirectionAccuracy float64 `json:"direction_accuracy"` // 0..1
	Reconciled        int64   `json:"reconciled"`
	Correct           int64   `json:"correct"`
}

type monitoringPredictions struct {
	LastPredictAt *string              `json:"last_predict_at"` // RFC3339 or null
	Staleness     string               `json:"staleness"`
	TodayTotal    int64                `json:"today_total"`
	ExpectedAlgos int                  `json:"expected_algos"`
	MissingToday  []string             `json:"missing_today"`
	Algorithms    []monitoringAlgoStat `json:"algorithms"`
}

type monitoringMarket struct {
	Market      string                `json:"market"`
	Crawl       monitoringCrawl       `json:"crawl"`
	Predictions monitoringPredictions `json:"predictions"`
}

type monitoringBotByMarket struct {
	Market   string  `json:"market"`
	Trades   int     `json:"trades"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	WinRate  float64 `json:"win_rate"`  // 0..1
	TotalPnl float64 `json:"total_pnl"`
}

type monitoringBotTableRow struct {
	BotID        string  `json:"bot_id"`
	Market       string  `json:"market"`
	Algorithm    string  `json:"algorithm"`
	Trades       int     `json:"trades"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	Breakeven    int     `json:"breakeven"`
	WinRate      float64 `json:"win_rate"`      // 0..1
	TotalPnl     float64 `json:"total_pnl"`
	ReturnPct    float64 `json:"return_pct"`
	ProfitFactor float64 `json:"profit_factor"`
}

type monitoringBotSummary struct {
	TotalBots  int                     `json:"total_bots"`
	ActiveBots int                     `json:"active_bots"`
	ByMarket   []monitoringBotByMarket `json:"by_market"`
}

type monitoringBots struct {
	Summary monitoringBotSummary    `json:"summary"`
	Table   []monitoringBotTableRow `json:"table"`
}

type monitoringOverviewResponse struct {
	GeneratedAt string             `json:"generated_at"` // RFC3339
	Markets     []monitoringMarket `json:"markets"`
	Bots        monitoringBots     `json:"bots"`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// formatStaleness returns a human-readable staleness string and whether the
// market is considered stale (last activity > 3h ago, or never, or nothing today).
// Timestamps are ICT (loc=Asia/Ho_Chi_Minh) so time.Since against time.Now() is correct.
func formatStaleness(lastAt *time.Time, hasToday bool) (string, bool) {
	if lastAt == nil || lastAt.IsZero() {
		return "never", true
	}
	diff := time.Since(*lastAt)
	if diff < 0 {
		diff = 0 // guard against minor clock skew
	}
	stale := diff > 3*time.Hour || !hasToday

	var humanStr string
	switch {
	case diff < time.Minute:
		humanStr = "just now"
	case diff < time.Hour:
		humanStr = fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		humanStr = fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		humanStr = fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
	return humanStr, stale
}

// rfc3339OrNil returns a pointer to an RFC3339-formatted time string, or nil.
func rfc3339OrNil(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// expectedAlgoKeys returns the set of algo keys from the registry (source of truth).
func expectedAlgoKeys() []string {
	defs := registry.All()
	keys := make([]string, 0, len(defs))
	for _, d := range defs {
		keys = append(keys, d.Key)
	}
	return keys
}

// buildMarketOverview constructs the monitoringMarket entry for one market key.
func buildMarketOverview(
	market string,
	store repository.DatabaseStore,
) monitoringMarket {
	// ── Crawl section ──────────────────────────────────────────────────────
	crawlSection := monitoringCrawl{}

	crawlStats, err := store.GetMarketCrawlStats(market)
	if err != nil {
		logger.Logger.Errorf("GetMarketCrawlStats(%s): %v", market, err)
	} else {
		// Most recent activity across daily and intraday is the canonical "last crawl".
		var lastCrawl *time.Time
		if crawlStats.LastDailyAt != nil {
			lastCrawl = crawlStats.LastDailyAt
		}
		if crawlStats.LastIntradayAt != nil {
			if lastCrawl == nil || crawlStats.LastIntradayAt.After(*lastCrawl) {
				lastCrawl = crawlStats.LastIntradayAt
			}
		}

		hasToday := (crawlStats.DailyToday + crawlStats.IntradayToday) > 0
		staleness, stale := formatStaleness(lastCrawl, hasToday)

		crawlSection = monitoringCrawl{
			LastCrawlAt:   rfc3339OrNil(lastCrawl),
			Staleness:     staleness,
			Stale:         stale,
			MarketOpen:    market_calendar.IsMarketOpenNow(market),
			DailyToday:    crawlStats.DailyToday,
			IntradayToday: crawlStats.IntradayToday,
		}
	}

	// ── Prediction section ─────────────────────────────────────────────────
	expectedKeys := expectedAlgoKeys()

	// Build a map from algo key → AlgoPredStats for quick lookup.
	predStatsMap := make(map[string]modelsapi.AlgoPredStats)
	allPredStats, err := store.GetMarketPredStats(market)
	if err != nil {
		logger.Logger.Errorf("GetMarketPredStats(%s): %v", market, err)
	} else {
		for _, ps := range allPredStats {
			predStatsMap[ps.AlgorithmName] = ps
		}
	}

	// Direction accuracy per algo.
	dirAccRows, err := store.GetDirectionAccuracy(market)
	if err != nil {
		logger.Logger.Errorf("GetDirectionAccuracy(%s): %v", market, err)
	}
	dirAccMap := make(map[string]struct{ total, correct int64 })
	for _, row := range dirAccRows {
		dirAccMap[row.Algorithm] = struct{ total, correct int64 }{row.Total, row.Correct}
	}

	// Build algorithm rows and derive aggregates.
	algoRows := make([]monitoringAlgoStat, 0, len(expectedKeys))
	var lastPredictAt *time.Time
	var totalToday int64
	missingToday := []string{}

	for _, key := range expectedKeys {
		ps := predStatsMap[key] // zero value if not present (TodayCount=0)
		da := dirAccMap[key]

		dirAcc := 0.0
		if da.total > 0 {
			dirAcc = float64(da.correct) / float64(da.total)
		}

		// Track most recent prediction across all algos.
		if ps.LastPredictAt != nil {
			if lastPredictAt == nil || ps.LastPredictAt.After(*lastPredictAt) {
				t := *ps.LastPredictAt
				lastPredictAt = &t
			}
		}

		totalToday += ps.TodayCount

		if ps.TodayCount == 0 {
			missingToday = append(missingToday, key)
		}

		algoRows = append(algoRows, monitoringAlgoStat{
			Algorithm:         key,
			TodayCount:        ps.TodayCount,
			DirectionAccuracy: dirAcc,
			Reconciled:        da.total,
			Correct:           da.correct,
		})
	}

	staleness, _ := formatStaleness(lastPredictAt, totalToday > 0)

	predSection := monitoringPredictions{
		LastPredictAt: rfc3339OrNil(lastPredictAt),
		Staleness:     staleness,
		TodayTotal:    totalToday,
		ExpectedAlgos: len(expectedKeys),
		MissingToday:  missingToday,
		Algorithms:    algoRows,
	}

	return monitoringMarket{
		Market:      market,
		Crawl:       crawlSection,
		Predictions: predSection,
	}
}

// buildBotSection builds the bots monitoring section using batch queries (N+1 → 4 queries).
func buildBotSection(store repository.DatabaseStore) monitoringBots {
	bots, err := store.GetAllSimBots()
	if err != nil {
		logger.Logger.Errorf("monitoring: GetAllSimBots: %v", err)
		return monitoringBots{
			Summary: monitoringBotSummary{ByMarket: []monitoringBotByMarket{}},
			Table:   []monitoringBotTableRow{},
		}
	}

	// -- 1. Get all sessions with snap counts (1 query) --
	allSessions, _ := store.GetAllSessionsWithSnapCount()
	sessionsByBot := make(map[string][]modelsdb.SimSessionWithCount)
	for _, s := range allSessions {
		sessionsByBot[s.BotID] = append(sessionsByBot[s.BotID], s)
	}
	chosenSessions := make(map[string]*modelsdb.SimSession)
	for _, bot := range bots {
		chosenSessions[bot.ID] = pickBestSession(sessionsByBot[bot.ID])
	}

	sessionIDs := make([]int64, 0, len(chosenSessions))
	for _, sess := range chosenSessions {
		if sess != nil {
			sessionIDs = append(sessionIDs, sess.ID)
		}
	}

	// -- 2. Batch trade stats + last snapshots (2 queries) --
	tradeStats, _ := store.GetSessionTradeStatsBatch(sessionIDs)
	lastSnaps, _ := store.GetLastSnapshotsBatch(sessionIDs)

	// -- 3. Build table rows --
	tableRows := make([]monitoringBotTableRow, 0, len(bots))
	byMarketMap := make(map[string]*monitoringBotByMarket)
	totalBots := len(bots)
	activeBots := 0

	for _, bot := range bots {
		if bot.IsActive {
			activeBots++
		}

		sess := chosenSessions[bot.ID]
		var stats modelsdb.SimTradeStats
		var returnPct float64

		if sess != nil {
			stats = tradeStats[sess.ID]
			if snap, ok := lastSnaps[sess.ID]; ok && snap != nil && snap.TotalReturnPct != nil {
				returnPct, _ = snap.TotalReturnPct.Float64()
			}
		}

		denominator := stats.Wins + stats.Losses
		winRate := 0.0
		if denominator > 0 {
			winRate = float64(stats.Wins) / float64(denominator)
		}
		profitFactor := 0.0
		if stats.LossPnl > 0 {
			profitFactor = stats.WinPnl / stats.LossPnl
		}

		tableRows = append(tableRows, monitoringBotTableRow{
			BotID:        bot.ID,
			Market:       bot.Market,
			Algorithm:    bot.Algorithm,
			Trades:       stats.TotalTrades,
			Wins:         stats.Wins,
			Losses:       stats.Losses,
			Breakeven:    stats.Breakeven,
			WinRate:      winRate,
			TotalPnl:     stats.TotalPnl,
			ReturnPct:    returnPct,
			ProfitFactor: profitFactor,
		})

		bm, exists := byMarketMap[bot.Market]
		if !exists {
			bm = &monitoringBotByMarket{Market: bot.Market}
			byMarketMap[bot.Market] = bm
		}
		bm.Trades += stats.TotalTrades
		bm.Wins += stats.Wins
		bm.Losses += stats.Losses
		bm.TotalPnl += stats.TotalPnl
	}

	sort.Slice(tableRows, func(i, j int) bool {
		if tableRows[i].WinRate != tableRows[j].WinRate {
			return tableRows[i].WinRate > tableRows[j].WinRate
		}
		return tableRows[i].TotalPnl > tableRows[j].TotalPnl
	})

	byMarketSlice := make([]monitoringBotByMarket, 0, len(byMarketMap))
	for _, bm := range byMarketMap {
		denom := bm.Wins + bm.Losses
		if denom > 0 {
			bm.WinRate = float64(bm.Wins) / float64(denom)
		}
		byMarketSlice = append(byMarketSlice, *bm)
	}
	sort.Slice(byMarketSlice, func(i, j int) bool {
		return byMarketSlice[i].Market < byMarketSlice[j].Market
	})

	return monitoringBots{
		Summary: monitoringBotSummary{
			TotalBots:  totalBots,
			ActiveBots: activeBots,
			ByMarket:   byMarketSlice,
		},
		Table: tableRows,
	}
}

// ─── Handler ─────────────────────────────────────────────────────────────────

// GetMonitoringOverview godoc
//
//	@Summary      Get monitoring overview
//	@Description  Returns a single aggregated view of crawl freshness, prediction activity per algorithm per market, and bot trading win/loss stats. Response is cached for 30 seconds. Requires JWT authentication.
//	@Tags         Monitoring
//	@Produce      json
//	@Security     BearerAuth
//	@Success      200  {object}  monitoringOverviewResponse
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/monitoring/overview [get]
func GetMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	if !requireAuth(w, r) {
		return
	}

	const cacheKey = "monitoring:overview"
	if cached, ok := globalCache.Get(cacheKey); ok {
		ResponseSuccess(w, http.StatusOK, cached)
		return
	}

	store := repository.GetSingleton()

	markets := []string{"GOLD", "NASDAQ", "CRYPTO", "SP500"}
	marketResults := make([]monitoringMarket, 0, len(markets))
	for _, mkt := range markets {
		marketResults = append(marketResults, buildMarketOverview(mkt, store))
	}

	botsSection := buildBotSection(store)

	result := &monitoringOverviewResponse{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Markets:     marketResults,
		Bots:        botsSection,
	}

	globalCache.Set(cacheKey, result, 30*time.Second)
	ResponseSuccess(w, http.StatusOK, result)
}
