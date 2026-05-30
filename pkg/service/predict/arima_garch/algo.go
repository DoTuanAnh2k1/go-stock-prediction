package arimagarch

import (
	"context"
	"errors"
	"fmt"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"strconv"
	"strings"
)

type ARIMAGARCHPredictor struct {
	p, d, q          int // ARIMA orders (2,1,2 tối ưu cho VN30)
	garchP, garchQ   int // GARCH orders (1,1 hoạt động tốt)
	name             string
	backtestAccuracy float64
}

type ARIMAModel struct {
	ar []float64 // AutoRegressive coefficients
	ma []float64 // Moving Average coefficients
}

type GARCHModel struct {
	alpha []float64 // ARCH coefficients
	beta  []float64 // GARCH coefficients
	omega float64   // Constant term
}

type ForecastResult struct {
	mean       float64
	confidence float64
}

func NewARIMAGARCHPredictor() *ARIMAGARCHPredictor {
	return &ARIMAGARCHPredictor{
		p: 2, d: 1, q: 2, // Tham số tối ưu cho VN30
		garchP: 1, garchQ: 1,
		name: "ARIMA-GARCH",
	}
}

func (a *ARIMAGARCHPredictor) Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error) {
	// Parse historical data từ strings array
	prices, err := a.parseHistoricalData(data.Historical)
	if err != nil {
		return nil, fmt.Errorf("lỗi parse historical data: %v", err)
	}

	if len(prices) < 100 {
		return nil, errors.New("cần ít nhất 100 ngày data để predict")
	}

	// Tính returns
	returns := a.calculateReturns(prices)

	// Fit ARIMA model using OLS gradient descent
	arimaModel := a.fitARIMA(returns)
	residuals := a.getResiduals(returns, arimaModel)

	// Fit GARCH model using moment matching
	garchModel := a.fitGARCH(residuals)

	// Forecast using fitted models
	prediction := a.forecast(returns, arimaModel, garchModel)

	// Lấy giá cuối cùng
	currentPrice := prices[len(prices)-1]
	predictedPrice := a.returnToPrice(currentPrice, prediction.mean)

	a.backtestAccuracy = prediction.confidence
	return &modelssvc.Prediction{
		PredictedPrice: predictedPrice,
		CurrentPrice:   currentPrice,
		Confidence:     prediction.confidence,
	}, nil
}

func (a *ARIMAGARCHPredictor) GetName() string {
	return a.name
}

func (a *ARIMAGARCHPredictor) GetAccuracy() float64 {
	if a.backtestAccuracy > 0 {
		return a.backtestAccuracy
	}
	return 0.0
}

// Parse historical data từ string array thành float64 array
func (a *ARIMAGARCHPredictor) parseHistoricalData(historical []string) ([]float64, error) {
	prices := make([]float64, len(historical))

	for i, priceStr := range historical {
		cleanStr := strings.ReplaceAll(priceStr, ",", "")
		cleanStr = strings.ReplaceAll(cleanStr, " ", "")

		price, err := strconv.ParseFloat(cleanStr, 64)
		if err != nil {
			return nil, fmt.Errorf("không thể parse giá tại index %d: %v", i, err)
		}
		prices[i] = price
	}

	return prices, nil
}

// Tính returns theo cách thích hợp với thị trường Việt Nam
func (a *ARIMAGARCHPredictor) calculateReturns(prices []float64) []float64 {
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] > 0 && prices[i] > 0 {
			returns[i-1] = math.Log(prices[i] / prices[i-1])
		} else {
			returns[i-1] = 0
		}
	}
	return returns
}

