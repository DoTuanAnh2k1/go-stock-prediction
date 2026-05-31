package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

type fuelPredictionItem struct {
	ID             uint             `json:"id"`
	ProductType    string           `json:"product_type"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `json:"current_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
	PredictionDate string           `json:"prediction_date"`
	TargetDate     string           `json:"target_date"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Accuracy       *decimal.Decimal `json:"accuracy"`
}

type fuelPredictionsResponse struct {
	Data  []fuelPredictionItem `json:"data"`
	Total int                  `json:"total"`
}

// GetFuelPredictionsLatest handles GET /api/fuel/predictions/latest
func GetFuelPredictionsLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestFuelPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/fuel/predictions/latest] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest fuel predictions")
		return
	}

	items := make([]fuelPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, fuelPredictionItem{
			ID:             p.ID,
			ProductType:    p.ProductType,
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

	ResponseSuccess(w, http.StatusOK, fuelPredictionsResponse{Data: items, Total: len(items)})
}

type fuelPredictionChartPoint struct {
	Date           string           `json:"date"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
}

type fuelPredictionChartResponse struct {
	ProductType string                     `json:"product_type"`
	Algorithm   string                     `json:"algorithm"`
	Data        []fuelPredictionChartPoint `json:"data"`
}

// GetFuelPredictionsChart handles GET /api/fuel/predictions/chart?product=ron95_iii&days=180
func GetFuelPredictionsChart(w http.ResponseWriter, r *http.Request) {
	productType := r.URL.Query().Get("product")
	algorithm := r.URL.Query().Get("algorithm")

	days := 180
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			days = v
		}
	}

	store := repository.GetSingleton()
	to := time.Now()
	from := to.AddDate(0, 0, -days)

	preds, err := store.GetFuelPredictionsByDateRange(productType, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/fuel/predictions/chart] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get fuel prediction chart data")
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

	data := make([]fuelPredictionChartPoint, 0, len(filtered))
	// reverse for chronological order (DB returns DESC)
	for i := len(filtered) - 1; i >= 0; i-- {
		p := filtered[i]
		data = append(data, fuelPredictionChartPoint{
			Date:           p.PredictionDate.Format("2006-01-02"),
			PredictedPrice: p.PredictedPrice,
			ActualPrice:    p.ActualPrice,
			Confidence:     p.Confidence,
		})
	}

	ResponseSuccess(w, http.StatusOK, fuelPredictionChartResponse{
		ProductType: productType,
		Algorithm:   algorithm,
		Data:        data,
	})
}

type fuelPredictionsPageResponse struct {
	Data  []fuelPredictionItem `json:"data"`
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
}

// GetFuelPredictions handles GET /api/fuel/predictions?page=1&limit=20&product=&algorithm=
func GetFuelPredictions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	productType := q.Get("product")
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
	preds, total, err := store.GetFuelPredictionsPage(page, limit, productType, algorithm, statusFilter, sortBy, sortDir)
	if err != nil {
		logger.Logger.Errorf("[api/fuel/predictions] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get fuel predictions")
		return
	}

	items := make([]fuelPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, fuelPredictionItem{
			ID:             p.ID,
			ProductType:    p.ProductType,
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

	ResponseSuccess(w, http.StatusOK, fuelPredictionsPageResponse{
		Data:  items,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}
