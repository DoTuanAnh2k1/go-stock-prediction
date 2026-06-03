package server

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
	pb "go-stock-prediction/proto/prediction"
)

// ─── KPI types ───────────────────────────────────────────────────────────────

type simBotKPIs struct {
	TotalReturnPct       float64 `json:"total_return_pct"`
	AnnualizedReturnPct  float64 `json:"annualized_return_pct"`
	SharpeRatio          float64 `json:"sharpe_ratio"`
	MaxDrawdownPct       float64 `json:"max_drawdown_pct"`
	WinRatePct           float64 `json:"win_rate_pct"`
	ProfitFactor         float64 `json:"profit_factor"`
	TotalTrades          int     `json:"total_trades"`
	AvgTradeDurationDays float64 `json:"avg_trade_duration_days"`
	BestTradePct         float64 `json:"best_trade_pct"`
	WorstTradePct        float64 `json:"worst_trade_pct"`
}

// computeKPIs derives KPI metrics from snapshots and closed (SELL) trades.
func computeKPIs(snaps []modelsdb.SimPortfolioSnapshot, trades []modelsdb.SimTrade) simBotKPIs {
	kpis := simBotKPIs{}
	if len(snaps) == 0 {
		return kpis
	}

	// ── total_return_pct ──────────────────────────────────────────────────
	last := snaps[len(snaps)-1]
	if last.TotalReturnPct != nil {
		v, _ := last.TotalReturnPct.Float64()
		kpis.TotalReturnPct = v
	}

	// ── annualized_return_pct ─────────────────────────────────────────────
	first := snaps[0]
	days := last.SnapshotDate.Sub(first.SnapshotDate).Hours() / 24
	if days > 0 {
		kpis.AnnualizedReturnPct = (math.Pow(1+kpis.TotalReturnPct/100, 365.0/days) - 1) * 100
	}

	// ── sharpe_ratio ──────────────────────────────────────────────────────
	if len(snaps) >= 2 {
		dailyReturns := make([]float64, 0, len(snaps)-1)
		for i := 1; i < len(snaps); i++ {
			prev, _ := snaps[i-1].TotalValue.Float64()
			curr, _ := snaps[i].TotalValue.Float64()
			if prev > 0 {
				dailyReturns = append(dailyReturns, (curr-prev)/prev)
			}
		}
		if len(dailyReturns) > 1 {
			mean := 0.0
			for _, r := range dailyReturns {
				mean += r
			}
			mean /= float64(len(dailyReturns))
			variance := 0.0
			for _, r := range dailyReturns {
				variance += math.Pow(r-mean, 2)
			}
			variance /= float64(len(dailyReturns) - 1)
			stddev := math.Sqrt(variance)
			if stddev > 0 {
				kpis.SharpeRatio = mean / stddev * math.Sqrt(252)
			}
		}
	}

	// ── max_drawdown_pct ──────────────────────────────────────────────────
	peak := 0.0
	for _, s := range snaps {
		v, _ := s.TotalValue.Float64()
		if v > peak {
			peak = v
		}
		if peak > 0 {
			dd := (peak - v) / peak * 100
			if dd > kpis.MaxDrawdownPct {
				kpis.MaxDrawdownPct = dd
			}
		}
	}
	kpis.MaxDrawdownPct = -kpis.MaxDrawdownPct // negative by convention

	// ── trade metrics from SELL trades ───────────────────────────────────
	var sellTrades []modelsdb.SimTrade
	for _, t := range trades {
		if t.Action == "SELL" {
			sellTrades = append(sellTrades, t)
		}
	}
	kpis.TotalTrades = len(sellTrades)

	if len(sellTrades) == 0 {
		return kpis
	}

	wins := 0
	totalProfit := 0.0
	totalLoss := 0.0
	bestPct := math.Inf(-1)
	worstPct := math.Inf(1)

	for _, t := range sellTrades {
		pnl := 0.0
		if t.PnL != nil {
			pnl, _ = t.PnL.Float64()
		}
		pnlPct := 0.0
		if t.PnLPct != nil {
			pnlPct, _ = t.PnLPct.Float64()
		}
		if pnl > 0 {
			wins++
			totalProfit += pnl
		} else if pnl < 0 {
			totalLoss += math.Abs(pnl)
		}
		if pnlPct > bestPct {
			bestPct = pnlPct
		}
		if pnlPct < worstPct {
			worstPct = pnlPct
		}
	}

	kpis.WinRatePct = float64(wins) / float64(len(sellTrades)) * 100
	if totalLoss > 0 {
		kpis.ProfitFactor = totalProfit / totalLoss
	}
	if !math.IsInf(bestPct, -1) {
		kpis.BestTradePct = bestPct
	}
	if !math.IsInf(worstPct, 1) {
		kpis.WorstTradePct = worstPct
	}

	// ── avg trade duration ────────────────────────────────────────────────
	// Build a map of buyID → tradeDate for BUY trades.
	buyDates := make(map[int64]time.Time)
	for _, t := range trades {
		if t.Action == "BUY" {
			buyDates[t.ID] = t.TradeDate
		}
	}
	totalDuration := 0.0
	counted := 0
	for _, t := range sellTrades {
		if t.EntryTradeID != nil {
			if buyDate, ok := buyDates[*t.EntryTradeID]; ok {
				d := t.TradeDate.Sub(buyDate).Hours() / 24
				totalDuration += d
				counted++
			}
		}
	}
	if counted > 0 {
		kpis.AvgTradeDurationDays = totalDuration / float64(counted)
	}

	return kpis
}

