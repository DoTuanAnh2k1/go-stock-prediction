package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

type sp500PredictionItem struct {
	ID             uint             `json:"id"`
	Symbol         string           `json:"symbol"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `json:"current_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
	PredictionDate string           `json:"prediction_date"`
	TargetDate     string           `json:"target_date"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Accuracy       *decimal.Decimal `json:"accuracy"`
}

type sp500PredictionsResponse struct {
	Data  []sp500PredictionItem `json:"data"`
	Total int                   `json:"total"`
}

// GetSP500PredictionsLatest godoc
//
//	@Summary      Get latest S&P 500 predictions
//	@Description  Returns the most recent prediction per (symbol, algorithm) combination
//	@Tags         SP500 Predictions
//	@Produce      json
//	@Success      200  {object}  sp500PredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/sp500/predictions/latest [get]
func GetSP500PredictionsLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestSP500Predictions()
	if err != nil {
		logger.Logger.Errorf("[api/sp500/predictions/latest] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest S&P 500 predictions")
		return
	}

	items := make([]sp500PredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, sp500PredictionItem{
			ID:             p.ID,
			Symbol:         p.Symbol,
			AlgorithmName:  p.AlgorithmName,
			PredictedPrice: p.PredictedPrice,
			CurrentPrice:   p.CurrentPrice,
			Confidence:     p.Confidence,
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, sp500PredictionsResponse{Data: items, Total: len(items)})
}

// GetSP500PredictionsLatestResults godoc
//
//	@Summary      Get latest confirmed S&P 500 prediction results
//	@Description  Returns the most recent confirmed prediction (actual_price IS NOT NULL) per (symbol, algorithm) combination
//	@Tags         SP500 Predictions
//	@Produce      json
//	@Success      200  {object}  sp500PredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/sp500/predictions/latest-results [get]
func GetSP500PredictionsLatestResults(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestConfirmedSP500Predictions()
	if err != nil {
		logger.Logger.Errorf("[api/sp500/predictions/latest-results] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest confirmed S&P 500 predictions")
		return
	}

	items := make([]sp500PredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, sp500PredictionItem{
			ID:             p.ID,
			Symbol:         p.Symbol,
			AlgorithmName:  p.AlgorithmName,
			PredictedPrice: p.PredictedPrice,
			CurrentPrice:   p.CurrentPrice,
			Confidence:     p.Confidence,
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, sp500PredictionsResponse{Data: items, Total: len(items)})
}

type sp500PredictionChartPoint struct {
	Date           string           `json:"date"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
}

type sp500PredictionChartResponse struct {
	Symbol    string                      `json:"symbol"`
	Algorithm string                      `json:"algorithm"`
	Data      []sp500PredictionChartPoint `json:"data"`
}

// GetSP500PredictionsChart godoc
//
//	@Summary      Get S&P 500 prediction chart data
//	@Description  Returns chronologically ordered predicted vs actual price data for charting, optionally filtered by algorithm
//	@Tags         SP500 Predictions
//	@Produce      json
//	@Param        symbol     query  string  false  "S&P 500 symbol (e.g. SPY, AAPL)"
//	@Param        algorithm  query  string  false  "Algorithm name (e.g. lstm_nn)"
//	@Param        days       query  int     false  "Number of days to look back (default 30)"
//	@Success      200        {object}  sp500PredictionChartResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/sp500/predictions/chart [get]
func GetSP500PredictionsChart(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	algorithm := r.URL.Query().Get("algorithm")

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}

	store := repository.GetSingleton()
	to := time.Now()
	from := to.AddDate(0, 0, -days)
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

	preds, err := store.GetSP500PredictionsByDateRange(symbol, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/sp500/predictions/chart] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get S&P 500 prediction chart data")
		return
	}

	// filter by algorithm if specified
	filtered := preds
	if algorithm != "" {
		filtered = filtered[:0]
		for _, p := range preds {
			if p.AlgorithmName == algorithm {
				filtered = append(filtered, p)
			}
		}
	}

	data := make([]sp500PredictionChartPoint, 0, len(filtered))
	// DB returns prediction_date ASC — iterate forward
	for _, p := range filtered {
		data = append(data, sp500PredictionChartPoint{
			Date:           p.TargetDate.Format("2006-01-02T15:04:05"),
			AlgorithmName:  p.AlgorithmName,
			PredictedPrice: p.PredictedPrice,
			ActualPrice:    p.ActualPrice,
			Confidence:     p.Confidence,
		})
	}

	ResponseSuccess(w, http.StatusOK, sp500PredictionChartResponse{
		Symbol:    symbol,
		Algorithm: algorithm,
		Data:      data,
	})
}

type sp500PredictionsPageResponse struct {
	Data  []sp500PredictionItem `json:"data"`
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

// GetSP500Predictions godoc
//
//	@Summary      List S&P 500 predictions with pagination
//	@Description  Returns paginated S&P 500 predictions with optional filtering by symbol, algorithm, and status
//	@Tags         SP500 Predictions
//	@Produce      json
//	@Param        symbol     query  string  false  "Filter by S&P 500 symbol"
//	@Param        algorithm  query  string  false  "Filter by algorithm name"
//	@Param        status     query  string  false  "Filter by status (pending, confirmed, wrong)"
//	@Param        sort_by    query  string  false  "Sort field"
//	@Param        sort_dir   query  string  false  "Sort direction (asc, desc)"
//	@Param        page       query  int     false  "Page number (default 1)"
//	@Param        limit      query  int     false  "Items per page, max 100 (default 20)"
//	@Success      200        {object}  sp500PredictionsPageResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/sp500/predictions [get]
func GetSP500Predictions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	symbol := q.Get("symbol")
	algorithm := q.Get("algorithm")
	statusFilter := q.Get("status")
	sortBy := q.Get("sort_by")
	sortDir := q.Get("sort_dir")

	page := 1
	if p := q.Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	limit := 20
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	store := repository.GetSingleton()
	preds, total, err := store.GetSP500PredictionsPage(page, limit, symbol, algorithm, statusFilter, sortBy, sortDir)
	if err != nil {
		logger.Logger.Errorf("[api/sp500/predictions] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get S&P 500 predictions")
		return
	}

	items := make([]sp500PredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, sp500PredictionItem{
			ID:             p.ID,
			Symbol:         p.Symbol,
			AlgorithmName:  p.AlgorithmName,
			PredictedPrice: p.PredictedPrice,
			CurrentPrice:   p.CurrentPrice,
			Confidence:     p.Confidence,
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, sp500PredictionsPageResponse{
		Data:  items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
