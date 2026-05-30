package lstmnn

import (
	"context"
	"fmt"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"testing"
)

func newLSTMPredictor() *LSTMPredictor {
	return NewLSTMPredictor()
}

// ---- parseHistoricalData ----

func TestLSTM_ParseHistoricalData_ValidStrings(t *testing.T) {
	l := newLSTMPredictor()
	input := []string{"100", "200.5", "300,000"}
	prices, err := l.parseHistoricalData(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 3 {
		t.Fatalf("expected 3 prices, got %d", len(prices))
	}
	if math.Abs(prices[0]-100) > 1e-9 {
		t.Errorf("prices[0] = %v, want 100", prices[0])
	}
}

func TestLSTM_ParseHistoricalData_InvalidString_ReturnsError(t *testing.T) {
	l := newLSTMPredictor()
	_, err := l.parseHistoricalData([]string{"100", "not_a_number"})
	if err == nil {
		t.Error("expected error for invalid string, got nil")
	}
}

func TestLSTM_ParseHistoricalData_Empty(t *testing.T) {
	l := newLSTMPredictor()
	prices, err := l.parseHistoricalData([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 0 {
		t.Errorf("expected empty slice, got %v", prices)
	}
}

// ---- calculateRSI ----

func TestLSTM_CalculateRSI_AllUp_HighValue(t *testing.T) {
	l := newLSTMPredictor()
	// 20 prices monotonically increasing
	prices := make([]float64, 20)
	prices[0] = 100
	for i := 1; i < 20; i++ {
		prices[i] = prices[i-1] + 1
	}
	rsi := l.calculateRSI(prices, 14, 14)
	// All gains, no losses → returns 100
	if rsi < 90 {
		t.Errorf("RSI all-up = %v, expected close to 100", rsi)
	}
}

func TestLSTM_CalculateRSI_AllDown_LowValue(t *testing.T) {
	l := newLSTMPredictor()
	prices := make([]float64, 20)
	prices[0] = 200
	for i := 1; i < 20; i++ {
		prices[i] = prices[i-1] - 1
	}
	rsi := l.calculateRSI(prices, 14, 14)
	// All losses, no gains → gains=0 → rs=0 → RSI = 100 - 100/(1+0) = 0
	if rsi > 10 {
		t.Errorf("RSI all-down = %v, expected close to 0", rsi)
	}
}

func TestLSTM_CalculateRSI_InsufficientData_ReturnsNeutral(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{100, 110, 120}
	rsi := l.calculateRSI(prices, 1, 14)
	// index < period → return 50
	if rsi != 50 {
		t.Errorf("RSI with insufficient data = %v, want 50", rsi)
	}
}

func TestLSTM_CalculateRSI_InRange(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{100, 102, 99, 103, 101, 104, 102, 105, 103, 106, 104, 107, 105, 108, 106, 109, 107, 110, 108, 111}
	rsi := l.calculateRSI(prices, len(prices)-1, 14)
	if rsi < 0 || rsi > 100 {
		t.Errorf("RSI = %v, expected in [0,100]", rsi)
	}
}

// ---- calculateSMA ----

func TestLSTM_CalculateSMA_Normal(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{10, 20, 30}
	// calculateSMA(prices, index=2, period=3) → sum(10+20+30)/3 = 20
	got := l.calculateSMA(prices, 2, 3)
	want := 20.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("calculateSMA = %v, want %v", got, want)
	}
}

func TestLSTM_CalculateSMA_InsufficientData_ReturnsCurrent(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{10, 20, 30}
	// index=1, period=3 → index < period-1, returns prices[1] = 20
	got := l.calculateSMA(prices, 1, 3)
	if math.Abs(got-20.0) > 1e-9 {
		t.Errorf("calculateSMA with insufficient data = %v, want 20.0 (current price)", got)
	}
}

func TestLSTM_CalculateSMA_SinglePeriod(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{10, 20, 30}
	got := l.calculateSMA(prices, 2, 1)
	if math.Abs(got-30.0) > 1e-9 {
		t.Errorf("calculateSMA period=1 = %v, want 30.0", got)
	}
}

// ---- calculateVolatility ----

func TestLSTM_CalculateVolatility_FlatPrices_NearZero(t *testing.T) {
	l := newLSTMPredictor()
	// All same prices → log returns = 0 → volatility = 0
	prices := make([]float64, 15)
	for i := range prices {
		prices[i] = 100.0
	}
	vol := l.calculateVolatility(prices, 14, 10)
	if math.Abs(vol) > 1e-9 {
		t.Errorf("volatility of flat prices = %v, want ~0", vol)
	}
}

func TestLSTM_CalculateVolatility_NonNegative(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{100, 102, 98, 105, 95, 110, 90, 108, 97, 103, 101, 99, 104}
	vol := l.calculateVolatility(prices, 12, 10)
	if vol < 0 {
		t.Errorf("volatility should be >= 0, got %v", vol)
	}
}

func TestLSTM_CalculateVolatility_InsufficientData_ReturnsZero(t *testing.T) {
	l := newLSTMPredictor()
	prices := []float64{100, 102, 104}
	vol := l.calculateVolatility(prices, 2, 10)
	// index(2) < period(10) → returns 0
	if vol != 0 {
		t.Errorf("volatility with insufficient data = %v, want 0", vol)
	}
}

// ---- extractFeatures ----

func TestLSTM_ExtractFeatures_Length(t *testing.T) {
	l := newLSTMPredictor()
	// Need at least sequenceLen=60 prices
	prices := make([]float64, 70)
	for i := range prices {
		prices[i] = 100.0 + float64(i)
	}
	features := l.extractFeatures(prices, 65)
	if len(features) != l.numFeatures {
		t.Errorf("features length = %d, want %d", len(features), l.numFeatures)
	}
}

func TestLSTM_ExtractFeatures_NormalizedPrice(t *testing.T) {
	l := newLSTMPredictor()
	prices := make([]float64, 70)
	for i := range prices {
		prices[i] = 100.0 // flat
	}
	features := l.extractFeatures(prices, 65)
	// Feature 0: normalized = p / base = 100 / 100 = 1.0
	if math.Abs(features[0]-1.0) > 1e-9 {
		t.Errorf("normalized price feature = %v, want 1.0", features[0])
	}
}

// ---- train and predict ----

func TestLSTM_TrainAndPredict_NonZero(t *testing.T) {
	l := newLSTMPredictor()
	// Build a simple dataset
	n := 20
	X := make([][]float64, n)
	Y := make([]float64, n)
	for i := range X {
		X[i] = []float64{1.0, 0.01, 0.02, 0.5, 1.0, 0.001}
		Y[i] = 100.0 + float64(i)
	}
	l.train(X, Y)
	if !l.trained {
		t.Error("predictor should be marked as trained")
	}
	features := []float64{1.0, 0.01, 0.02, 0.5, 1.0, 0.001}
	result := l.predict(features)
	if result == 0 {
		t.Error("predict after training should return non-zero")
	}
}

func TestLSTM_Predict_NotTrained_ReturnsZero(t *testing.T) {
	l := newLSTMPredictor()
	features := []float64{1.0, 0.01, 0.02, 0.5, 1.0, 0.001}
	result := l.predict(features)
	if result != 0 {
		t.Errorf("predict without training = %v, want 0", result)
	}
}

// ---- Predict (main method) ----

func TestLSTMPredict_InsufficientData_ReturnsError(t *testing.T) {
	l := newLSTMPredictor()
	// Only 30 prices, need 60
	historical := make([]string, 30)
	for i := range historical {
		historical[i] = "100"
	}
	data := &modelssvc.StockData{Historical: historical}
	_, err := l.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for insufficient data, got nil")
	}
}

func TestLSTMPredict_EmptyData_ReturnsError(t *testing.T) {
	l := newLSTMPredictor()
	data := &modelssvc.StockData{Historical: []string{}}
	_, err := l.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for empty data, got nil")
	}
}