// ─── JSON-safe DTOs (decimal.Decimal → float64) ──────────────────────────────

// simBotJSON mirrors SimBot with all decimal fields as float64 so they
// serialize to JSON numbers instead of strings.
type simBotJSON struct {
	ID             string    `json:"id"`
	Market         string    `json:"market"`
	Algorithm      string    `json:"algorithm"`
	DisplayName    string    `json:"display_name"`
	InitialCapital float64   `json:"initial_capital"`
	Currency       string    `json:"currency"`
	BuyThreshold   float64   `json:"buy_threshold"`
	SellThreshold  float64   `json:"sell_threshold"`
	MinConfidence  float64   `json:"min_confidence"`
	StopLoss       float64   `json:"stop_loss"`
	TakeProfit     float64   `json:"take_profit"`
	MaxPositionPct float64   `json:"max_position_pct"`
	MaxPositions   int       `json:"max_positions"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func botToJSON(b modelsdb.SimBot) simBotJSON {
	ic, _ := b.InitialCapital.Float64()
	bt, _ := b.BuyThreshold.Float64()
	st, _ := b.SellThreshold.Float64()
	mc, _ := b.MinConfidence.Float64()
	sl, _ := b.StopLoss.Float64()
	tp, _ := b.TakeProfit.Float64()
	mp, _ := b.MaxPositionPct.Float64()
	return simBotJSON{
		ID:             b.ID,
		Market:         b.Market,
		Algorithm:      b.Algorithm,
		DisplayName:    b.DisplayName,
		InitialCapital: ic,
		Currency:       b.Currency,
		BuyThreshold:   bt,
		SellThreshold:  st,
		MinConfidence:  mc,
		StopLoss:       sl,
		TakeProfit:     tp,
		MaxPositionPct: mp,
		MaxPositions:   b.MaxPositions,
		IsActive:       b.IsActive,
		CreatedAt:      b.CreatedAt,
		UpdatedAt:      b.UpdatedAt,
	}
}

// simTradeJSON mirrors SimTrade with all decimal fields as float64.
type simTradeJSON struct {
	ID             int64     `json:"id"`
	SessionID      int64     `json:"session_id"`
	BotID          string    `json:"bot_id"`
	Symbol         string    `json:"symbol"`
	Action         string    `json:"action"`
	Quantity       float64   `json:"quantity"`
	Price          float64   `json:"price"`
	TradeValue     float64   `json:"trade_value"`
	SignalStrength *float64  `json:"signal_strength,omitempty"`
	Confidence     *float64  `json:"confidence,omitempty"`
	TradeDate      time.Time `json:"trade_date"`
	CloseReason    string    `json:"close_reason,omitempty"`
	EntryTradeID   *int64    `json:"entry_trade_id,omitempty"`
	PnL            *float64  `json:"pnl,omitempty"`
	PnLPct         *float64  `json:"pnl_pct,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func tradeToJSON(t modelsdb.SimTrade) simTradeJSON {
	q, _ := t.Quantity.Float64()
	p, _ := t.Price.Float64()
	tv, _ := t.TradeValue.Float64()
	j := simTradeJSON{
		ID:           t.ID,
		SessionID:    t.SessionID,
		BotID:        t.BotID,
		Symbol:       t.Symbol,
		Action:       t.Action,
		Quantity:     q,
		Price:        p,
		TradeValue:   tv,
		TradeDate:    t.TradeDate,
		CloseReason:  t.CloseReason,
		EntryTradeID: t.EntryTradeID,
		CreatedAt:    t.CreatedAt,
	}
	if t.SignalStrength != nil {
		v, _ := t.SignalStrength.Float64()
		j.SignalStrength = &v
	}
	if t.Confidence != nil {
		v, _ := t.Confidence.Float64()
		j.Confidence = &v
	}
	if t.PnL != nil {
		v, _ := t.PnL.Float64()
		j.PnL = &v
	}
	if t.PnLPct != nil {
		v, _ := t.PnLPct.Float64()
		j.PnLPct = &v
	}
	return j
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

type simBotListItem struct {
	simBotJSON
	LastSession    *simSessionSummary `json:"last_session,omitempty"`
	TotalReturnPct float64            `json:"total_return_pct"`
	TotalTrades    int                `json:"total_trades"`
}

type simSessionSummary struct {
	ID        int64      `json:"id"`
	Status    string     `json:"status"`
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
}

type simBotDetail struct {
	simBotJSON
	LastSession *simSessionSummary `json:"last_session,omitempty"`
	KPIs        simBotKPIs         `json:"kpis"`
}

type simTradesPage struct {
	BotID     string         `json:"bot_id"`
	SessionID int64          `json:"session_id"`
	Total     int64          `json:"total"`
	Page      int            `json:"page"`
	Limit     int            `json:"limit"`
	Data      []simTradeJSON `json:"data"`
}

type simChartResponse struct {
	BotID      string    `json:"bot_id"`
	SessionID  int64     `json:"session_id"`
	Dates      []string  `json:"dates"`
	Values     []float64 `json:"values"`
	ReturnsPct []float64 `json:"returns_pct"`
}

type leaderboardEntry struct {
	Rank                int                `json:"rank"`
	BotID               string             `json:"bot_id"`
	DisplayName         string             `json:"display_name"`
	Market              string             `json:"market"`
	Algorithm           string             `json:"algorithm"`
	Currency            string             `json:"currency"`
	IsActive            bool               `json:"is_active"`
	InitialCapital      float64            `json:"initial_capital"`
	FinalValue          float64            `json:"final_value"`
	TotalReturnPct      float64            `json:"total_return_pct"`
	AnnualizedReturnPct float64            `json:"annualized_return_pct"`
	SharpeRatio         float64            `json:"sharpe_ratio"`
	MaxDrawdownPct      float64            `json:"max_drawdown_pct"`
	WinRatePct          float64            `json:"win_rate_pct"`
	ProfitFactor        float64            `json:"profit_factor"`
	TotalTrades         int                `json:"total_trades"`
	SimulationPeriod    *simPeriod         `json:"simulation_period,omitempty"`
}

type simPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type leaderboardResponse struct {
	Leaderboard []leaderboardEntry `json:"leaderboard"`
	Summary     leaderboardSummary `json:"summary"`
}

type leaderboardSummary struct {
	TotalBots     int     `json:"total_bots"`
	BestMarket    string  `json:"best_market"`
	BestAlgorithm string  `json:"best_algorithm"`
	AvgReturnPct  float64 `json:"avg_return_pct"`
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// GetSimBots godoc
//
//	@Summary      List all simulation bots
//	@Description  Returns all trading bots with their latest session status and summary KPIs.
//	@Tags         Simulation
//	@Produce      json
//	@Success      200  {array}   simBotListItem
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/simulation/bots [get]
func GetSimBots(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	bots, err := store.GetAllSimBots()
	if err != nil {
		logger.Logger.Errorf("GetSimBots: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to fetch bots")
		return
	}

	items := make([]simBotListItem, 0, len(bots))
	for _, bot := range bots {
		item := simBotListItem{simBotJSON: botToJSON(bot)}

		sess, err := store.GetLatestSimSession(bot.ID)
		if err == nil && sess != nil {
			item.LastSession = &simSessionSummary{
				ID:        sess.ID,
				Status:    sess.Status,
				StartDate: sess.StartDate,
				EndDate:   sess.EndDate,
			}
			// Quick KPI: last snapshot's total_return_pct + sell trade count
			snaps, err := store.GetSimPortfolioSnapshots(sess.ID)
			if err == nil && len(snaps) > 0 {
				last := snaps[len(snaps)-1]
				if last.TotalReturnPct != nil {
					v, _ := last.TotalReturnPct.Float64()
					item.TotalReturnPct = v
				}
			}
			trades, _, err := store.GetSimTrades(sess.ID, 0, 10000)
			if err == nil {
				for _, t := range trades {
					if t.Action == "SELL" {
						item.TotalTrades++
					}
				}
			}
		}
		items = append(items, item)
	}

	ResponseSuccess(w, http.StatusOK, items)
}

// GetSimBot godoc
//
//	@Summary      Get simulation bot details
//	@Description  Returns full details and KPIs for a single bot identified by {id}.
//	@Tags         Simulation
//	@Produce      json
//	@Param        id   path      string  true  "Bot ID (e.g. vn30_lstm_nn)"
//	@Success      200  {object}  simBotDetail
//	@Failure      404  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id} [get]
func GetSimBot(w http.ResponseWriter, r *http.Request) {
	// Pattern registered as "/api/simulation/bots/" — extract id from URL tail.
	id := r.PathValue("id")
	if id == "" {
		// Fallback: trim prefix manually for the catch-all pattern.
		path := r.URL.Path
		prefix := "/api/simulation/bots/"
		if len(path) > len(prefix) {
			id = path[len(prefix):]
		}
	}
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	store := repository.GetSingleton()
	bot, err := store.GetSimBotByID(id)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "bot not found")
		return
	}

	detail := simBotDetail{simBotJSON: botToJSON(*bot)}

	// Use session with most snapshots for KPIs (backtest), fall back to latest
	sess, err := store.GetBestSimSessionForChart(bot.ID)
	if err != nil || sess == nil {
		sess, err = store.GetLatestSimSession(bot.ID)
	}
	if err == nil && sess != nil {
		detail.LastSession = &simSessionSummary{
			ID:        sess.ID,
			Status:    sess.Status,
			StartDate: sess.StartDate,
			EndDate:   sess.EndDate,
		}
		snaps, err := store.GetSimPortfolioSnapshots(sess.ID)
		if err != nil {
			snaps = nil
		}
		trades, _, err := store.GetSimTrades(sess.ID, 0, 100000)
		if err != nil {
			trades = nil
		}
		detail.KPIs = computeKPIs(snaps, trades)
	}

	ResponseSuccess(w, http.StatusOK, detail)
}

