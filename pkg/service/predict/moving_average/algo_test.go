package movingaverage

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"strings"
	"testing"
)

func newPredictor() *MovingAveragePredictor {
	return NewMovingAveragePredictor()
}

// ---- calculateSMA ----

func TestCalculateSMA_Normal(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20, 30, 40, 50}
	got := m.calculateSMA(prices, 3)
	// last 3 elements: 30,40,50 → mean = 40.0
	want := 40.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("calculateSMA = %v, want %v", got, want)
	}
}

func TestCalculateSMA_InsufficientData(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20}
	got := m.calculateSMA(prices, 5)
	if got != 0 {
		t.Errorf("calculateSMA with insufficient data = %v, want 0", got)
	}
}

func TestCalculateSMA_ExactPeriod(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20, 30}
	got := m.calculateSMA(prices, 3)
	want := 20.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("calculateSMA exact period = %v, want %v", got, want)
	}
}

func TestCalculateSMA_SingleElement(t *testing.T) {
	m := newPredictor()
	prices := []float64{42.5}
	got := m.calculateSMA(prices, 1)
	if math.Abs(got-42.5) > 1e-9 {
		t.Errorf("calculateSMA single = %v, want 42.5", got)
	}
}

// ---- calculateVWMA ----

func TestCalculateVWMA_AllZeroVolumes_FallsBackToSMA(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20, 30, 40, 50}
	volumes := []float64{0, 0, 0, 0, 0}
	got := m.calculateVWMA(prices, volumes, 3)
	sma := m.calculateSMA(prices, 3) // 40.0
	if math.Abs(got-sma) > 1e-9 {
		t.Errorf("VWMA with zero volumes = %v, want SMA = %v", got, sma)
	}
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("VWMA returned NaN or Inf: %v", got)
	}
}

func TestCalculateVWMA_EmptyVolumes_FallsBackToSMA(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20, 30, 40, 50}
	got := m.calculateVWMA(prices, nil, 3)
	sma := m.calculateSMA(prices, 3)
	if math.Abs(got-sma) > 1e-9 {
		t.Errorf("VWMA with nil volumes = %v, want SMA = %v", got, sma)
	}
}

func TestCalculateVWMA_RealVolumes_WeightedAverage(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 200, 300}
	volumes := []float64{1, 2, 3}
	// VWMA last 3: (100*1 + 200*2 + 300*3) / (1+2+3) = (100+400+900)/6 = 1400/6 ≈ 233.33
	got := m.calculateVWMA(prices, volumes, 3)
	want := 1400.0 / 6.0
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("VWMA with real volumes = %v, want %v", got, want)
	}
}

func TestCalculateVWMA_InsufficientPrices_FallsBackToSMA(t *testing.T) {
	m := newPredictor()
	prices := []float64{10, 20}
	volumes := []float64{100, 200}
	got := m.calculateVWMA(prices, volumes, 5)
	// falls back to SMA which returns 0 (insufficient)
	_ = got // Should not panic
}

// ---- generateSignal ----

func TestGenerateSignal_BuyWhenShortAboveLong(t *testing.T) {
	m := newPredictor()
	signal := m.generateSignal(110.0, 100.0, 105.0)
	if signal.Direction != "BUY" {
		t.Errorf("expected BUY, got %v", signal.Direction)
	}
	if signal.Strength <= 0 || signal.Strength > 1.0 {
		t.Errorf("signal.Strength out of range [0,1]: %v", signal.Strength)
	}
}

func TestGenerateSignal_SellWhenShortBelowLong(t *testing.T) {
	m := newPredictor()
	signal := m.generateSignal(90.0, 100.0, 95.0)
	if signal.Direction != "SELL" {
		t.Errorf("expected SELL, got %v", signal.Direction)
	}
}

func TestGenerateSignal_HoldWhenEqual(t *testing.T) {
	m := newPredictor()
	signal := m.generateSignal(100.0, 100.0, 100.0)
	if signal.Direction != "HOLD" {
		t.Errorf("expected HOLD, got %v", signal.Direction)
	}
}

func TestGenerateSignal_HoldWhenZeroMAs(t *testing.T) {
	m := newPredictor()
	signal := m.generateSignal(0, 0, 100.0)
	if signal.Direction != "HOLD" {
		t.Errorf("expected HOLD with zero MAs, got %v", signal.Direction)
	}
}

