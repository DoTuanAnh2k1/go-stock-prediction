package server

import (
	"encoding/json"
	"fmt"
	grpcclient "go-stock-prediction/pkg/grpc/client"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	pb "go-stock-prediction/proto/prediction"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
)

type ResponseFailure struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

// requireGRPCClient returns the prediction gRPC client, or writes a 503 error
// and returns nil if the client is not initialised (e.g. in unit tests).
func requireGRPCClient(w http.ResponseWriter) pb.PredictionServiceClient {
	c := grpcclient.GetClient()
	if c == nil {
		ResponseError(w, http.StatusServiceUnavailable, "prediction service not available")
		return nil
	}
	return c
}

func ResponseError(w http.ResponseWriter, status int, message string) {
	response := ResponseFailure{
		StatusCode: status,
		Message:    message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	bodyResponse, err := json.Marshal(response)
	if err != nil {
		logger.Logger.Error("Failed to marshal error response", "error", err)
		return
	}

	w.Write(bodyResponse)
}

func ResponseSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		bodyResponse, err := json.Marshal(data)
		if err != nil {
			logger.Logger.Error("Failed to marshal success response", "error", err)
			ResponseError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		w.Write(bodyResponse)
	}
}

// getPredictionStatus computes the prediction status based on target date and actual price.
// Rules:
//   - "pending"              — target_date is still in the future, no actual price yet
//   - "pending_confirmation" — target_date has passed but actual_price is still unknown
//   - "confirmed"            — actual_price is known and error < 5 %
//   - "wrong"               — actual_price is known and error >= 5 %
func getPredictionStatus(targetDate time.Time, actualPrice *decimal.Decimal) string {
	return getPredictionStatusWithPredicted(targetDate, actualPrice, nil)
}

// getPredictionStatusWithPredicted is the full version that uses both actual and predicted prices.
func getPredictionStatusWithPredicted(targetDate time.Time, actualPrice *decimal.Decimal, predictedPrice *decimal.Decimal) string {
	if actualPrice == nil {
		if time.Now().After(targetDate) {
			return "pending_confirmation"
		}
		return "pending"
	}

	// actual price is known — determine confirmed vs wrong
	if predictedPrice != nil && actualPrice.GreaterThan(decimal.Zero) {
		diff := predictedPrice.Sub(*actualPrice).Abs()
		errPct := diff.Div(*actualPrice)
		threshold := decimal.NewFromFloat(0.05)
		if errPct.GreaterThanOrEqual(threshold) {
			return "wrong"
		}
	}
	return "confirmed"
}

// Helper functions
func getMarketStatus() string {
	now := time.Now()
	hour := now.Hour()

	// Vietnam stock market hours: 9:00-11:30, 13:00-15:00
	if (hour >= 9 && hour < 11) || (hour == 11 && now.Minute() <= 30) ||
		(hour >= 13 && hour < 15) {
		return "open"
	}

	if hour < 9 {
		return "pre_market"
	}

	if hour >= 15 {
		return "after_hours"
	}

	return "closed"
}

func calculateDateRange(period string) (time.Time, time.Time) {
	now := time.Now()
	var fromDate time.Time

	switch period {
	case "1D":
		fromDate = now.AddDate(0, 0, -1)
	case "1W":
		fromDate = now.AddDate(0, 0, -7)
	case "1M":
		fromDate = now.AddDate(0, -1, 0)
	case "3M":
		fromDate = now.AddDate(0, -3, 0)
	case "6M":
		fromDate = now.AddDate(0, -6, 0)
	case "1Y":
		fromDate = now.AddDate(-1, 0, 0)
	default:
		fromDate = now.AddDate(0, -1, 0) // Default to 1 month
	}

	return fromDate, now
}

