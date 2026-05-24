package movingaverage

import (
	"context"
	"fmt"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// parseVolumes parses volume strings into float64 slice.
// Returns empty slice on any error rather than failing hard.
func (m *MovingAveragePredictor) parseVolumes(volumeStrs []string) []float64 {
	if len(volumeStrs) == 0 {
		return nil
	}
	volumes := make([]float64, len(volumeStrs))
	for i, volStr := range volumeStrs {
		cleanStr := strings.ReplaceAll(volStr, ",", "")
		cleanStr = strings.ReplaceAll(cleanStr, " ", "")
		vol, err := strconv.ParseFloat(cleanStr, 64)
		if err != nil {
			return nil
		}
		volumes[i] = vol
	}
	return volumes
}

type MovingAveragePredictor struct {
	shortPeriod      int        // 5 ngày cho pattern tuần của VN
	longPeriod       int        // 20 ngày cho pattern tháng
	volumeWeight     bool       // Cực kỳ quan trọng cho thị trường VN
	rng              *rand.Rand
	backtestAccuracy float64
}

type Signal struct {
	Direction string  // "BUY", "SELL", "HOLD"
	Strength  float64 // Độ mạnh của signal [0-1]
}

func NewMovingAveragePredictor() *MovingAveragePredictor {
	return &MovingAveragePredictor{
		shortPeriod:  5,  // 1 tuần giao dịch
		longPeriod:   20, // 1 tháng giao dịch
		volumeWeight: true,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *MovingAveragePredictor) Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error) {
	// Parse historical data
	prices, err := m.parseHistoricalData(data.Historical)
	if err != nil {
		return nil, fmt.Errorf("lỗi parse data: %v", err)
	}

	if len(prices) < m.longPeriod {
		return nil, fmt.Errorf("cần ít nhất %d ngày data", m.longPeriod)
	}

	// Parse volume data (may be empty if not provided)
	volumes := m.parseVolumes(data.Volume)

	// Tính MA có trọng số volume
	shortMA := m.calculateVWMA(prices, volumes, m.shortPeriod)
	longMA := m.calculateVWMA(prices, volumes, m.longPeriod)

	// Get current price
	currentPrice := prices[len(prices)-1]

	// Generate signal
	signal := m.generateSignal(shortMA, longMA, currentPrice)

	// Điều chỉnh cho phiên giao dịch VN (sáng/chiều)
	confidence := m.adjustForVietnameseSession(signal)

	// Dự đoán giá
	predictedPrice := m.calculatePredictedPrice(currentPrice, signal, confidence)

	return &modelssvc.Prediction{
		PredictedPrice: predictedPrice,
		CurrentPrice:   currentPrice, // <- Thêm field này!
	}, nil
}

func (m *MovingAveragePredictor) parseHistoricalData(historical []string) ([]float64, error) {
	prices := make([]float64, len(historical))

	for i, priceStr := range historical {
		// Clean Vietnamese number format
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

// Volume-Weighted Moving Average phù hợp với VN
// Falls back to SMA if volumes are empty or unavailable.
func (m *MovingAveragePredictor) calculateVWMA(prices, volumes []float64, period int) float64 {
	if len(prices) < period {
		return m.calculateSMA(prices, period)
	}
	start := len(prices) - period
	totalWeightedPrice := 0.0
	totalVolume := 0.0
	for i := start; i < len(prices); i++ {
		vol := 1.0
		if i < len(volumes) && volumes[i] > 0 {
			vol = volumes[i]
		}
		totalWeightedPrice += prices[i] * vol
		totalVolume += vol
	}
	if totalVolume == 0 {
		return m.calculateSMA(prices, period)
	}
	return totalWeightedPrice / totalVolume
}

func (m *MovingAveragePredictor) calculateSMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	startIdx := len(prices) - period
	sum := 0.0

	for i := startIdx; i < len(prices); i++ {
		sum += prices[i]
	}

	return sum / float64(period)
}


func (m *MovingAveragePredictor) generateSignal(shortMA, longMA, currentPrice float64) Signal {
	signal := Signal{
		Direction: "HOLD",
		Strength:  0.5,
	}

	if shortMA == 0 || longMA == 0 {
		return signal
	}

	// Calculate signal strength based on MA difference
	maDiff := (shortMA - longMA) / longMA * 100 // Percentage difference

	// Determine direction
	if shortMA > longMA {
		signal.Direction = "BUY"
		signal.Strength = math.Min(1.0, 0.5+math.Abs(maDiff)/10)
	} else if shortMA < longMA {
		signal.Direction = "SELL"
		signal.Strength = math.Min(1.0, 0.5+math.Abs(maDiff)/10)
	}

	// Adjust based on current price position relative to MAs
	if currentPrice > shortMA && currentPrice > longMA {
		if signal.Direction == "BUY" {
			signal.Strength *= 1.2 // Strengthen buy signal
		} else {
			signal.Strength *= 0.8 // Weaken sell signal
		}
	} else if currentPrice < shortMA && currentPrice < longMA {
		if signal.Direction == "SELL" {
			signal.Strength *= 1.2 // Strengthen sell signal
		} else {
			signal.Strength *= 0.8 // Weaken buy signal
		}
	}

	// Ensure strength stays within bounds
	if signal.Strength > 1.0 {
		signal.Strength = 1.0
	}
	if signal.Strength < 0.1 {
		signal.Strength = 0.1
	}

	return signal
}

func (m *MovingAveragePredictor) adjustForVietnameseSession(signal Signal) float64 {
	// Điều chỉnh confidence dựa trên đặc điểm thị trường VN
	baseConfidence := signal.Strength * 0.75 // Base confidence từ signal strength

	// Thị trường VN có đặc điểm:
	// - Phiên sáng thường có volume cao hơn
	// - Biến động mạnh vào đầu và cuối phiên
	currentHour := time.Now().Hour()

	// Điều chỉnh theo giờ giao dịch
	var timeAdjustment float64
	if currentHour >= 9 && currentHour <= 11 {
		// Phiên sáng - tăng confidence
		timeAdjustment = 0.1
	} else if currentHour >= 13 && currentHour <= 14 {
		// Phiên chiều - confidence bình thường
		timeAdjustment = 0.05
	} else {
		// Ngoài giờ giao dịch - giảm confidence
		timeAdjustment = -0.05
	}

	adjustedConfidence := baseConfidence + timeAdjustment

	// Ensure confidence stays within reasonable bounds
	if adjustedConfidence > 0.9 {
		adjustedConfidence = 0.9
	}
	if adjustedConfidence < 0.3 {
		adjustedConfidence = 0.3
	}

	return adjustedConfidence
}

func (m *MovingAveragePredictor) calculatePredictedPrice(currentPrice float64, signal Signal, confidence float64) float64 {
	// Base prediction starts with current price
	predictedPrice := currentPrice

	// Calculate price movement based on signal
	var priceMovement float64

	switch signal.Direction {
	case "BUY":
		// Predict price increase
		priceMovement = currentPrice * signal.Strength * 0.02 * confidence // Max 2% increase
	case "SELL":
		// Predict price decrease
		priceMovement = -currentPrice * signal.Strength * 0.02 * confidence // Max 2% decrease
	default: // "HOLD"
		// Predict minimal movement
		priceMovement = currentPrice * (signal.Strength - 0.5) * 0.005 // Max 0.5% movement
	}

	predictedPrice += priceMovement

	// Add market noise - Vietnamese market characteristics
	noiseRange := currentPrice * 0.001 * (1 - confidence) // Noise decreases with confidence
	noise := (m.random() - 0.5) * 2 * noiseRange          // Random noise in range

	predictedPrice += noise

	// Ensure predicted price is reasonable (within daily limits)
	maxDailyChange := currentPrice * 0.07 // 7% daily limit for HOSE

	if predictedPrice > currentPrice+maxDailyChange {
		predictedPrice = currentPrice + maxDailyChange
	}
	if predictedPrice < currentPrice-maxDailyChange {
		predictedPrice = currentPrice - maxDailyChange
	}

	// Ensure price doesn't go negative
	if predictedPrice < 0 {
		predictedPrice = currentPrice * 0.01 // Minimum 1% of current price
	}

	return predictedPrice
}

// Simple random number generator
func (m *MovingAveragePredictor) random() float64 {
	return m.rng.Float64()
}

// Calculate additional technical indicators for enhanced prediction
func (m *MovingAveragePredictor) calculateMomentum(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}

	currentPrice := prices[len(prices)-1]
	pastPrice := prices[len(prices)-1-period]

	if pastPrice == 0 {
		return 0
	}

	momentum := (currentPrice - pastPrice) / pastPrice
	return momentum
}

func (m *MovingAveragePredictor) calculateBollingerBands(prices []float64, period int) (float64, float64, float64) {
	if len(prices) < period {
		return 0, 0, 0
	}

	// Calculate SMA
	sma := m.calculateSMA(prices, period)

	// Calculate standard deviation
	startIdx := len(prices) - period
	variance := 0.0

	for i := startIdx; i < len(prices); i++ {
		variance += math.Pow(prices[i]-sma, 2)
	}
	variance /= float64(period)
	stdDev := math.Sqrt(variance)

	upperBand := sma + (2 * stdDev)
	lowerBand := sma - (2 * stdDev)

	return upperBand, sma, lowerBand
}

func (m *MovingAveragePredictor) calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50 // Neutral RSI
	}

	gains := 0.0
	losses := 0.0

	startIdx := len(prices) - period
	for i := startIdx; i < len(prices); i++ {
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

	if period == 0 || losses == 0 {
		return 50
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// Enhanced prediction using multiple indicators
func (m *MovingAveragePredictor) enhancedPredict(prices []float64) float64 {
	if len(prices) < m.longPeriod {
		return prices[len(prices)-1] // Return current price if insufficient data
	}

	currentPrice := prices[len(prices)-1]

	// Calculate multiple indicators
	shortMA := m.calculateSMA(prices, m.shortPeriod)
	longMA := m.calculateSMA(prices, m.longPeriod)
	rsi := m.calculateRSI(prices, 14)
	momentum := m.calculateMomentum(prices, 10)
	upperBand, _, lowerBand := m.calculateBollingerBands(prices, 20)

	// Weight different signals
	maSignal := 0.0
	if shortMA > longMA {
		maSignal = 1.0
	} else {
		maSignal = -1.0
	}

	rsiSignal := 0.0
	if rsi < 30 {
		rsiSignal = 1.0 // Oversold - buy signal
	} else if rsi > 70 {
		rsiSignal = -1.0 // Overbought - sell signal
	}

	momentumSignal := 0.0
	if momentum > 0.02 {
		momentumSignal = 1.0
	} else if momentum < -0.02 {
		momentumSignal = -1.0
	}

	bbSignal := 0.0
	if currentPrice < lowerBand {
		bbSignal = 1.0 // Below lower band - buy signal
	} else if currentPrice > upperBand {
		bbSignal = -1.0 // Above upper band - sell signal
	}

	// Combine signals with weights
	combinedSignal := (maSignal * 0.4) + (rsiSignal * 0.2) + (momentumSignal * 0.2) + (bbSignal * 0.2)

	// Calculate predicted price
	maxMove := currentPrice * 0.03 // Maximum 3% move
	predictedMove := combinedSignal * maxMove

	predictedPrice := currentPrice + predictedMove

	// Ensure reasonable bounds
	if predictedPrice < currentPrice*0.9 {
		predictedPrice = currentPrice * 0.9
	}
	if predictedPrice > currentPrice*1.1 {
		predictedPrice = currentPrice * 1.1
	}

	return predictedPrice
}

func (m *MovingAveragePredictor) GetName() string {
	return "Volume-Weighted Moving Average"
}

func (m *MovingAveragePredictor) GetAccuracy() float64 {
	if m.backtestAccuracy > 0 {
		return m.backtestAccuracy
	}
	return 0.0
}