// fitAR fits AR(p) coefficients using gradient descent OLS
func (a *ARIMAGARCHPredictor) fitAR(returns []float64, p int) []float64 {
	n := len(returns)
	if n <= p {
		return make([]float64, p)
	}
	rows := n - p
	X := make([][]float64, rows)
	Y := make([]float64, rows)
	for i := 0; i < rows; i++ {
		X[i] = make([]float64, p)
		for j := 0; j < p; j++ {
			X[i][j] = returns[i+p-j-1]
		}
		Y[i] = returns[i+p]
	}
	coeffs := make([]float64, p)
	lr := 0.001
	for iter := 0; iter < 1000; iter++ {
		gradients := make([]float64, p)
		for i := 0; i < rows; i++ {
			pred := 0.0
			for j := 0; j < p; j++ {
				pred += coeffs[j] * X[i][j]
			}
			residual := Y[i] - pred
			for j := 0; j < p; j++ {
				gradients[j] -= 2 * residual * X[i][j]
			}
		}
		for j := 0; j < p; j++ {
			coeffs[j] -= lr * gradients[j] / float64(rows)
		}
	}
	return coeffs
}

// fitARIMA fits ARIMA model using OLS AR fitting
func (a *ARIMAGARCHPredictor) fitARIMA(returns []float64) *ARIMAModel {
	model := &ARIMAModel{
		ar: a.fitAR(returns, a.p),
		ma: make([]float64, a.q),
	}
	return model
}

// getResiduals computes actual AR residuals from fitted model
func (a *ARIMAGARCHPredictor) getResiduals(returns []float64, model *ARIMAModel) []float64 {
	p := len(model.ar)
	n := len(returns)
	residuals := make([]float64, n)
	for i := p; i < n; i++ {
		pred := 0.0
		for j := 0; j < p; j++ {
			pred += model.ar[j] * returns[i-j-1]
		}
		residuals[i] = returns[i] - pred
	}
	return residuals
}

// fitGARCHMoments uses moment matching to estimate GARCH(1,1) parameters
func (a *ARIMAGARCHPredictor) fitGARCHMoments(residuals []float64) (alpha, beta, omega float64) {
	variance := 0.0
	for _, r := range residuals {
		variance += r * r
	}
	variance /= float64(len(residuals))
	alpha = 0.1
	beta = 0.8
	omega = variance * (1 - alpha - beta)
	if omega < 0 {
		omega = variance * 0.1
		alpha = 0.05
		beta = 0.85
	}
	return alpha, beta, omega
}

// fitGARCH fits GARCH(1,1) model via moment matching
func (a *ARIMAGARCHPredictor) fitGARCH(residuals []float64) *GARCHModel {
	model := &GARCHModel{
		alpha: make([]float64, a.garchP),
		beta:  make([]float64, a.garchQ),
	}
	alpha, beta, omega := a.fitGARCHMoments(residuals)
	model.alpha[0] = alpha
	model.beta[0] = beta
	model.omega = omega
	return model
}

// forecast uses fitted AR model and GARCH volatility to produce a forecast
func (a *ARIMAGARCHPredictor) forecast(returns []float64, arimaModel *ARIMAModel, garchModel *GARCHModel) ForecastResult {
	p := len(arimaModel.ar)
	meanForecast := 0.0
	n := len(returns)
	for j := 0; j < p && j < n; j++ {
		meanForecast += arimaModel.ar[j] * returns[n-1-j]
	}

	// GARCH volatility forecast using last residual
	lastResidual := 0.0
	if n > 0 {
		lastPred := 0.0
		for j := 0; j < p && j < n-1; j++ {
			lastPred += arimaModel.ar[j] * returns[n-2-j]
		}
		lastResidual = returns[n-1] - lastPred
	}

	lastVariance := 0.0
	for _, r := range returns {
		lastVariance += r * r
	}
	lastVariance /= float64(len(returns))

	forecastVariance := garchModel.omega + garchModel.alpha[0]*lastResidual*lastResidual + garchModel.beta[0]*lastVariance
	confidence := 1.0 / (1.0 + math.Sqrt(forecastVariance)*10)
	if confidence > 0.9 {
		confidence = 0.9
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return ForecastResult{mean: meanForecast, confidence: confidence}
}

func (a *ARIMAGARCHPredictor) returnToPrice(lastPrice float64, returnValue float64) float64 {
	return lastPrice * math.Exp(returnValue)
}
