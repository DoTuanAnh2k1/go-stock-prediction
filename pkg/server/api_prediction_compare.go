package server

import (
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// PredictionComparePoint is one data point in the compare response.
type PredictionComparePoint struct {
	Date      string          `json:"date"`
	Predicted decimal.Decimal `json:"predicted"`
	Actual    decimal.Decimal `json:"actual"`
	Algorithm string          `json:"algorithm"`
}

// PredictionCompareDTO is the response envelope for GET /api/predictions/compare/{symbol}.
type PredictionCompareDTO struct {
	Symbol string                   `json:"symbol"`
	Data   []PredictionComparePoint `json:"data"`
}

// ErrorDistributionPoint is one scatter-plot data point.
type ErrorDistributionPoint struct {
	PredictedChangePct float64 `json:"predicted_change_pct"`
	ActualChangePct    float64 `json:"actual_change_pct"`
	Algorithm          string  `json:"algorithm"`
	Symbol             string  `json:"symbol"`
}

// ErrorDistributionDTO is the response for GET /api/predictions/error-distribution.
type ErrorDistributionDTO struct {
	Data []ErrorDistributionPoint `json:"data"`
}

// GetPredictionCompare handles GET /api/predictions/compare/{symbol}
// Query params: ?days=30 (default 30), ?algorithm=lstm_nn (optional)
func GetPredictionCompare(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		ResponseError(w, http.StatusBadRequest, "symbol is required")
		return
	}
	if err := validateSymbol(symbol); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	algorithm := r.URL.Query().Get("algorithm")
	if err := validateAlgorithm(algorithm); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	days := 30
	if dStr := r.URL.Query().Get("days"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}

	store := repository.GetSingleton()

	stock, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Stock not found")
		return
	}

	predictions, err := store.GetPredictionsWithActual(&stock.ID, algorithm, days)
	if err != nil {
		logger.Logger.Errorf("Failed to get predictions for compare symbol=%s: %v", symbol, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	points := make([]PredictionComparePoint, 0, len(predictions))
	for _, p := range predictions {
		if p.ActualPrice == nil {
			continue
		}
		points = append(points, PredictionComparePoint{
			Date:      p.TargetDate.Format(time.DateOnly),
			Predicted: p.PredictedPrice,
			Actual:    *p.ActualPrice,
			Algorithm: p.AlgorithmName,
		})
	}

	ResponseSuccess(w, http.StatusOK, &PredictionCompareDTO{
		Symbol: symbol,
		Data:   points,
	})
}

// GetErrorDistribution handles GET /api/predictions/error-distribution
// Returns scatter plot data: predicted_change_pct vs actual_change_pct per prediction.
func GetErrorDistribution(w http.ResponseWriter, r *http.Request) {
	algorithm := r.URL.Query().Get("algorithm")
	if err := validateAlgorithm(algorithm); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := repository.GetSingleton()

	// Fetch all predictions that have an actual price (no stock filter, no day limit)
	predictions, err := store.GetPredictionsWithActual(nil, algorithm, 0)
	if err != nil {
		logger.Logger.Errorf("Failed to get predictions for error distribution: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
		return
	}

	// Build stock symbol cache
	stockSymbols := make(map[uint]string)

	points := make([]ErrorDistributionPoint, 0, len(predictions))
	for _, p := range predictions {
		if p.ActualPrice == nil {
			continue
		}
		if p.CurrentPrice.IsZero() {
			continue
		}

		// Look up symbol
		sym, ok := stockSymbols[p.StockID]
		if !ok {
			s, err := store.GetStockByID(p.StockID)
			if err != nil {
				logger.Logger.Warnf("Stock %d not found for error distribution: %v", p.StockID, err)
				continue
			}
			sym = s.Symbol
			stockSymbols[p.StockID] = sym
		}

		hundred := decimal.NewFromInt(100)

		predictedChangePct, _ := p.PredictedPrice.Sub(p.CurrentPrice).
			Div(p.CurrentPrice).Mul(hundred).Float64()

		actualChangePct, _ := p.ActualPrice.Sub(p.CurrentPrice).
			Div(p.CurrentPrice).Mul(hundred).Float64()

		points = append(points, ErrorDistributionPoint{
			PredictedChangePct: predictedChangePct,
			ActualChangePct:    actualChangePct,
			Algorithm:          p.AlgorithmName,
			Symbol:             sym,
		})
	}

	ResponseSuccess(w, http.StatusOK, &ErrorDistributionDTO{Data: points})
}