// GetSimBotTrades godoc
//
//	@Summary      Get bot trade history
//	@Description  Returns paginated trade history for a bot's latest session. Query: ?page=1&limit=50
//	@Tags         Simulation
//	@Produce      json
//	@Param        id     path      string  true   "Bot ID"
//	@Param        page   query     int     false  "Page number (default 1)"
//	@Param        limit  query     int     false  "Results per page (default 50, max 500)"
//	@Success      200    {object}  simTradesPage
//	@Failure      400    {object}  ResponseFailure
//	@Failure      404    {object}  ResponseFailure
//	@Failure      500    {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id}/trades [get]
func GetSimBotTrades(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	page, err := validatePage(r.URL.Query().Get("page"))
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}
	limit, err := validateLimit(r.URL.Query().Get("limit"), 50, 500)
	if err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := repository.GetSingleton()
	sess, err := store.GetLatestSimSession(id)
	if err != nil {
		// No session yet — return empty page (200) instead of 404
		ResponseSuccess(w, http.StatusOK, simTradesPage{
			BotID: id, Total: 0, Page: page, Limit: limit,
			Data: []simTradeJSON{},
		})
		return
	}

	offset := (page - 1) * limit
	trades, total, err := store.GetSimTrades(sess.ID, offset, limit)
	if err != nil {
		logger.Logger.Errorf("GetSimBotTrades: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to fetch trades")
		return
	}

	tradesJSON := make([]simTradeJSON, len(trades))
	for i, t := range trades {
		tradesJSON[i] = tradeToJSON(t)
	}
	ResponseSuccess(w, http.StatusOK, simTradesPage{
		BotID:     id,
		SessionID: sess.ID,
		Total:     total,
		Page:      page,
		Limit:     limit,
		Data:      tradesJSON,
	})
}

