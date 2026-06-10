package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

type nasdaqPredictionItem struct {
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

type nasdaqPredictionsResponse struct {
	Data  []nasdaqPredictionItem `json:"data"`
	Total int                    `json:"total"`
}

// GetNasdaqPredictionsLatest godoc
//
//	@Summary      Get latest NASDAQ predictions
//	@Description  Returns the most recent prediction per (symbol, algorithm) combination
//	@Tags         NASDAQ Predictions
//	@Produce      json
//	@Success      200  {object}  nasdaqPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/nasdaq/predictions/latest [get]
func GetNasdaqPredictionsLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestNasdaqPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/predictions/latest] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest NASDAQ predictions")
		return
	}

	items := make([]nasdaqPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, nasdaqPredictionItem{
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

	ResponseSuccess(w, http.StatusOK, nasdaqPredictionsResponse{Data: items, Total: len(items)})
}

// GetNasdaqPredictionsLatestResults godoc
//
//	@Summary      Get latest confirmed NASDAQ prediction results
//	@Description  Returns the most recent confirmed prediction (actual_price IS NOT NULL) per (symbol, algorithm) combination
//	@Tags         NASDAQ Predictions
//	@Produce      json
//	@Success      200  {object}  nasdaqPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/nasdaq/predictions/latest-results [get]
func GetNasdaqPredictionsLatestResults(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestConfirmedNasdaqPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/predictions/latest-results] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest confirmed NASDAQ predictions")
		return
	}

	items := make([]nasdaqPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, nasdaqPredictionItem{
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

	ResponseSuccess(w, http.StatusOK, nasdaqPredictionsResponse{Data: items, Total: len(items)})
}

type nasdaqPredictionChartPoint struct {
	Date           string           `json:"date"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
}

type nasdaqPredictionChartResponse struct {
	Symbol    string                       `json:"symbol"`
	Algorithm string                       `json:"algorithm"`
	Data      []nasdaqPredictionChartPoint `json:"data"`
}

// GetNasdaqPredictionsChart godoc
//
//	@Summary      Get NASDAQ prediction chart data
//	@Description  Returns chronologically ordered predicted vs actual price data for charting, optionally filtered by algorithm
//	@Tags         NASDAQ Predictions
//	@Produce      json
//	@Param        symbol     query  string  false  "NASDAQ symbol (e.g. AAPL, MSFT)"
//	@Param        algorithm  query  string  false  "Algorithm name (e.g. lstm_nn)"
//	@Param        days       query  int     false  "Number of days to look back (default 30)"
//	@Success      200        {object}  nasdaqPredictionChartResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/nasdaq/predictions/chart [get]
func GetNasdaqPredictionsChart(w http.ResponseWriter, r *http.Request) {
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

	preds, err := store.GetNasdaqPredictionsByDateRange(symbol, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/predictions/chart] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get NASDAQ prediction chart data")
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

	data := make([]nasdaqPredictionChartPoint, 0, len(filtered))
	// DB returns prediction_date ASC — iterate forward
	for _, p := range filtered {
		data = append(data, nasdaqPredictionChartPoint{
			Date:           p.TargetDate.Format("2006-01-02T15:04:05"),
			AlgorithmName:  p.AlgorithmName,
			PredictedPrice: p.PredictedPrice,
			ActualPrice:    p.ActualPrice,
			Confidence:     p.Confidence,
		})
	}

	ResponseSuccess(w, http.StatusOK, nasdaqPredictionChartResponse{
		Symbol:    symbol,
		Algorithm: algorithm,
		Data:      data,
	})
}

type nasdaqPredictionsPageResponse struct {
	Data  []nasdaqPredictionItem `json:"data"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

// GetNasdaqPredictions godoc
//
//	@Summary      List NASDAQ predictions with pagination
//	@Description  Returns paginated NASDAQ predictions with optional filtering by symbol, algorithm, and status
//	@Tags         NASDAQ Predictions
//	@Produce      json
//	@Param        symbol     query  string  false  "Filter by NASDAQ symbol"
//	@Param        algorithm  query  string  false  "Filter by algorithm name"
//	@Param        status     query  string  false  "Filter by status (pending, confirmed, wrong)"
//	@Param        sort_by    query  string  false  "Sort field"
//	@Param        sort_dir   query  string  false  "Sort direction (asc, desc)"
//	@Param        page       query  int     false  "Page number (default 1)"
//	@Param        limit      query  int     false  "Items per page, max 100 (default 20)"
//	@Success      200        {object}  nasdaqPredictionsPageResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/nasdaq/predictions [get]
func GetNasdaqPredictions(w http.ResponseWriter, r *http.Request) {
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
	preds, total, err := store.GetNasdaqPredictionsPage(page, limit, symbol, algorithm, statusFilter, sortBy, sortDir)
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/predictions] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get NASDAQ predictions")
		return
	}

	items := make([]nasdaqPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, nasdaqPredictionItem{
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

	ResponseSuccess(w, http.StatusOK, nasdaqPredictionsPageResponse{
		Data:  items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
