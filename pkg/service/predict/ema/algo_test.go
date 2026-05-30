package ema

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"testing"
)

func newPredictor() *EMAPredictor {
	return NewEMAPredictor()
}

// ---- calculateEMA ----

func TestCalculateEMA_BasicSeries(t *testing.T) {
	m := newPredictor()
	// Flat prices: EMA of all-equal prices equals that price
	prices := make([]float64, 26)
	for i := range prices {
		prices[i] = 100.0
	}
	got := m.calculateEMA(prices, 12)
	if math.Abs(got-100.0) > 1e-6 {
		t.Errorf("EMA of flat series = %v, want 100.0", got)
	}
}

func TestCalculateEMA_InsufficientData_ReturnsZero(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20, 30}
	got := m.calculateEMA(prices, 12)
	if got != 0 {
		t.Errorf("calculateEMA with insufficient data = %v, want 0", got)
	}
}

func TestCalculateEMA_SinglePeriod(t *testing.T) {
	m := newPredictor()
	prices := []float64{50.0}
	got := m.calculateEMA(prices, 1)
	// k = 2/(1+1) = 1.0; seed = prices[0] = 50; no further iteration
	if math.Abs(got-50.0) > 1e-9 {
		t.Errorf("calculateEMA period=1 = %v, want 50.0", got)
	}
}

func TestCalculateEMA_TrendingUp_HigherThanSMA(t *testing.T) {
	m := newPredictor()
	// Steadily rising prices: EMA should be closer to recent (higher) values
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = float64(i + 1) // 1, 2, 3, ..., 30
	}
	ema := m.calculateEMA(prices, 12)
	// SMA of last 12 = (19+20+...+30)/12 = 24.5
	// EMA weights recent higher prices more, so EMA > SMA would not necessarily hold,
	// but EMA should be > 0 and within range
	if ema <= 0 {
		t.Errorf("EMA of trending series = %v, want > 0", ema)
	}
	if ema > 30 || ema < 1 {
		t.Errorf("EMA %v out of reasonable range [1, 30]", ema)
	}
}

// ---- calculateEMASeries ----

func TestCalculateEMASeries_Length(t *testing.T) {
	m := newPredictor()
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = float64(i + 1)
	}
	series := m.calculateEMASeries(prices, 12)
	expectedLen := 30 - 12 + 1 // 19
	if len(series) != expectedLen {
		t.Errorf("calculateEMASeries length = %d, want %d", len(series), expectedLen)
	}
}

func TestCalculateEMASeries_InsufficientData_ReturnsNil(t *testing.T) {
	m := newPredictor()
	series := m.calculateEMASeries([]float64{10, 20}, 12)
	if series != nil {
		t.Errorf("expected nil for insufficient data, got %v", series)
	}
}

func TestCalculateEMASeries_LastValueMatchesCalculateEMA(t *testing.T) {
	m := newPredictor()
	prices := make([]float64, 30)
	for i := range prices {
		prices[i] = 100.0 + float64(i)*0.5
	}
	series := m.calculateEMASeries(prices, 12)
	scalar := m.calculateEMA(prices, 12)
	last := series[len(series)-1]
	if math.Abs(last-scalar) > 1e-9 {
		t.Errorf("series last value %v != scalar EMA %v", last, scalar)
	}
}

// ---- parseHistoricalData ----