// GetSimBotChart godoc
//
//	@Summary      Get bot portfolio value time series
//	@Description  Returns daily portfolio value and cumulative return percentages for chart rendering.
//	@Tags         Simulation
//	@Produce      json
//	@Param        id  path      string  true  "Bot ID"
//	@Success      200 {object}  simChartResponse
//	@Failure      404 {object}  ResponseFailure
//	@Failure      500 {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id}/chart [get]
func GetSimBotChart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	store := repository.GetSingleton()

	// Prefer the session with the most snapshots (typically a completed backtest)
	// instead of the latest session which may be a running live-step with only 1 point.
	sess, err := store.GetBestSimSessionForChart(id)
	if err != nil || sess == nil {
		sess, err = store.GetLatestSimSession(id)
	}
	if err != nil {
		// No session yet — return empty chart (200) instead of 404
		ResponseSuccess(w, http.StatusOK, simChartResponse{
			BotID: id, Dates: []string{}, Values: []float64{}, ReturnsPct: []float64{},
		})
		return
	}

	snaps, err := store.GetSimPortfolioSnapshots(sess.ID)
	if err != nil {
		logger.Logger.Errorf("GetSimBotChart: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to fetch snapshots")
		return
	}

	dates := make([]string, 0, len(snaps))
	values := make([]float64, 0, len(snaps))
	returnsPct := make([]float64, 0, len(snaps))

	for _, s := range snaps {
		dates = append(dates, s.SnapshotDate.Format("2006-01-02"))
		v, _ := s.TotalValue.Float64()
		values = append(values, v)
		rPct := 0.0
		if s.TotalReturnPct != nil {
			rPct, _ = s.TotalReturnPct.Float64()
		}
		returnsPct = append(returnsPct, rPct)
	}

	ResponseSuccess(w, http.StatusOK, simChartResponse{
		BotID:      id,
		SessionID:  sess.ID,
		Dates:      dates,
		Values:     values,
		ReturnsPct: returnsPct,
	})
}

