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

// GetFuelPredictionsLatest godoc
//
//	@Summary      Get latest fuel predictions
//	@Description  Returns the most recent prediction per (product_type, algorithm) combination
//	@Tags         Fuel Predictions
//	@Produce      json
//	@Success      200  {object}  fuelPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/fuel/predictions/latest [get]
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
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, fuelPredictionsResponse{Data: items, Total: len(items)})
}

// GetFuelPredictionsLatestResults godoc
//
//	@Summary      Get latest confirmed fuel prediction results
//	@Description  Returns the most recent confirmed prediction (actual_price IS NOT NULL) per (product_type, algorithm) combination
//	@Tags         Fuel Predictions
//	@Produce      json
//	@Success      200  {object}  fuelPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/fuel/predictions/latest-results [get]
func GetFuelPredictionsLatestResults(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestConfirmedFuelPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/fuel/predictions/latest-results] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest confirmed fuel predictions")
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
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, fuelPredictionsResponse{Data: items, Total: len(items)})
}

type fuelPredictionChartPoint struct {
	Date           string           `json:"date"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
}

type fuelPredictionChartResponse struct {
	ProductType string                     `json:"product_type"`
	Algorithm   string                     `json:"algorithm"`
	Data        []fuelPredictionChartPoint `json:"data"`
}

// GetFuelPredictionsChart godoc
//
//	@Summary      Get fuel prediction chart data
//	@Description  Returns chronologically ordered predicted vs actual price data for charting, optionally filtered by algorithm
//	@Tags         Fuel Predictions
//	@Produce      json
//	@Param        product    query  string  false  "Fuel product type (e.g. ron95_iii, ron95_v)"
//	@Param        algorithm  query  string  false  "Algorithm name (e.g. lstm_nn)"
//	@Param        days       query  int     false  "Number of days to look back (default 180)"
//	@Success      200        {object}  fuelPredictionChartResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/fuel/predictions/chart [get]
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
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

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
			Date:           p.TargetDate.Format("2006-01-02"),
			AlgorithmName:  p.AlgorithmName,
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

// GetFuelPredictions godoc
//
//	@Summary      List fuel predictions with pagination
//	@Description  Returns paginated fuel predictions with optional filtering by product type, algorithm, and status
//	@Tags         Fuel Predictions
//	@Produce      json
//	@Param        product    query  string  false  "Filter by fuel product type"
//	@Param        algorithm  query  string  false  "Filter by algorithm name"
//	@Param        status     query  string  false  "Filter by status (pending, confirmed, wrong)"
//	@Param        sort_by    query  string  false  "Sort field"
//	@Param        sort_dir   query  string  false  "Sort direction (asc, desc)"
//	@Param        page       query  int     false  "Page number (default 1)"
//	@Param        limit      query  int     false  "Items per page, max 100 (default 20)"
//	@Success      200        {object}  fuelPredictionsPageResponse
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/fuel/predictions [get]
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
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
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
