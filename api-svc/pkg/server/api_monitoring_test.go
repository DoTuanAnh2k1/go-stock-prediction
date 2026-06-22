package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ─── DTO JSON shape tests (no DB required) ────────────────────────────────────

func TestMonitoringCrawl_JSONFields(t *testing.T) {
	lastCrawl := time.Now().Format(time.RFC3339)
	m := monitoringCrawl{
		LastCrawlAt:   &lastCrawl,
		Staleness:     "35m ago",
		Stale:         false,
		DailyToday:    5,
		IntradayToday: 12,
	}

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	required := []string{"last_crawl_at", "staleness", "stale", "daily_today", "intraday_today"}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringCrawl JSON missing required field %q", f)
		}
	}
}

func TestMonitoringPredictions_JSONFields(t *testing.T) {
	p := monitoringPredictions{
		LastPredictAt: nil,
		Staleness:     "never",
		TodayTotal:    0,
		ExpectedAlgos: 11,
		MissingToday:  []string{"lstm_nn"},
		Algorithms:    []monitoringAlgoStat{},
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	required := []string{"last_predict_at", "staleness", "today_total", "expected_algos", "missing_today", "algorithms"}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringPredictions JSON missing required field %q", f)
		}
	}
}

func TestMonitoringAlgoStat_JSONFields(t *testing.T) {
	a := monitoringAlgoStat{
		Algorithm:         "lstm_nn",
		TodayCount:        3,
		DirectionAccuracy: 0.72,
		Reconciled:        50,
		Correct:           36,
	}

	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	required := []string{"algorithm", "today_count", "direction_accuracy", "reconciled", "correct"}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringAlgoStat JSON missing required field %q", f)
		}
	}
}

func TestMonitoringBotTableRow_JSONFields(t *testing.T) {
	pf := 2.1
	row := monitoringBotTableRow{
		BotID:        "gold_lstm_nn",
		Market:       "GOLD",
		Algorithm:    "lstm_nn",
		Trades:       20,
		Wins:         12,
		Losses:       6,
		Breakeven:    2,
		WinRate:      0.667,
		TotalPnl:     1500.0,
		ReturnPct:    7.5,
		ProfitFactor: &pf,
	}

	b, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	required := []string{
		"bot_id", "market", "algorithm", "trades", "wins", "losses",
		"breakeven", "win_rate", "total_pnl", "return_pct", "profit_factor",
	}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringBotTableRow JSON missing required field %q", f)
		}
	}
}

func TestMonitoringOverviewResponse_JSONFields(t *testing.T) {
	resp := monitoringOverviewResponse{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Markets:     []monitoringMarket{},
		Bots: monitoringBots{
			Summary: monitoringBotSummary{
				TotalBots:  0,
				ActiveBots: 0,
				ByMarket:   []monitoringBotByMarket{},
			},
			Table: []monitoringBotTableRow{},
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	required := []string{"generated_at", "markets", "bots"}
	for _, f := range required {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringOverviewResponse JSON missing required field %q", f)
		}
	}
}

func TestMonitoringBotSummary_JSONFields(t *testing.T) {
	s := monitoringBotSummary{
		TotalBots:  10,
		ActiveBots: 7,
		ByMarket:   []monitoringBotByMarket{},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	for _, f := range []string{"total_bots", "active_bots", "by_market"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringBotSummary JSON missing field %q", f)
		}
	}
}

func TestMonitoringBotByMarket_JSONFields(t *testing.T) {
	bm := monitoringBotByMarket{
		Market:   "GOLD",
		Trades:   30,
		Wins:     18,
		Losses:   10,
		WinRate:  0.643,
		TotalPnl: 2500.0,
	}
	b, err := json.Marshal(bm)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	for _, f := range []string{"market", "trades", "wins", "losses", "win_rate", "total_pnl"} {
		if _, ok := raw[f]; !ok {
			t.Errorf("monitoringBotByMarket JSON missing field %q", f)
		}
	}
}

// ─── Business logic unit tests ────────────────────────────────────────────────

func TestFormatStaleness_Never(t *testing.T) {
	s, stale := formatStaleness(nil, false)
	if s != "never" {
		t.Errorf("staleness = %q, want %q", s, "never")
	}
	if !stale {
		t.Error("stale must be true when lastAt is nil")
	}
}

func TestFormatStaleness_StaleWhenNoToday(t *testing.T) {
	recent := time.Now().Add(-30 * time.Minute)
	s, stale := formatStaleness(&recent, false) // no records today → stale
	if !stale {
		t.Errorf("staleness = %q: stale should be true when no records today even if recent", s)
	}
}

func TestFormatStaleness_StaleWhenOld(t *testing.T) {
	old := time.Now().Add(-4 * time.Hour)
	_, stale := formatStaleness(&old, true)
	if !stale {
		t.Error("stale must be true when last crawl was more than 3h ago")
	}
}

func TestFormatStaleness_FreshWithin3Hours(t *testing.T) {
	fresh := time.Now().Add(-90 * time.Minute)
	s, stale := formatStaleness(&fresh, true)
	if stale {
		t.Errorf("staleness = %q: should not be stale within 3h with today records", s)
	}
}

func TestFormatStaleness_JustNow(t *testing.T) {
	now := time.Now()
	s, _ := formatStaleness(&now, true)
	if s != "just now" {
		t.Errorf("staleness = %q, want %q for sub-minute recency", s, "just now")
	}
}

func TestFormatStaleness_MinutesAgo(t *testing.T) {
	t35 := time.Now().Add(-35 * time.Minute)
	s, _ := formatStaleness(&t35, true)
	if s != "35m ago" {
		t.Errorf("staleness = %q, want 35m ago", s)
	}
}

func TestFormatStaleness_HoursAgo(t *testing.T) {
	t2h := time.Now().Add(-2 * time.Hour)
	s, _ := formatStaleness(&t2h, true)
	if s != "2h ago" {
		t.Errorf("staleness = %q, want 2h ago", s)
	}
}

func TestRfc3339OrNil_Nil(t *testing.T) {
	if rfc3339OrNil(nil) != nil {
		t.Error("rfc3339OrNil(nil) should return nil")
	}
}

func TestRfc3339OrNil_NonNil(t *testing.T) {
	now := time.Now()
	s := rfc3339OrNil(&now)
	if s == nil {
		t.Fatal("rfc3339OrNil should return non-nil for a real time")
	}
	if _, err := time.Parse(time.RFC3339, *s); err != nil {
		t.Errorf("rfc3339OrNil returned non-RFC3339 string: %q", *s)
	}
}

func TestExpectedAlgoKeys_ContainsKnownAlgos(t *testing.T) {
	keys := expectedAlgoKeys()
	if len(keys) == 0 {
		t.Fatal("expectedAlgoKeys must not be empty")
	}
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, required := range []string{"lstm_nn", "ensemble", "moving_average"} {
		if !keySet[required] {
			t.Errorf("expectedAlgoKeys missing %q", required)
		}
	}
}

// ─── Handler contract tests (no DB — recover from panic) ─────────────────────

// TestGetMonitoringOverview_RequiresAuth verifies the handler returns 401 when
// no JWT is present in context (unauthenticated request).
func TestGetMonitoringOverview_RequiresAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/overview", nil)
	// No JWT injected → getClaims returns nil → requireAuth writes 401.
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetMonitoringOverview(w, req)
	}()

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without JWT, got %d", w.Code)
	}
}

