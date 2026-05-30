package server

import (
	"go-stock-prediction/pkg/logger"
	goldpredict "go-stock-prediction/pkg/service/predict/gold"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

type goldPredictionItem struct {
	ID             uint             `json:"id"`
	Source         string           `json:"source"`
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

type goldPredictionsResponse struct {
	Data  []goldPredictionItem `json:"data"`
	Total int                  `json:"total"`
}

// GetGoldPredictions handles GET /api/gold/predictions
// Query params: source, product_type, algorithm, days (default 30), limit (default 100)
func GetGoldPredictions(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	productType := r.URL.Query().Get("product_type")
	algorithm := r.URL.Query().Get("algorithm")

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	store := repository.GetSingleton()
	preds, err := store.GetGoldPredictions(source, productType, algorithm, limit)
	if err != nil {
		logger.Logger.Errorf("[api/gold/predictions] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get gold predictions")
		return
	}

	items := make([]goldPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, goldPredictionItem{
			ID:             p.ID,
			Source:         p.Source,
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

	ResponseSuccess(w, http.StatusOK, goldPredictionsResponse{Data: items, Total: len(items)})
}

// GetLatestGoldPredictions handles GET /api/gold/predictions/latest
// Returns the most recent prediction per (source, product_type, algorithm).
func GetLatestGoldPredictions(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestGoldPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/gold/predictions/latest] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest gold predictions")
		return
	}

	items := make([]goldPredictionItem, 0, len(preds))
	for _, p := range preds {
		items = append(items, goldPredictionItem{
			ID:             p.ID,
			Source:         p.Source,
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

	ResponseSuccess(w, http.StatusOK, goldPredictionsResponse{Data: items, Total: len(items)})
}

type goldPredictionChartPoint struct {
	Date           string          `json:"date"`
	PredictedPrice decimal.Decimal `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal `json:"confidence"`
}

type goldPredictionChartResponse struct {
	Source      string                     `json:"source"`
	ProductType string                     `json:"product_type"`
	Algorithm   string                     `json:"algorithm"`
	Data        []goldPredictionChartPoint `json:"data"`
}

// GetGoldPredictionChart handles GET /api/gold/predictions/chart
// Query params: source, product_type, algorithm, days (default 30)
func GetGoldPredictionChart(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	productType := r.URL.Query().Get("product_type")
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

	preds, err := store.GetGoldPredictionsByDateRange(source, productType, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/gold/predictions/chart] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get gold prediction chart data")
		return
	}

	// filter by algorithm if specified, reverse for chronological order
	filtered := preds
	if algorithm != "" {
		filtered = filtered[:0]
		for _, p := range preds {
			if p.AlgorithmName == algorithm {
				filtered = append(filtered, p)
			}
		}
	}

	data := make([]goldPredictionChartPoint, 0, len(filtered))
	for i := len(filtered) - 1; i >= 0; i-- {
		p := filtered[i]
		data = append(data, goldPredictionChartPoint{
			Date:           p.PredictionDate.Format("2006-01-02"),
			PredictedPrice: p.PredictedPrice,
			ActualPrice:    p.ActualPrice,
			Confidence:     p.Confidence,
		})
	}

	ResponseSuccess(w, http.StatusOK, goldPredictionChartResponse{
		Source:      source,
		ProductType: productType,
		Algorithm:   algorithm,
		Data:        data,
	})
}

// TriggerGoldPredictHandler handles POST /api/trigger/gold-predict
func TriggerGoldPredictHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		count, err := goldpredict.RunNow()
		if err != nil {
			logger.Logger.Errorf("Manual gold prediction failed: %v", err)
			return
		}
		logger.Logger.Infof("Manual gold prediction done: %d predictions saved", count)
	}()
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "Gold prediction triggered"})
}