func calculateStockStats(prices []modelsdb.StockPrice) modelsapi.StockStatsDTO {
	if len(prices) == 0 {
		return modelsapi.StockStatsDTO{}
	}

	var highest, lowest, totalPrice decimal.Decimal
	var totalVolume int64
	var totalValue decimal.Decimal
	var priceChanges []float64

	// Initialize with first price
	highest = prices[0].HighPrice
	lowest = prices[0].LowPrice

	for _, price := range prices {
		// Find highest/lowest
		if price.HighPrice.GreaterThan(highest) {
			highest = price.HighPrice
		}
		if price.LowPrice.LessThan(lowest) {
			lowest = price.LowPrice
		}

		// Sum for averages
		totalPrice = totalPrice.Add(price.ClosePrice)
		totalVolume += price.Volume
		totalValue = totalValue.Add(price.Value)

		// Collect price changes for volatility
		changeFloat, _ := price.ChangePercent.Float64()
		priceChanges = append(priceChanges, changeFloat)
	}

	avgPrice := totalPrice.Div(decimal.NewFromInt(int64(len(prices))))

	// Calculate period change
	var priceChange, percentChange decimal.Decimal
	if len(prices) > 1 {
		startPrice := prices[len(prices)-1].ClosePrice // Oldest price
		endPrice := prices[0].ClosePrice               // Latest price
		priceChange = endPrice.Sub(startPrice)
		if startPrice.GreaterThan(decimal.Zero) {
			percentChange = priceChange.Div(startPrice).Mul(decimal.NewFromInt(100))
		}
	}

	// Calculate volatility (standard deviation)
	volatility := calculateVolatility(priceChanges)

	return modelsapi.StockStatsDTO{
		HighestPrice:  highest,
		LowestPrice:   lowest,
		AveragePrice:  avgPrice,
		TotalVolume:   totalVolume,
		TotalValue:    totalValue,
		PriceChange:   priceChange,
		PercentChange: percentChange,
		Volatility:    decimal.NewFromFloat(volatility),
		TradingDays:   len(prices),
	}
}

func calculateVolatility(changes []float64) float64 {
	if len(changes) <= 1 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, change := range changes {
		sum += change
	}
	mean := sum / float64(len(changes))

	// Calculate variance
	variance := 0.0
	for _, change := range changes {
		variance += math.Pow(change-mean, 2)
	}
	variance /= float64(len(changes) - 1)

	// Return standard deviation
	return math.Sqrt(variance)
}

func getTopByChange(stocks []modelsapi.StockCurrentPriceDTO, gainers bool, limit int) []modelsapi.StockCurrentPriceDTO {
	tmp := make([]modelsapi.StockCurrentPriceDTO, len(stocks))
	copy(tmp, stocks)
	// Sort by change percent
	sort.Slice(tmp, func(i, j int) bool {
		if gainers {
			return tmp[i].ChangePercent.GreaterThan(tmp[j].ChangePercent)
		}
		return tmp[i].ChangePercent.LessThan(tmp[j].ChangePercent)
	})

	if len(tmp) > limit {
		return tmp[:limit]
	}
	return tmp
}

func getTopByVolume(stocks []modelsapi.StockCurrentPriceDTO, limit int) []modelsapi.StockCurrentPriceDTO {
	tmp := make([]modelsapi.StockCurrentPriceDTO, len(stocks))
	copy(tmp, stocks)
	// Sort by volume
	sort.Slice(tmp, func(i, j int) bool {
		return tmp[i].Volume > tmp[j].Volume
	})

	if len(tmp) > limit {
		return tmp[:limit]
	}
	return tmp
}

var validAlgorithms = map[string]bool{
	"moving_average": true,
	"lstm_nn":        true,
	"arima_garch":    true,
	"ema":            true,
	"ensemble":       true,
}

var validPeriods = map[string]bool{
	"1D": true, "1W": true, "1M": true,
	"3M": true, "6M": true, "1Y": true,
}

func validateSymbol(symbol string) error {
	if symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if len(symbol) < 2 || len(symbol) > 5 {
		return fmt.Errorf("invalid symbol format")
	}
	for _, c := range symbol {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			return fmt.Errorf("symbol contains invalid characters")
		}
	}
	return nil
}

func validateLimit(limitStr string, defaultLimit, maxLimit int) (int, error) {
	if limitStr == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > maxLimit {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
	}
	return limit, nil
}

func validateAlgorithm(algorithm string) error {
	if algorithm == "" {
		return nil
	}
	if !validAlgorithms[algorithm] {
		return fmt.Errorf("unknown algorithm: %s", algorithm)
	}
	return nil
}

func validatePeriod(period string) error {
	if period == "" {
		return nil
	}
	if !validPeriods[period] {
		return fmt.Errorf("unknown period: %s (valid: 1D, 1W, 1M, 3M, 6M, 1Y)", period)
	}
	return nil
}

func validatePage(pageStr string) (int, error) {
	if pageStr == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 0, fmt.Errorf("page must be a positive integer")
	}
	return page, nil
}

func validateDateParam(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date format, expected YYYY-MM-DD")
	}
	return t, nil
}
