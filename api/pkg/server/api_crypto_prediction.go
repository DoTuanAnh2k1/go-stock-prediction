package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

type cryptoPredictionItem struct {
	ID             uint             `json:"id"`
	CoinID         string           `json:"coin_id"`
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

type cryptoPredictionsResponse struct {
	Data  []cryptoPredictionItem `json:"data"`
	Total int                    `json:"total"`
}

// GetCryptoPredictionsLatest godoc
//
//	@Summary      Get latest crypto predictions
//	@Description  Returns the most recent prediction per (coin_id, algorithm) combination
//	@Tags         Crypto Predictions
//	@Produce      json
//	@Success      200  {object}  cryptoPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/crypto/predictions/latest [get]
func GetCryptoPredictionsLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestCryptoPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/crypto/predictions/latest] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest crypto predictions")
		return
	}

	items := make([]cryptoPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, cryptoPredictionItem{
			ID:             p.ID,
			CoinID:         p.CoinID,
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

	ResponseSuccess(w, http.StatusOK, cryptoPredictionsResponse{Data: items, Total: len(items)})
}

// GetCryptoPredictionsLatestResults godoc
//
//	@Summary      Get latest confirmed crypto prediction results
//	@Description  Returns the most recent confirmed prediction (actual_price IS NOT NULL) per (coin_id, algorithm) combination, showing how predictions performed
//	@Tags         Crypto Predictions
//	@Produce      json
//	@Success      200  {object}  cryptoPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/crypto/predictions/latest-results [get]
func GetCryptoPredictionsLatestResults(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestConfirmedCryptoPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/crypto/predictions/latest-results] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest confirmed crypto predictions")
		return
	}

	items := make([]cryptoPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, cryptoPredictionItem{
			ID:             p.ID,
			CoinID:         p.CoinID,
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

	ResponseSuccess(w, http.StatusOK, cryptoPredictionsResponse{Data: items, Total: len(items)})
}

type cryptoPredictionChartPoint struct {
	Date           string           `json:"date"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
}

type cryptoPredictionChartResponse struct {
	CoinID    string                       `json:"coin_id"`
	Algorithm string                       `json:"algorithm"`
	Data      []cryptoPredictionChartPoint `json:"data"`
}

// GetCryptoPredictionsChart godoc
//
//	@Summary      Get crypto prediction chart data
//	@Description  Returns chronologically ordered predicted vs actual price data for charting, optionally filtered by algorithm
//	@Tags         Crypto Predictions
//	@Produce      json
//	@Param        coin       query  string  false  "Coin ID (e.g. bitcoin, ethereum)"
//	@Param        algorithm  query  string  false  "Algorithm name (e.g. lstm_nn)"
//	@Param        days       query  int     false  "Number of days to look back (default 30)"
//	@Success      200        {object}  cryptoPredictionChartResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/crypto/predictions/chart [get]
func GetCryptoPredictionsChart(w http.ResponseWriter, r *http.Request) {
	coinID := r.URL.Query().Get("coin")
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

	preds, err := store.GetCryptoPredictionsByDateRange(coinID, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/crypto/predictions/chart] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get crypto prediction chart data")
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

	data := make([]cryptoPredictionChartPoint, 0, len(filtered))
	// DB returns prediction_date ASC — iterate forward
	for _, p := range filtered {
		data = append(data, cryptoPredictionChartPoint{
			Date:           p.TargetDate.Format("2006-01-02T15:04:05"),
			AlgorithmName:  p.AlgorithmName,
			PredictedPrice: p.PredictedPrice,
			ActualPrice:    p.ActualPrice,
			Confidence:     p.Confidence,
		})
	}

	ResponseSuccess(w, http.StatusOK, cryptoPredictionChartResponse{
		CoinID:    coinID,
		Algorithm: algorithm,
		Data:      data,
	})
}

type cryptoPredictionsPageResponse struct {
	Data  []cryptoPredictionItem `json:"data"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

// GetCryptoPredictions godoc
//
//	@Summary      List crypto predictions with pagination
//	@Description  Returns paginated crypto predictions with optional filtering by coin, algorithm, and status
//	@Tags         Crypto Predictions
//	@Produce      json
//	@Param        coin       query  string  false  "Filter by coin ID (e.g. bitcoin)"
//	@Param        algorithm  query  string  false  "Filter by algorithm name"
//	@Param        status     query  string  false  "Filter by status (pending, confirmed, wrong)"
//	@Param        sort_by    query  string  false  "Sort field"
//	@Param        sort_dir   query  string  false  "Sort direction (asc, desc)"
//	@Param        page       query  int     false  "Page number (default 1)"
//	@Param        limit      query  int     false  "Items per page, max 100 (default 20)"
//	@Success      200        {object}  cryptoPredictionsPageResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/crypto/predictions [get]
func GetCryptoPredictions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	coinID := q.Get("coin")
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
	preds, total, err := store.GetCryptoPredictionsPage(page, limit, coinID, algorithm, statusFilter, sortBy, sortDir)
	if err != nil {
		logger.Logger.Errorf("[api/crypto/predictions] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get crypto predictions")
		return
	}

	items := make([]cryptoPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, cryptoPredictionItem{
			ID:             p.ID,
			CoinID:         p.CoinID,
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

	ResponseSuccess(w, http.StatusOK, cryptoPredictionsPageResponse{
		Data:  items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
