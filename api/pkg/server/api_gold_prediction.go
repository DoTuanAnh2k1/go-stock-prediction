package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
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

// GetGoldPredictions godoc
//
//	@Summary      List gold predictions
//	@Description  Returns gold price predictions filtered by source, product type, and algorithm
//	@Tags         Gold Predictions
//	@Produce      json
//	@Param        source        query  string  false  "Gold source (e.g. SJC, BTMC)"
//	@Param        product_type  query  string  false  "Product type (e.g. 1l, nhan_tron)"
//	@Param        algorithm     query  string  false  "Algorithm name (e.g. lstm_nn)"
//	@Param        limit         query  int     false  "Maximum number of results (default 100)"
//	@Success      200           {object}  goldPredictionsResponse
//	@Failure      500           {object}  ResponseFailure
//	@Router       /api/gold/predictions [get]
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
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, goldPredictionsResponse{Data: items, Total: len(items)})
}

// GetLatestGoldPredictions godoc
//
//	@Summary      Get latest gold predictions
//	@Description  Returns the most recent prediction per (source, product_type, algorithm) combination
//	@Tags         Gold Predictions
//	@Produce      json
//	@Success      200  {object}  goldPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/gold/predictions/latest [get]
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
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
			TargetDate:     p.TargetDate.Format("2006-01-02"),
			ActualPrice:    p.ActualPrice,
			Accuracy:       p.Accuracy,
		})
	}

	ResponseSuccess(w, http.StatusOK, goldPredictionsResponse{Data: items, Total: len(items)})
}

// GetGoldPredictionsLatestResults godoc
//
//	@Summary      Get latest confirmed gold prediction results
//	@Description  Returns the most recent confirmed prediction (actual_price IS NOT NULL) per (source, product_type, algorithm) combination
//	@Tags         Gold Predictions
//	@Produce      json
//	@Success      200  {object}  goldPredictionsResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/gold/predictions/latest-results [get]
func GetGoldPredictionsLatestResults(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()
	preds, err := store.GetLatestConfirmedGoldPredictions()
	if err != nil {
		logger.Logger.Errorf("[api/gold/predictions/latest-results] Failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest confirmed gold predictions")
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
			PredictionDate: p.PredictionDate.Format(time.RFC3339),
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

// GetGoldPredictionChart godoc
//
//	@Summary      Get gold prediction chart data
//	@Description  Returns chronologically ordered predicted vs actual price data for charting, optionally filtered by algorithm
//	@Tags         Gold Predictions
//	@Produce      json
//	@Param        source        query  string  false  "Gold source (e.g. SJC, BTMC)"
//	@Param        product_type  query  string  false  "Product type (e.g. 1l, nhan_tron)"
//	@Param        algorithm     query  string  false  "Algorithm name (e.g. lstm_nn)"
//	@Param        days          query  int     false  "Number of days to look back (default 30)"
//	@Success      200           {object}  goldPredictionChartResponse
//	@Failure      500           {object}  ResponseFailure
//	@Router       /api/gold/predictions/chart [get]
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
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

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
	// DB returns prediction_date ASC — iterate forward
	for _, p := range filtered {
		data = append(data, goldPredictionChartPoint{
			Date:           p.TargetDate.Format("2006-01-02T15:04:05"),
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

// TriggerGoldPredictHandler godoc
//
//	@Summary      Trigger gold prediction
//	@Description  Runs a prediction for all gold instruments (XAU/spot, BTMC/sjc, BTMC/nhan_tron) in the background via the prediction service.
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]string
//	@Failure      401  {object}  ResponseFailure
//	@Failure      500  {object}  ResponseFailure
//	@Failure      503  {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/gold-predict [post]
func TriggerGoldPredictHandler(w http.ResponseWriter, r *http.Request) {
	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerGoldPredict(r.Context(), &pb.Empty{})
	if err != nil {
		logger.Logger.Errorf("Manual gold prediction failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(w, http.StatusOK, map[string]string{"message": "Gold prediction triggered"})
}