// GetSimLeaderboard godoc
//
//	@Summary      Get simulation leaderboard
//	@Description  Returns all bots sorted by total_return_pct. Supports filtering by market, algorithm, and currency.
//	@Tags         Simulation
//	@Produce      json
//	@Param        market     query  string  false  "Filter by market (e.g. VN30, CRYPTO)"
//	@Param        algorithm  query  string  false  "Filter by algorithm key (e.g. lstm_nn)"
//	@Param        currency   query  string  false  "Filter by currency (VND or USD)"
//	@Success      200        {object}  leaderboardResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/simulation/leaderboard [get]
func GetSimLeaderboard(w http.ResponseWriter, r *http.Request) {
	marketFilter := r.URL.Query().Get("market")
	algoFilter := r.URL.Query().Get("algorithm")
	currencyFilter := r.URL.Query().Get("currency")

	store := repository.GetSingleton()
	bots, err := store.GetAllSimBots()
	if err != nil {
		logger.Logger.Errorf("GetSimLeaderboard: %v", err)
		ResponseError(w, http.StatusInternalServerError, "failed to fetch bots")
		return
	}

	entries := make([]leaderboardEntry, 0, len(bots))
	for _, bot := range bots {
		if marketFilter != "" && bot.Market != marketFilter {
			continue
		}
		if algoFilter != "" && bot.Algorithm != algoFilter {
			continue
		}
		if currencyFilter != "" && bot.Currency != currencyFilter {
			continue
		}

		entry := leaderboardEntry{
			BotID:          bot.ID,
			DisplayName:    bot.DisplayName,
			Market:         bot.Market,
			Algorithm:      bot.Algorithm,
			Currency:       bot.Currency,
			IsActive:       bot.IsActive,
		}
		ic, _ := bot.InitialCapital.Float64()
		entry.InitialCapital = ic

		// Prefer session with most snapshots for KPIs
		sess, err := store.GetBestSimSessionForChart(bot.ID)
		if err != nil || sess == nil {
			sess, err = store.GetLatestSimSession(bot.ID)
		}
		if err == nil && sess != nil {
			entry.SimulationPeriod = &simPeriod{
				Start: sess.StartDate.Format("2006-01-02"),
			}
			if sess.EndDate != nil {
				entry.SimulationPeriod.End = sess.EndDate.Format("2006-01-02")
			}

			snaps, err := store.GetSimPortfolioSnapshots(sess.ID)
			if err != nil {
				snaps = nil
			}
			trades, _, err := store.GetSimTrades(sess.ID, 0, 100000)
			if err != nil {
				trades = nil
			}
			kpis := computeKPIs(snaps, trades)

			entry.TotalReturnPct = kpis.TotalReturnPct
			entry.AnnualizedReturnPct = kpis.AnnualizedReturnPct
			entry.SharpeRatio = kpis.SharpeRatio
			entry.MaxDrawdownPct = kpis.MaxDrawdownPct
			entry.WinRatePct = kpis.WinRatePct
			entry.ProfitFactor = kpis.ProfitFactor
			entry.TotalTrades = kpis.TotalTrades

			if len(snaps) > 0 {
				fv, _ := snaps[len(snaps)-1].TotalValue.Float64()
				entry.FinalValue = fv
			}
		}
		entries = append(entries, entry)
	}

	// Sort by TotalReturnPct DESC.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TotalReturnPct > entries[j].TotalReturnPct
	})
	for i := range entries {
		entries[i].Rank = i + 1
	}

	// Compute summary.
	summary := leaderboardSummary{TotalBots: len(entries)}
	if len(entries) > 0 {
		sum := 0.0
		for _, e := range entries {
			sum += e.TotalReturnPct
		}
		summary.AvgReturnPct = sum / float64(len(entries))
		summary.BestMarket = entries[0].Market
		summary.BestAlgorithm = entries[0].Algorithm
	}

	ResponseSuccess(w, http.StatusOK, leaderboardResponse{
		Leaderboard: entries,
		Summary:     summary,
	})
}

