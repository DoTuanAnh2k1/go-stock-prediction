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

// GetCryptoPredictionsLatest handles GET /api/crypto/predictions/latest
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
			PredictionDate: p.PredictionDate.Format("2006-01-02"),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, cryptoPredictionsResponse{Data: items, Total: len(items)})
}

type cryptoPredictionChartPoint struct {
	Date           string           `json:"date"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
}

type cryptoPredictionChartResponse struct {
	CoinID    string                       `json:"coin_id"`
	Algorithm string                       `json:"algorithm"`
	Data      []cryptoPredictionChartPoint `json:"data"`
}

// GetCryptoPredictionsChart handles GET /api/crypto/predictions/chart?coin=bitcoin&days=30
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
	// reverse for chronological order (DB returns DESC)
	for i := len(filtered) - 1; i >= 0; i-- {
		p := filtered[i]
		data = append(data, cryptoPredictionChartPoint{
			Date:           p.PredictionDate.Format("2006-01-02"),
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

// GetCryptoPredictions handles GET /api/crypto/predictions?page=1&limit=20&coin=&algorithm=
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
			PredictionDate: p.PredictionDate.Format("2006-01-02"),
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