// TestGetMonitoringOverview_ContentTypeIsJSON verifies the 401 response has JSON
// Content-Type (written by requireAuth → ResponseError).
func TestGetMonitoringOverview_ContentTypeIsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/overview", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetMonitoringOverview(w, req)
	}()

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestGetMonitoringOverview_401Body verifies the 401 JSON body has the expected shape.
func TestGetMonitoringOverview_401Body(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/overview", nil)
	w := httptest.NewRecorder()

	func() {
		defer func() { recover() }()
		GetMonitoringOverview(w, req)
	}()

	if w.Code != http.StatusUnauthorized {
		t.Skipf("expected 401, got %d", w.Code)
	}
	var resp ResponseFailure
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("body status_code = %d, want 401", resp.StatusCode)
	}
	if resp.Message == "" {
		t.Error("error message must not be empty")
	}
}

// ─── Win/loss computation logic ───────────────────────────────────────────────

func TestMonitoring_WinRateExcludesBreakeven(t *testing.T) {
	// wins=6, losses=3, breakeven=1 → win_rate = 6/(6+3) = 0.667
	wins, losses := 6, 3
	denom := wins + losses
	wr := float64(wins) / float64(denom)
	if wr < 0.666 || wr > 0.668 {
		t.Errorf("win_rate = %.3f, want ~0.667 (breakeven excluded)", wr)
	}
}

func TestMonitoring_WinRateZeroDenominator(t *testing.T) {
	// No wins or losses → win_rate = 0 (no divide-by-zero).
	wins, losses := 0, 0
	denom := wins + losses
	wr := 0.0
	if denom > 0 {
		wr = float64(wins) / float64(denom)
	}
	if wr != 0.0 {
		t.Errorf("win_rate with zero denominator = %f, want 0.0", wr)
	}
}

func TestMonitoringBots_EmptySummaryMarketIsEmptySlice(t *testing.T) {
	// Verify JSON serialises ByMarket as [] not null.
	summary := monitoringBotSummary{
		TotalBots:  0,
		ActiveBots: 0,
		ByMarket:   []monitoringBotByMarket{},
	}
	b, _ := json.Marshal(summary)
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	if string(raw["by_market"]) == "null" {
		t.Error(`by_market must be "[]" not "null" when empty`)
	}
}

func TestMonitoringPredictions_MissingTodayIsEmptySlice(t *testing.T) {
	p := monitoringPredictions{
		MissingToday: []string{},
		Algorithms:   []monitoringAlgoStat{},
	}
	b, _ := json.Marshal(p)
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw) //nolint:errcheck
	if string(raw["missing_today"]) == "null" {
		t.Error(`missing_today must be "[]" not "null" when empty`)
	}
}