// TriggerSimBotRun godoc
//
//	@Summary      Trigger backtest for a single bot
//	@Description  Sends a gRPC request to run backtest simulation for the specified bot. Accepts optional JSON body with start_date and end_date.
//	@Tags         Simulation
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path      string  true  "Bot ID"
//	@Param        body  body      object  false "Optional: {start_date: '2024-01-01', end_date: '2026-05-31'}"
//	@Success      200   {object}  map[string]string
//	@Failure      400   {object}  ResponseFailure
//	@Failure      401   {object}  ResponseFailure
//	@Failure      500   {object}  ResponseFailure
//	@Failure      503   {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id}/run [post]
func TriggerSimBotRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	var body struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	resp, err := client.TriggerSimulationBacktest(r.Context(), &pb.SimulationRequest{
		BotId:     id,
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
	})
	if err != nil {
		logger.Logger.Errorf("TriggerSimBotRun(%s): %v", id, err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// TriggerSimRunAll godoc
//
//	@Summary      Trigger backtest for all bots
//	@Description  Sends a gRPC request to run backtest simulation for all active bots. Accepts optional JSON body with start_date and end_date.
//	@Tags         Simulation
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        body  body      object  false "Optional: {start_date: '2024-01-01', end_date: '2026-05-31'}"
//	@Success      200   {object}  map[string]string
//	@Failure      401   {object}  ResponseFailure
//	@Failure      500   {object}  ResponseFailure
//	@Failure      503   {object}  ResponseFailure
//	@Router       /api/simulation/run-all [post]
func TriggerSimRunAll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	client := requireGRPCClient(w)
	if client == nil {
		return
	}

	resp, err := client.TriggerSimulationBacktest(r.Context(), &pb.SimulationRequest{
		BotId:     "",
		StartDate: body.StartDate,
		EndDate:   body.EndDate,
	})
	if err != nil {
		logger.Logger.Errorf("TriggerSimRunAll: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": resp.Message})
}

// UpdateSimBotConfig godoc
//
//	@Summary      Update bot configuration
//	@Description  Updates trading parameters for a single bot. Requires admin role.
//	@Tags         Simulation
//	@Accept       json
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id    path      string          true  "Bot ID"
//	@Param        body  body      modelsdb.SimBot true  "Bot configuration fields to update"
//	@Success      200   {object}  map[string]string
//	@Failure      400   {object}  ResponseFailure
//	@Failure      401   {object}  ResponseFailure
//	@Failure      403   {object}  ResponseFailure
//	@Failure      404   {object}  ResponseFailure
//	@Failure      500   {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id}/config [put]
func UpdateSimBotConfig(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	store := repository.GetSingleton()
	existing, err := store.GetSimBotByID(id)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "bot not found")
		return
	}

	var req modelsdb.SimBot
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ResponseError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Only update configurable fields — preserve ID, Market, Algorithm, Currency.
	existing.BuyThreshold = req.BuyThreshold
	existing.SellThreshold = req.SellThreshold
	existing.MinConfidence = req.MinConfidence
	existing.StopLoss = req.StopLoss
	existing.TakeProfit = req.TakeProfit
	existing.MaxPositionPct = req.MaxPositionPct
	existing.MaxPositions = req.MaxPositions
	existing.IsActive = req.IsActive
	existing.UpdatedAt = time.Now()

	if err := store.UpdateSimBotConfig(existing); err != nil {
		logger.Logger.Errorf("UpdateSimBotConfig(%s): %v", id, err)
		ResponseError(w, http.StatusInternalServerError, "failed to update bot config")
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "bot config updated"})
}