func TestLSTMPredict_InvalidData_ReturnsError(t *testing.T) {
	l := newLSTMPredictor()
	data := &modelssvc.StockData{Historical: []string{"not_a_number"}}
	_, err := l.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for invalid data, got nil")
	}
}

func TestLSTMPredict_SufficientData_ReturnsPrediction(t *testing.T) {
	l := newLSTMPredictor()
	// 75 prices with slight variation
	historical := make([]string, 75)
	for i := range historical {
		// Use ascending prices to provide meaningful training signal
		historical[i] = "100"
	}
	data := &modelssvc.StockData{Historical: historical}
	result, err := l.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.CurrentPrice != 100.0 {
		t.Errorf("current price = %v, want 100.0", result.CurrentPrice)
	}
}

func TestLSTMPredict_PriceBounded_WithinDailyLimit(t *testing.T) {
	l := newLSTMPredictor()
	historical := make([]string, 75)
	for i := range historical {
		historical[i] = "100"
	}
	data := &modelssvc.StockData{Historical: historical}
	result, err := l.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Max 7% daily change from 100
	if result.PredictedPrice > 107.5 || result.PredictedPrice < 92.5 {
		t.Errorf("predicted price %v outside ±7%% of 100", result.PredictedPrice)
	}
}

// ---- GetName / GetAccuracy ----

func TestLSTMGetName(t *testing.T) {
	l := newLSTMPredictor()
	if l.GetName() == "" {
		t.Error("GetName should return non-empty string")
	}
}

func TestLSTMGetAccuracy_ZeroBeforeBacktest(t *testing.T) {
	l := newLSTMPredictor()
	if l.GetAccuracy() != 0.0 {
		t.Errorf("GetAccuracy before backtest = %v, want 0.0", l.GetAccuracy())
	}
}

// ---- Phase 1 regression: confidence and CurrentPrice fixes ----

func TestLSTMPredict_ConfidenceGreaterThanZero(t *testing.T) {
	l := newLSTMPredictor()
	// Need at least 60 data points (sequenceLen) plus extras for training
	historical := make([]string, 100)
	for i := range historical {
		historical[i] = fmt.Sprintf("%f", 50000.0+float64(i)*100.0)
	}
	data := &modelssvc.StockData{Historical: historical}
	result, err := l.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence <= 0 {
		t.Errorf("Confidence should be > 0 after training, got %v", result.Confidence)
	}
	if result.Confidence > 1.0 {
		t.Errorf("Confidence should be <= 1.0, got %v", result.Confidence)
	}
}

func TestLSTMPredict_GetAccuracyAfterPredict(t *testing.T) {
	l := newLSTMPredictor()
	historical := make([]string, 100)
	for i := range historical {
		historical[i] = fmt.Sprintf("%f", 50000.0+float64(i)*100.0)
	}
	data := &modelssvc.StockData{Historical: historical}
	_, err := l.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	accuracy := l.GetAccuracy()
	if accuracy <= 0 {
		t.Errorf("GetAccuracy() after Predict should be > 0, got %v", accuracy)
	}
}

func TestLSTMPredict_CurrentPriceSet(t *testing.T) {
	l := newLSTMPredictor()
	historical := make([]string, 100)
	for i := range historical {
		historical[i] = "50000"
	}
	data := &modelssvc.StockData{Historical: historical}
	result, err := l.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentPrice != 50000 {
		t.Errorf("CurrentPrice should be 50000, got %v", result.CurrentPrice)
	}
}
