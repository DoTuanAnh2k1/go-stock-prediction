package server

import (
	"encoding/json"
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type ResponseFailure struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
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

func getPredictionStatus(targetDate time.Time, actualPrice *decimal.Decimal) string {
	if actualPrice != nil {
		return "confirmed"
	}

	if time.Now().After(targetDate) {
		return "pending_confirmation"
	}

	return "pending"
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
	// Sort by change percent
	sort.Slice(stocks, func(i, j int) bool {
		if gainers {
			return stocks[i].ChangePercent.GreaterThan(stocks[j].ChangePercent)
		}
		return stocks[i].ChangePercent.LessThan(stocks[j].ChangePercent)
	})

	if len(stocks) > limit {
		return stocks[:limit]
	}
	return stocks
}

func getTopByVolume(stocks []modelsapi.StockCurrentPriceDTO, limit int) []modelsapi.StockCurrentPriceDTO {
	// Sort by volume
	sort.Slice(stocks, func(i, j int) bool {
		return stocks[i].Volume > stocks[j].Volume
	})

	if len(stocks) > limit {
		return stocks[:limit]
	}
	return stocks
}