// ToggleSimBot godoc
//
//	@Summary      Toggle bot active status
//	@Description  Flips the is_active flag for a bot. Requires admin role.
//	@Tags         Simulation
//	@Produce      json
//	@Security     BearerAuth
//	@Param        id  path      string  true  "Bot ID"
//	@Success      200 {object}  map[string]interface{}
//	@Failure      400 {object}  ResponseFailure
//	@Failure      401 {object}  ResponseFailure
//	@Failure      403 {object}  ResponseFailure
//	@Failure      404 {object}  ResponseFailure
//	@Failure      500 {object}  ResponseFailure
//	@Router       /api/simulation/bots/{id}/toggle [post]
func ToggleSimBot(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	id := r.PathValue("id")
	if id == "" {
		ResponseError(w, http.StatusBadRequest, "bot id is required")
		return
	}

	store := repository.GetSingleton()
	existing, err := store.GetSimBotByID(id)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "bot not found")
		return
	}

	existing.IsActive = !existing.IsActive
	existing.UpdatedAt = time.Now()

	if err := store.UpdateSimBotConfig(existing); err != nil {
		logger.Logger.Errorf("ToggleSimBot(%s): %v", id, err)
		ResponseError(w, http.StatusInternalServerError, "failed to toggle bot")
		return
	}

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"message":   "bot toggled",
		"is_active": existing.IsActive,
	})
}

// ── unused import guard for strconv ─────────────────────────────────────────
var _ = strconv.Itoa
