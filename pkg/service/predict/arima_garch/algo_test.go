package arimagarch

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"testing"
)

func newARIMAPredictor() *ARIMAGARCHPredictor {
	return NewARIMAGARCHPredictor()
}

// ---- parseHistoricalData ----

func TestARIMA_ParseHistoricalData_ValidStrings(t *testing.T) {
	a := newARIMAPredictor()
	input := []string{"100", "110.5", "120,000"}
	prices, err := a.parseHistoricalData(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 3 {
		t.Fatalf("expected 3 prices, got %d", len(prices))
	}
	if math.Abs(prices[0]-100.0) > 1e-9 {
		t.Errorf("prices[0] = %v, want 100", prices[0])
	}
	if math.Abs(prices[1]-110.5) > 1e-9 {
		t.Errorf("prices[1] = %v, want 110.5", prices[1])
	}
}

func TestARIMA_ParseHistoricalData_InvalidString_ReturnsError(t *testing.T) {
	a := newARIMAPredictor()
	_, err := a.parseHistoricalData([]string{"100", "bad_value"})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestARIMA_ParseHistoricalData_Empty(t *testing.T) {
	a := newARIMAPredictor()
	prices, err := a.parseHistoricalData([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prices) != 0 {
		t.Errorf("expected empty slice, got %v", prices)
	}
}

// ---- calculateReturns ----

func TestCalculateReturns_TwoPrices(t *testing.T) {
	a := newARIMAPredictor()
	prices := []float64{100, 110}
	returns := a.calculateReturns(prices)
	if len(returns) != 1 {
		t.Fatalf("expected 1 return, got %d", len(returns))
	}
	// ln(110/100) ≈ 0.09531
	want := math.Log(110.0 / 100.0)
	if math.Abs(returns[0]-want) > 1e-9 {
		t.Errorf("return = %v, want %v", returns[0], want)
	}
}

func TestCalculateReturns_MultiplePrices(t *testing.T) {
	a := newARIMAPredictor()
	prices := []float64{100, 105, 110, 108}
	returns := a.calculateReturns(prices)
	if len(returns) != 3 {
		t.Fatalf("expected 3 returns, got %d", len(returns))
	}
	// All should be valid finite numbers
	for i, r := range returns {
		if math.IsNaN(r) || math.IsInf(r, 0) {
			t.Errorf("return[%d] is NaN or Inf: %v", i, r)
		}
	}
}

func TestCalculateReturns_ZeroPriceHandled(t *testing.T) {
	a := newARIMAPredictor()
	prices := []float64{100, 0, 110}
	returns := a.calculateReturns(prices)
	// Zero prices should produce 0 returns
	if math.IsNaN(returns[0]) || math.IsInf(returns[0], 0) {
		t.Errorf("return with zero price should not be NaN/Inf: %v", returns[0])
	}
}

func TestCalculateReturns_SinglePrice_EmptyReturns(t *testing.T) {
	a := newARIMAPredictor()
	prices := []float64{100}
	returns := a.calculateReturns(prices)
	if len(returns) != 0 {
		t.Errorf("expected 0 returns for 1 price, got %d", len(returns))
	}
}

// ---- fitAR ----

func TestFitAR_WhiteNoise_CoefficientsSmall(t *testing.T) {
	a := newARIMAPredictor()
	// Generate white noise returns
	n := 100
	returns := make([]float64, n)
	for i := range returns {
		returns[i] = 0.001 * float64(i%7-3) // small oscillating values
	}
	coeffs := a.fitAR(returns, 2)
	if len(coeffs) != 2 {
		t.Fatalf("expected 2 coefficients, got %d", len(coeffs))
	}
	for i, c := range coeffs {
		if math.Abs(c) >= 1.0 {
			t.Errorf("AR coefficient[%d] = %v, expected |c| < 1.0", i, c)
		}
	}
}

func TestFitAR_InsufficientData_ReturnsZeroCoeffs(t *testing.T) {
	a := newARIMAPredictor()
	returns := []float64{0.01, 0.02} // only 2, p=2 → n<=p
	coeffs := a.fitAR(returns, 2)
	if len(coeffs) != 2 {
		t.Fatalf("expected 2 coefficients, got %d", len(coeffs))
	}
	for _, c := range coeffs {
		if c != 0 {
			t.Errorf("expected zero coeffs for insufficient data, got %v", c)
		}
	}
}

// ---- fitGARCHMoments ----

func TestFitGARCHMoments_ValidResiduals(t *testing.T) {
	a := newARIMAPredictor()
	residuals := make([]float64, 50)
	for i := range residuals {
		residuals[i] = 0.01 * float64(i%5-2)
	}
	alpha, beta, omega := a.fitGARCHMoments(residuals)

	if alpha+beta >= 1.0 {
		t.Errorf("alpha(%v) + beta(%v) should be < 1.0", alpha, beta)
	}
	if omega <= 0 {
		t.Errorf("omega should be > 0, got %v", omega)
	}
	if math.IsNaN(alpha) || math.IsNaN(beta) || math.IsNaN(omega) {
		t.Errorf("GARCH params contain NaN: alpha=%v beta=%v omega=%v", alpha, beta, omega)
	}
}

func TestFitGARCHMoments_OmegaPositive(t *testing.T) {
	a := newARIMAPredictor()
	// Residuals with larger variance to force omega calculation
	residuals := []float64{0.05, -0.05, 0.05, -0.05, 0.05, -0.05, 0.05, -0.05, 0.05, -0.05}
	_, _, omega := a.fitGARCHMoments(residuals)
	if omega <= 0 {
		t.Errorf("omega should be > 0, got %v", omega)
	}
}

// ---- getResiduals ----

func TestGetResiduals_ZeroARCoeffs_ReturnsReturns(t *testing.T) {
	a := newARIMAPredictor()
	returns := []float64{0.01, 0.02, 0.03, 0.04, 0.05}
	model := &ARIMAModel{
		ar: []float64{0, 0}, // zero AR coefficients
		ma: []float64{0, 0},
	}
	residuals := a.getResiduals(returns, model)
	if len(residuals) != len(returns) {
		t.Fatalf("expected %d residuals, got %d", len(returns), len(residuals))
	}
	// With zero AR, predictions are 0, so residuals[i] = returns[i] for i >= p
	p := len(model.ar)
	for i := p; i < len(returns); i++ {
		if math.Abs(residuals[i]-returns[i]) > 1e-9 {
			t.Errorf("residuals[%d] = %v, want %v (with zero AR)", i, residuals[i], returns[i])
		}
	}
}

func TestGetResiduals_Length(t *testing.T) {
	a := newARIMAPredictor()
	returns := make([]float64, 50)
	for i := range returns {
		returns[i] = 0.001 * float64(i)
	}
	model := &ARIMAModel{ar: []float64{0.1, 0.2}, ma: []float64{}}
	residuals := a.getResiduals(returns, model)
	if len(residuals) != len(returns) {
		t.Errorf("residuals length = %d, want %d", len(residuals), len(returns))
	}
}

// ---- Predict ----

func TestARIMAPredict_InsufficientData_ReturnsError(t *testing.T) {
	a := newARIMAPredictor()
	// 50 prices, need 100
	historical := make([]string, 50)
	for i := range historical {
		historical[i] = "100"
	}
	data := &modelssvc.StockData{Historical: historical}
	_, err := a.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for insufficient data, got nil")
	}
}

func TestARIMAPredict_EmptyData_ReturnsError(t *testing.T) {
	a := newARIMAPredictor()
	data := &modelssvc.StockData{Historical: []string{}}
	_, err := a.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for empty data, got nil")
	}
}

func TestARIMAPredict_InvalidData_ReturnsError(t *testing.T) {
	a := newARIMAPredictor()
	data := &modelssvc.StockData{Historical: []string{"abc"}}
	_, err := a.Predict(context.Background(), data)
	if err == nil {
		t.Error("expected error for invalid data, got nil")
	}
}

func TestARIMAPredict_SufficientData_ReturnsPrediction(t *testing.T) {
	a := newARIMAPredictor()
	// 110 prices around 100
	historical := make([]string, 110)
	for i := range historical {
		historical[i] = "100"
	}
	data := &modelssvc.StockData{Historical: historical}
	result, err := a.Predict(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PredictedPrice <= 0 {
		t.Errorf("predicted price = %v, want > 0", result.PredictedPrice)
	}
	if result.Confidence < 0.3 || result.Confidence > 0.9 {
		t.Errorf("confidence = %v, expected in [0.3, 0.9]", result.Confidence)
	}
}

// ---- returnToPrice ----

func TestReturnToPrice_PositiveReturn(t *testing.T) {
	a := newARIMAPredictor()
	// exp(0) = 1 → same price
	result := a.returnToPrice(100.0, 0.0)
	if math.Abs(result-100.0) > 1e-9 {
		t.Errorf("returnToPrice(100, 0) = %v, want 100", result)
	}
}

func TestReturnToPrice_PositiveReturnValue(t *testing.T) {
	a := newARIMAPredictor()
	// ln(110/100) → returnToPrice(100, ln(1.1)) ≈ 110
	ret := math.Log(1.1)
	result := a.returnToPrice(100.0, ret)
	if math.Abs(result-110.0) > 1e-6 {
		t.Errorf("returnToPrice(100, ln(1.1)) = %v, want ≈110", result)
	}
}

// ---- GetName / GetAccuracy ----

func TestARIMAGetName(t *testing.T) {
	a := newARIMAPredictor()
	if a.GetName() == "" {
		t.Error("GetName should return non-empty string")
	}
}

func TestARIMAGetAccuracy_ZeroBeforeBacktest(t *testing.T) {
	a := newARIMAPredictor()
	if a.GetAccuracy() != 0.0 {
		t.Errorf("GetAccuracy before backtest = %v, want 0.0", a.GetAccuracy())
	}
}
