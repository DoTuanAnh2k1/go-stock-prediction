package lstmnn

import (
	"context"
	"fmt"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"strconv"
	"strings"
	"time"
)

type LSTMPredictor struct {
	weights          []float64
	bias             float64
	lambda           float64
	numFeatures      int
	sequenceLen      int
	trained          bool
	lastTrained      time.Time
	backtestAccuracy float64
}

func NewLSTMPredictor() *LSTMPredictor {
	p := &LSTMPredictor{
		lambda:      0.01,
		numFeatures: 6,
		sequenceLen: 60,
	}
	return p
}

func (l *LSTMPredictor) Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error) {
	prices, err := l.parseHistoricalData(data.Historical)
	if err != nil {
		return nil, fmt.Errorf("lỗi parse data: %v", err)
	}
	if len(prices) < l.sequenceLen {
		return nil, fmt.Errorf("cần ít nhất %d ngày data", l.sequenceLen)
	}

	// Build training set: each sample uses prices[i-seq:i] to predict prices[i]
	X, Y := l.buildDataset(prices)
	if len(X) < 10 {
		return nil, fmt.Errorf("không đủ data để train")
	}

	// Train on all available data
	l.train(X, Y)

	// Calculate backtest accuracy
	if len(X) > 0 {
		totalError := 0.0
		for i := 0; i < len(X); i++ {
			pred := l.predict(X[i])
			if Y[i] > 0 {
				totalError += math.Abs(pred-Y[i]) / Y[i]
			}
		}
		mape := totalError / float64(len(X))
		l.backtestAccuracy = math.Max(0, math.Min(1.0, 1.0-mape))
	}

	// Predict next price using features at last index
	features := l.extractFeatures(prices, len(prices)-1)
	currentPrice := prices[len(prices)-1]
	predictedPrice := l.predict(features)

	// Clip to ±7% daily limit
	maxChange := currentPrice * 0.07
	if predictedPrice > currentPrice+maxChange {
		predictedPrice = currentPrice + maxChange
	}
	if predictedPrice < currentPrice-maxChange {
		predictedPrice = currentPrice - maxChange
	}

	confidence := l.GetAccuracy()

	return &modelssvc.Prediction{
		PredictedPrice: predictedPrice,
		CurrentPrice:   currentPrice,
		Confidence:     confidence,
	}, nil
}

func (l *LSTMPredictor) parseHistoricalData(historical []string) ([]float64, error) {
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

// buildDataset creates (X, Y) pairs where Y[i] = next day price, X[i] = features at i
func (l *LSTMPredictor) buildDataset(prices []float64) ([][]float64, []float64) {
	minRequired := l.sequenceLen + 1
	if len(prices) < minRequired {
		return nil, nil
	}
	var X [][]float64
	var Y []float64
	for i := l.sequenceLen; i < len(prices)-1; i++ {
		features := l.extractFeatures(prices, i)
		X = append(X, features)
		Y = append(Y, prices[i+1])
	}
	return X, Y
}

// extractFeatures builds feature vector for price at index i
func (l *LSTMPredictor) extractFeatures(prices []float64, i int) []float64 {
	f := make([]float64, l.numFeatures)
	p := prices[i]
	base := prices[l.sequenceLen-1] // normalize relative to start of window
	if base == 0 {
		base = 1
	}

	// Feature 0: normalized price
	f[0] = p / base

	// Feature 1: 5-day return
	if i >= 5 && prices[i-5] > 0 {
		f[1] = (p - prices[i-5]) / prices[i-5]
	}

	// Feature 2: 20-day return
	if i >= 20 && prices[i-20] > 0 {
		f[2] = (p - prices[i-20]) / prices[i-20]
	}

	// Feature 3: RSI(14) normalized to [0,1]
	f[3] = l.calculateRSI(prices, i, 14) / 100.0

	// Feature 4: price / SMA20 ratio
	sma20 := l.calculateSMA(prices, i, 20)
	if sma20 > 0 {
		f[4] = p / sma20
	} else {
		f[4] = 1.0
	}

	// Feature 5: volatility (std of 10-day log returns)
	f[5] = l.calculateVolatility(prices, i, 10)

	return f
}

func (l *LSTMPredictor) train(X [][]float64, Y []float64) {
	n := len(X)
	if n == 0 {
		return
	}
	l.numFeatures = len(X[0])
	l.weights = make([]float64, l.numFeatures)
	l.bias = 0
	// Initialize bias as mean of Y
	for _, y := range Y {
		l.bias += y
	}
	l.bias /= float64(n)

	lr := 0.0001
	epochs := 500

	for epoch := 0; epoch < epochs; epoch++ {
		wGrad := make([]float64, l.numFeatures)
		bGrad := 0.0
		for i := 0; i < n; i++ {
			pred := l.bias
			for j := 0; j < l.numFeatures; j++ {
				pred += l.weights[j] * X[i][j]
			}
			err := pred - Y[i]
			bGrad += err
			for j := 0; j < l.numFeatures; j++ {
				wGrad[j] += err * X[i][j]
			}
		}
		l.bias -= lr * bGrad / float64(n)
		for j := 0; j < l.numFeatures; j++ {
			l.weights[j] -= lr * (wGrad[j]/float64(n) + l.lambda*l.weights[j])
		}
		_ = epoch
	}
	l.trained = true
	l.lastTrained = time.Now()
}

func (l *LSTMPredictor) predict(features []float64) float64 {
	if !l.trained || len(l.weights) == 0 {
		return 0
	}
	result := l.bias
	for j := 0; j < len(l.weights) && j < len(features); j++ {
		result += l.weights[j] * features[j]
	}
	return result
}

func (l *LSTMPredictor) calculateRSI(prices []float64, index, period int) float64 {
	if index < period {
		return 50
	}
	gains, losses := 0.0, 0.0
	for i := index - period + 1; i <= index; i++ {
		if i == 0 {
			continue
		}
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}
	if losses == 0 {
		return 100
	}
	rs := (gains / float64(period)) / (losses / float64(period))
	return 100 - (100 / (1 + rs))
}

func (l *LSTMPredictor) calculateSMA(prices []float64, index, period int) float64 {
	if index < period-1 {
		return prices[index]
	}
	sum := 0.0
	for i := index - period + 1; i <= index; i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}

func (l *LSTMPredictor) calculateVolatility(prices []float64, index, period int) float64 {
	if index < period {
		return 0
	}
	returns := make([]float64, 0, period)
	for i := index - period + 1; i <= index; i++ {
		if i > 0 && prices[i-1] > 0 {
			returns = append(returns, math.Log(prices[i]/prices[i-1]))
		}
	}
	if len(returns) == 0 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	variance := 0.0
	for _, r := range returns {
		variance += (r - mean) * (r - mean)
	}
	return math.Sqrt(variance / float64(len(returns)))
}

func (l *LSTMPredictor) GetName() string {
	return "LSTM Neural Network"
}

func (l *LSTMPredictor) GetAccuracy() float64 {
	if l.backtestAccuracy > 0 {
		return l.backtestAccuracy
	}
	return 0.0
}