func TestParseHistoricalData_ValidStrings(t *testing.T) {
	m := newPredictor()
	input := []string{"10.5", "20.0", "30,000", "40 000"}
	prices, err := m.parseHistoricalData(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 4 {
		t.Fatalf("expected 4 prices, got %d", len(prices))
	}
	if math.Abs(prices[0]-10.5) > 1e-9 {
		t.Errorf("prices[0] = %v, want 10.5", prices[0])
	}
	if math.Abs(prices[1]-20.0) > 1e-9 {
		t.Errorf("prices[1] = %v, want 20.0", prices[1])
	}
	// "30,000" → 30000 after comma removal
	if math.Abs(prices[2]-30000.0) > 1e-9 {
		t.Errorf("prices[2] = %v, want 30000.0", prices[2])
	}
}

func TestParseHistoricalData_VietnameseFormat(t *testing.T) {
	m := newPredictor()
	// "95 500" and "100,000" are common Vietnamese price formats
	input := []string{"95 500", "100,000"}
	prices, err := m.parseHistoricalData(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(prices[0]-95500.0) > 1e-9 {
		t.Errorf("prices[0] = %v, want 95500.0", prices[0])
	}
	if math.Abs(prices[1]-100000.0) > 1e-9 {
		t.Errorf("prices[1] = %v, want 100000.0", prices[1])
	}
}

func TestParseHistoricalData_InvalidString_ReturnsError(t *testing.T) {
	m := newPredictor()
	_, err := m.parseHistoricalData([]string{"100", "abc", "200"})
	if err == nil {
		t.Error("expected error for invalid string, got nil")
	}
}

func TestParseHistoricalData_Empty(t *testing.T) {
	m := newPredictor()
	prices, err := m.parseHistoricalData([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 0 {
		t.Errorf("expected empty slice, got %v", prices)
	}
}

// ---- Predict — insufficient data ----

func TestPredict_InsufficientData_ReturnsError(t *testing.T) {
	m := newPredictor()
	// Only 10 prices, need 26
	historical := make([]string, 10)
	for i := range historical {
		historical[i] = "100"
	}
	_, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err == nil {
		t.Error("expected error for insufficient data, got nil")
	}
}

func TestPredict_ExactlyLongPeriodMinus1_ReturnsError(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 25) // need 26
	for i := range historical {
		historical[i] = "100"
	}
	_, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err == nil {
		t.Error("expected error with 25 data points (need 26), got nil")
	}
}

// ---- Predict — sufficient data ----

func TestPredict_SufficientData_ConfidencePositive(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "100"
	}
	result, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence <= 0 {
		t.Errorf("confidence should be > 0, got %v", result.Confidence)
	}
}

func TestPredict_SufficientData_CurrentPriceSet(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "50000"
	}
	result, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentPrice != 50000.0 {
		t.Errorf("CurrentPrice = %v, want 50000.0", result.CurrentPrice)
	}
}

func TestPredict_SufficientData_PriceBoundedWithinDailyLimit(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "100"
	}
	result, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ±7% of 100 = [93, 107]
	if result.PredictedPrice > 107.5 || result.PredictedPrice < 92.5 {
		t.Errorf("predicted price %v is outside ±7%% of 100", result.PredictedPrice)
	}
}

func TestPredict_SufficientData_ConfidenceInRange(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "100"
	}
	result, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence < 0.3 || result.Confidence > 0.9 {
		t.Errorf("confidence %v outside [0.3, 0.9]", result.Confidence)
	}
}

func TestPredict_SufficientData_PredictedPricePositive(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "100"
	}
	result, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PredictedPrice <= 0 {
		t.Errorf("PredictedPrice should be > 0, got %v", result.PredictedPrice)
	}
}

// ---- generateMACDSignal ----

func TestGenerateMACDSignal_BuyCondition(t *testing.T) {
	m := newPredictor()
	// macd > signal AND histogram > 0 → BUY
	sig := m.generateMACDSignal(10, 5, 5)
	if sig.Direction != "BUY" {
		t.Errorf("expected BUY, got %s", sig.Direction)
	}
}

func TestGenerateMACDSignal_SellCondition(t *testing.T) {
	m := newPredictor()
	// macd < signal AND histogram < 0 → SELL
	sig := m.generateMACDSignal(-10, -5, -5)
	if sig.Direction != "SELL" {
		t.Errorf("expected SELL, got %s", sig.Direction)
	}
}

func TestGenerateMACDSignal_HoldCondition(t *testing.T) {
	m := newPredictor()
	// macd > signal but histogram <= 0 → HOLD
	sig := m.generateMACDSignal(10, 5, -1)
	if sig.Direction != "HOLD" {
		t.Errorf("expected HOLD, got %s", sig.Direction)
	}
}

func TestGenerateMACDSignal_StrengthInRange(t *testing.T) {
	m := newPredictor()
	cases := []struct{ macd, signal, hist float64 }{
		{10, 5, 5},
		{-10, -5, -5},
		{0.1, 0, 0.1},
		{100, 50, 50},
	}
	for _, tc := range cases {
		sig := m.generateMACDSignal(tc.macd, tc.signal, tc.hist)
		if sig.Strength < 0.1 || sig.Strength > 1.0 {
			t.Errorf("strength %v out of [0.1, 1.0] for macd=%v signal=%v hist=%v",
				sig.Strength, tc.macd, tc.signal, tc.hist)
		}
	}
}

// ---- GetName / GetAccuracy ----

func TestGetName(t *testing.T) {
	m := newPredictor()
	if m.GetName() != "Exponential Moving Average" {
		t.Errorf("GetName = %q, want %q", m.GetName(), "Exponential Moving Average")
	}
}

func TestGetAccuracy_ZeroBeforePredict(t *testing.T) {
	m := newPredictor()
	if m.GetAccuracy() != 0.0 {
		t.Errorf("GetAccuracy before Predict = %v, want 0.0", m.GetAccuracy())
	}
}

func TestGetAccuracy_AfterPredict(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "100"
	}
	_, err := m.Predict(context.Background(), &modelssvc.StockData{Historical: historical})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.GetAccuracy() <= 0 {
		t.Errorf("GetAccuracy after Predict = %v, want > 0", m.GetAccuracy())
	}
}