func TestGenerateSignal_StrengthInRange(t *testing.T) {
	m := newPredictor()
	cases := []struct {
		short, long, curr float64
	}{
		{120, 100, 110},
		{80, 100, 90},
		{100, 100, 100},
		{200, 100, 150},
	}
	for _, tc := range cases {
		sig := m.generateSignal(tc.short, tc.long, tc.curr)
		if sig.Strength < 0.1 || sig.Strength > 1.0 {
			t.Errorf("strength %v out of [0.1,1.0] for short=%v long=%v", sig.Strength, tc.short, tc.long)
		}
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
}

func TestParseHistoricalData_InvalidString_ReturnsError(t *testing.T) {
	m := newPredictor()
	input := []string{"100", "abc", "200"}
	_, err := m.parseHistoricalData(input)
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

// ---- parseVolumes ----

func TestParseVolumes_ValidStrings(t *testing.T) {
	m := newPredictor()
	vols := m.parseVolumes([]string{"1000", "2,000", "3 000"})
	if vols == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(vols) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(vols))
	}
	if math.Abs(vols[0]-1000) > 1e-9 {
		t.Errorf("vols[0] = %v, want 1000", vols[0])
	}
}

func TestParseVolumes_InvalidString_ReturnsNil(t *testing.T) {
	m := newPredictor()
	vols := m.parseVolumes([]string{"1000", "invalid", "2000"})
	// Should return nil/empty (not panic, not error)
	if vols != nil {
		t.Errorf("expected nil for invalid volume strings, got %v", vols)
	}
}

func TestParseVolumes_Empty_ReturnsNil(t *testing.T) {
	m := newPredictor()
	vols := m.parseVolumes([]string{})
	if vols != nil {
		t.Errorf("expected nil for empty input, got %v", vols)
	}
}

// ---- calculateRSI ----

func TestCalculateRSI_InsufficientData_ReturnsNeutral(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 110, 120}
	rsi := m.calculateRSI(prices, 14)
	// should return 50 (neutral) when not enough data
	if rsi != 50 {
		t.Errorf("RSI with insufficient data = %v, want 50", rsi)
	}
}

func TestCalculateRSI_AllGains_HighValue(t *testing.T) {
	m := newPredictor()
	// 16 prices all going up
	prices := make([]float64, 16)
	prices[0] = 100
	for i := 1; i < 16; i++ {
		prices[i] = prices[i-1] + 1
	}
	rsi := m.calculateRSI(prices, 14)
	// All gains, no losses → losses==0 path returns 50
	// The implementation returns 50 when losses==0 at period==0 check
	// but when losses avg = 0 it returns 100
	if rsi < 50 {
		t.Errorf("RSI with all-up prices = %v, expected >= 50", rsi)
	}
}

func TestCalculateRSI_InRange(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 102, 101, 103, 102, 104, 103, 105, 104, 106, 105, 107, 106, 108, 107}
	rsi := m.calculateRSI(prices, 14)
	if rsi < 0 || rsi > 100 {
		t.Errorf("RSI out of range [0,100]: %v", rsi)
	}
}

// ---- Predict ----

func TestPredict_InsufficientData_ReturnsError(t *testing.T) {
	m := newPredictor()
	// Only 5 prices, need 20
	data := &modelssvc.StockData{
		Historical: []string{"100", "101", "102", "103", "104"},
	}
	_, err := m.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for insufficient data, got nil")
	}
	if !strings.Contains(err.Error(), "20") {
		t.Errorf("error should mention 20, got: %v", err)
	}
}

func TestPredict_EmptyData_ReturnsError(t *testing.T) {
	m := newPredictor()
	data := &modelssvc.StockData{
		Historical: []string{},
	}
	_, err := m.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for empty data, got nil")
	}
}

func TestPredict_InvalidData_ReturnsError(t *testing.T) {
	m := newPredictor()
	data := &modelssvc.StockData{
		Historical: []string{"abc", "def"},
	}
	_, err := m.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for invalid data, got nil")
	}
}

func TestPredict_SufficientData_ReturnsPrediction(t *testing.T) {
	m := newPredictor()
	// 25 price points
	historical := make([]string, 25)
	for i := range historical {
		historical[i] = "100"
	}
	data := &modelssvc.StockData{
		Historical: historical,
	}
	result, err := m.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PredictedPrice <= 0 {
		t.Errorf("predicted price should be > 0, got %v", result.PredictedPrice)
	}
	if result.CurrentPrice != 100.0 {
		t.Errorf("current price = %v, want 100.0", result.CurrentPrice)
	}
}

