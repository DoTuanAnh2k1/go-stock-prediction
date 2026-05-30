package ema

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

// EMAPredictor implements the Exponential Moving Average algorithm with MACD signal.
// Uses standard EMA(12), EMA(26) and MACD signal(9) — the classic MACD setup.
type EMAPredictor struct {
	shortPeriod      int // 12 — standard MACD short EMA
	longPeriod       int // 26 — standard MACD long EMA
	signalPeriod     int // 9  — MACD signal line
	rng              *rand.Rand
	backtestAccuracy float64
}

// MACDSignal holds the trading signal derived from MACD indicators.
type MACDSignal struct {
	Direction string  // "BUY", "SELL", "HOLD"
	Strength  float64 // Signal strength [0–1]
}

// NewEMAPredictor constructs an EMAPredictor with standard MACD parameters.
func NewEMAPredictor() *EMAPredictor {
	return &EMAPredictor{
		shortPeriod:  12,
		longPeriod:   26,
		signalPeriod: 9,
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Predict generates a price prediction using EMA/MACD analysis.
// data.Historical must contain at least longPeriod (26) price strings.
func (m *EMAPredictor) Predict(ctx context.Context, data *modelssvc.StockData) (*modelssvc.Prediction, error) {
	prices, err := m.parseHistoricalData(data.Historical)
	if err != nil {
		return nil, fmt.Errorf("lỗi parse data: %v", err)
	}

	if len(prices) < m.longPeriod {
		return nil, fmt.Errorf("cần ít nhất %d ngày data", m.longPeriod)
	}

	currentPrice := prices[len(prices)-1]

	// Calculate short and long EMA (scalar — latest value)
	shortEMA := m.calculateEMA(prices, m.shortPeriod)
	longEMA := m.calculateEMA(prices, m.longPeriod)

	// Build full EMA series needed to compute the MACD signal line
	shortEMASeries := m.calculateEMASeries(prices, m.shortPeriod)
	longEMASeries := m.calculateEMASeries(prices, m.longPeriod)

	// Align series lengths — longEMASeries is shorter because it requires more history
	// shortEMASeries has len = len(prices) - shortPeriod + 1
	// longEMASeries  has len = len(prices) - longPeriod  + 1
	// The overlap length equals len(longEMASeries) since longPeriod > shortPeriod
	overlapLen := len(longEMASeries)
	shortOffset := len(shortEMASeries) - overlapLen
	macdSeries := make([]float64, overlapLen)
	for i := 0; i < overlapLen; i++ {
		macdSeries[i] = shortEMASeries[shortOffset+i] - longEMASeries[i]
	}

	// Latest MACD value
	macdLine := shortEMA - longEMA

	// Signal line = EMA of MACD series with signalPeriod
	var signalLine float64
	if len(macdSeries) >= m.signalPeriod {
		signalLine = m.calculateEMA(macdSeries, m.signalPeriod)
	} else {
		// Not enough history for signal line — use simple average as fallback
		sum := 0.0
		for _, v := range macdSeries {
			sum += v
		}
		signalLine = sum / float64(len(macdSeries))
	}

	// Histogram = MACD - Signal
	histogram := macdLine - signalLine

	// Generate trading signal
	signal := m.generateMACDSignal(macdLine, signalLine, histogram)

	// Confidence based on Vietnamese trading session
	confidence := m.adjustForVietnameseSession(signal)

	// Predicted price
	predictedPrice := m.calculatePredictedPrice(currentPrice, signal, confidence)

	m.backtestAccuracy = confidence

	return &modelssvc.Prediction{
		PredictedPrice: predictedPrice,
		CurrentPrice:   currentPrice,
		Confidence:     confidence,
	}, nil
}

// GetName returns the human-readable algorithm name.
func (m *EMAPredictor) GetName() string {
	return "Exponential Moving Average"
}

// GetAccuracy returns the last computed backtest accuracy.
func (m *EMAPredictor) GetAccuracy() float64 {
	return m.backtestAccuracy
}

// parseHistoricalData converts Vietnamese-format price strings to float64 slice.
// Handles comma separators (e.g. "30,000") and spaces (e.g. "30 000").
func (m *EMAPredictor) parseHistoricalData(historical []string) ([]float64, error) {
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

// calculateEMA computes the EMA for the last element of prices using the given period.
// The first EMA value is seeded with the SMA of the first `period` elements.
// Returns 0 if len(prices) < period.
func (m *EMAPredictor) calculateEMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	k := 2.0 / (float64(period) + 1.0)

	// Seed: SMA of first `period` prices
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	ema := sum / float64(period)

	// Iterate forward from element at index `period`
	for i := period; i < len(prices); i++ {
		ema = prices[i]*k + ema*(1-k)
	}

	return ema
}

// calculateEMASeries computes the full EMA series for the given period.
// The returned slice starts at the index where enough history is available
// (i.e. from index period-1 onward), giving length = len(prices) - period + 1.
// Returns nil if len(prices) < period.
func (m *EMAPredictor) calculateEMASeries(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}

	k := 2.0 / (float64(period) + 1.0)
	resultLen := len(prices) - period + 1
	series := make([]float64, resultLen)

	// Seed with SMA of first `period` elements
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	series[0] = sum / float64(period)

	for i := 1; i < resultLen; i++ {
		series[i] = prices[period-1+i]*k + series[i-1]*(1-k)
	}

	return series
}

// generateMACDSignal derives a trading signal from MACD line, signal line, and histogram.
func (m *EMAPredictor) generateMACDSignal(macdLine, signalLine, histogram float64) MACDSignal {
	signal := MACDSignal{Direction: "HOLD", Strength: 0.5}

	if macdLine > signalLine && histogram > 0 {
		signal.Direction = "BUY"
	} else if macdLine < signalLine && histogram < 0 {
		signal.Direction = "SELL"
	}

	// Signal strength based on histogram relative to MACD magnitude
	if macdLine != 0 {
		signal.Strength = math.Min(1.0, math.Abs(histogram)/math.Abs(macdLine)*2+0.5)
	} else {
		signal.Strength = 0.5
	}

	// Clamp to valid range
	if signal.Strength > 1.0 {
		signal.Strength = 1.0
	}
	if signal.Strength < 0.1 {
		signal.Strength = 0.1
	}

	return signal
}

// adjustForVietnameseSession adjusts prediction confidence based on VN trading hours.
func (m *EMAPredictor) adjustForVietnameseSession(signal MACDSignal) float64 {
	baseConfidence := signal.Strength * 0.75

	currentHour := time.Now().Hour()

	var timeAdjustment float64
	if currentHour >= 9 && currentHour <= 11 {
		// Morning session — higher volume
		timeAdjustment = 0.1
	} else if currentHour >= 13 && currentHour <= 14 {
		// Afternoon session
		timeAdjustment = 0.05
	} else {
		// Outside trading hours
		timeAdjustment = -0.05
	}

	confidence := baseConfidence + timeAdjustment

	// Clamp to [0.3, 0.9]
	if confidence > 0.9 {
		confidence = 0.9
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

// calculatePredictedPrice computes the next-day price estimate.
func (m *EMAPredictor) calculatePredictedPrice(currentPrice float64, signal MACDSignal, confidence float64) float64 {
	predictedPrice := currentPrice

	var priceMovement float64
	switch signal.Direction {
	case "BUY":
		priceMovement = currentPrice * signal.Strength * 0.025 * confidence
	case "SELL":
		priceMovement = -currentPrice * signal.Strength * 0.025 * confidence
	default: // "HOLD"
		priceMovement = currentPrice * (signal.Strength - 0.5) * 0.005
	}

	predictedPrice += priceMovement

	// Market noise — decreases with higher confidence
	noiseRange := currentPrice * 0.001 * (1 - confidence)
	noise := (m.random() - 0.5) * 2 * noiseRange
	predictedPrice += noise

	// Clamp to ±7% Vietnamese daily limit
	maxDailyChange := currentPrice * 0.07
	if predictedPrice > currentPrice+maxDailyChange {
		predictedPrice = currentPrice + maxDailyChange
	}
	if predictedPrice < currentPrice-maxDailyChange {
		predictedPrice = currentPrice - maxDailyChange
	}

	// Price must be positive
	if predictedPrice < 0 {
		predictedPrice = currentPrice * 0.01
	}

	return predictedPrice
}

// random returns a pseudo-random float64 in [0, 1) using the seeded RNG.
func (m *EMAPredictor) random() float64 {
	return m.rng.Float64()
}