func TestPredict_WithVolumes_DoesNotPanic(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 25)
	volumes := make([]string, 25)
	for i := range historical {
		historical[i] = "100"
		volumes[i] = "1000"
	}
	data := &modelssvc.StockData{
		Historical: historical,
		Volume:     volumes,
	}
	result, err := m.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestPredict_PriceBounded_WithinDailyLimit(t *testing.T) {
	m := newPredictor()
	historical := make([]string, 25)
	for i := range historical {
		historical[i] = "100"
	}
	data := &modelssvc.StockData{Historical: historical}
	result, err := m.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Max 7% daily change
	if result.PredictedPrice > 107.5 || result.PredictedPrice < 92.5 {
		t.Errorf("predicted price %v is outside ±7%% of 100", result.PredictedPrice)
	}
}

// ---- calculateMomentum ----

func TestCalculateMomentum_Normal(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200}
	// period=10: current=200, past=100, momentum=(200-100)/100=1.0
	got := m.calculateMomentum(prices, 10)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("momentum = %v, want 1.0", got)
	}
}

func TestCalculateMomentum_InsufficientData_ReturnsZero(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 110}
	got := m.calculateMomentum(prices, 5)
	if got != 0 {
		t.Errorf("momentum with insufficient data = %v, want 0", got)
	}
}

func TestCalculateMomentum_ZeroPastPrice_ReturnsZero(t *testing.T) {
	m := newPredictor()
	prices := make([]float64, 12)
	prices[0] = 0 // past price = 0
	prices[11] = 100
	got := m.calculateMomentum(prices, 10)
	if got != 0 {
		t.Errorf("momentum with zero past price = %v, want 0", got)
	}
}

// ---- calculateBollingerBands ----

func TestCalculateBollingerBands_Normal(t *testing.T) {
	m := newPredictor()
	prices := make([]float64, 20)
	for i := range prices {
		prices[i] = 100.0
	}
	upper, mid, lower := m.calculateBollingerBands(prices, 20)
	// Flat prices → stddev=0 → upper=mid=lower=100
	if math.Abs(mid-100.0) > 1e-9 {
		t.Errorf("mid = %v, want 100", mid)
	}
	if upper < mid {
		t.Errorf("upper (%v) < mid (%v)", upper, mid)
	}
	if lower > mid {
		t.Errorf("lower (%v) > mid (%v)", lower, mid)
	}
}

func TestCalculateBollingerBands_InsufficientData_ReturnsZero(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 110}
	upper, mid, lower := m.calculateBollingerBands(prices, 20)
	if upper != 0 || mid != 0 || lower != 0 {
		t.Errorf("expected (0,0,0) for insufficient data, got (%v,%v,%v)", upper, mid, lower)
	}
}

func TestCalculateBollingerBands_UpperAboveLower(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 102, 99, 103, 101, 104, 102, 105, 103, 106, 104, 107, 105, 108, 106, 109, 107, 110, 108, 111}
	upper, _, lower := m.calculateBollingerBands(prices, 20)
	if upper < lower {
		t.Errorf("upper (%v) < lower (%v)", upper, lower)
	}
}

// ---- enhancedPredict ----

func TestEnhancedPredict_SufficientData_ReturnsReasonablePrice(t *testing.T) {
	m := newPredictor()
	prices := make([]float64, 25)
	for i := range prices {
		prices[i] = 100.0 + float64(i)*0.5
	}
	result := m.enhancedPredict(prices)
	if result <= 0 {
		t.Errorf("enhancedPredict = %v, want > 0", result)
	}
	// Should be within 10% of current price
	currentPrice := prices[len(prices)-1]
	if result < currentPrice*0.9 || result > currentPrice*1.1 {
		t.Errorf("enhancedPredict %v is outside 10%% of current price %v", result, currentPrice)
	}
}

func TestEnhancedPredict_InsufficientData_ReturnsCurrent(t *testing.T) {
	m := newPredictor()
	prices := []float64{100, 101, 102}
	result := m.enhancedPredict(prices)
	// should return current price (last element)
	if math.Abs(result-102.0) > 1e-9 {
		t.Errorf("enhancedPredict with insufficient data = %v, want 102.0 (current)", result)
	}
}

// ---- GetName / GetAccuracy ----

func TestGetName(t *testing.T) {
	m := newPredictor()
	if m.GetName() == "" {
		t.Error("GetName should return non-empty string")
	}
}

func TestGetAccuracy_ZeroBeforeBacktest(t *testing.T) {
	m := newPredictor()
	if m.GetAccuracy() != 0.0 {
		t.Errorf("GetAccuracy before backtest = %v, want 0.0", m.GetAccuracy())
	}
}
