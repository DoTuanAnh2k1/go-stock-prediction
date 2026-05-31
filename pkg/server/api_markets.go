package server

import (
	"math"
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"

	"github.com/shopspring/decimal"
)

// -----------------------------------------------------------------------
// Response types
// -----------------------------------------------------------------------

type marketPredictionsResponse struct {
	Market     string        `json:"market"`
	Data       interface{}   `json:"data"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalPages int           `json:"total_pages"`
}

type marketTrainingResponse struct {
	Market     string      `json:"market"`
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// vn30PredictionItem is the per-row DTO for VN30 market predictions.
type vn30PredictionItem struct {
	ID             uint             `json:"id"`
	Symbol         string           `json:"symbol"`
	CompanyName    string           `json:"company_name"`
	AlgorithmName  string           `json:"algorithm_name"`
	PredictedPrice decimal.Decimal  `json:"predicted_price"`
	CurrentPrice   decimal.Decimal  `json:"current_price"`
	Confidence     decimal.Decimal  `json:"confidence"`
	PredictionDate string           `json:"prediction_date"`
	TargetDate     string           `json:"target_date"`
	ActualPrice    *decimal.Decimal `json:"actual_price"`
	Accuracy       *decimal.Decimal `json:"accuracy"`
	Status         string           `json:"status"`
}

// goldPredictionPageItem is the per-row DTO for gold market predictions.
type goldPredictionPageItem struct {
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

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func parsePaginationParams(r *http.Request) (page, limit int) {
	page = 1
	limit = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v >= 1 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	return page, limit
}

func totalPages(total int64, limit int) int {
	if limit <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(limit)))
}

// -----------------------------------------------------------------------
// GET /api/markets/{key}/predictions
// -----------------------------------------------------------------------

// GetMarketPredictions godoc
//
//	@Summary      List predictions for a market
//	@Description  Returns a paginated list of predictions for the specified market (vn30 or gold). Supports filtering by algorithm, status, and free-text search. Sortable by any field.
//	@Tags         Markets
//	@Produce      json
//	@Param        key        path      string  true   "Market key: vn30 or gold"
//	@Param        page       query     int     false  "Page number (default 1)"
//	@Param        limit      query     int     false  "Page size (1-100, default 20)"
//	@Param        search     query     string  false  "Free-text search by symbol or company name"
//	@Param        algorithm  query     string  false  "Filter by algorithm key (e.g. lstm_nn)"
//	@Param        status     query     string  false  "Filter by prediction status (e.g. confirmed, pending)"
//	@Param        sort_by    query     string  false  "Sort field (default prediction_date)"
//	@Param        sort_dir   query     string  false  "Sort direction: asc or desc (default desc)"
//	@Success      200        {object}  marketPredictionsResponse
//	@Failure      404        {object}  ResponseFailure
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/markets/{key}/predictions [get]
func GetMarketPredictions(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	page, limit := parsePaginationParams(r)
	search := r.URL.Query().Get("search")
	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")
	algorithm := r.URL.Query().Get("algorithm")
	status := r.URL.Query().Get("status")

	if sortBy == "" {
		sortBy = "prediction_date"
	}
	if sortDir == "" {
		sortDir = "desc"
	}

	store := repository.GetSingleton()

	switch key {
	case "vn30":
		preds, total, err := store.GetPredictionsByMarketPage(key, page, limit, search, sortBy, sortDir, algorithm, status)
		if err != nil {
			logger.Logger.Errorf("[api/markets/%s/predictions] DB error: %v", key, err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
			return
		}

		items := make([]vn30PredictionItem, 0, len(preds))
		for _, p := range preds {
			symbol := ""
			companyName := ""
			if p.Stock != nil {
				symbol = p.Stock.Symbol
				companyName = p.Stock.CompanyName
			}
			items = append(items, vn30PredictionItem{
				ID:             p.ID,
				Symbol:         symbol,
				CompanyName:    companyName,
				AlgorithmName:  p.AlgorithmName,
				PredictedPrice: p.PredictedPrice,
				CurrentPrice:   p.CurrentPrice,
				Confidence:     p.Confidence,
				PredictionDate: p.PredictionDate.Format("2006-01-02"),
				TargetDate:     p.TargetDate.Format("2006-01-02"),
				ActualPrice:    p.ActualPrice,
				Accuracy:       p.Accuracy,
				Status:         p.Status,
			})
		}

		ResponseSuccess(w, http.StatusOK, marketPredictionsResponse{
			Market:     key,
			Data:       items,
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages(total, limit),
		})

	case "gold":
		preds, total, err := store.GetGoldPredictionsPage(page, limit, search, algorithm, status, sortBy, sortDir)
		if err != nil {
			logger.Logger.Errorf("[api/markets/%s/predictions] DB error: %v", key, err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get predictions")
			return
		}

		items := make([]goldPredictionPageItem, 0, len(preds))
		for _, p := range preds {
			items = append(items, goldPredictionPageItem{
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

		ResponseSuccess(w, http.StatusOK, marketPredictionsResponse{
			Market:     key,
			Data:       items,
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages(total, limit),
		})

	default:
		ResponseError(w, http.StatusNotFound, "unknown market key: "+key)
	}
}

// -----------------------------------------------------------------------
// GET /api/markets/{key}/training
// -----------------------------------------------------------------------

// GetMarketTraining godoc
//
//	@Summary      List training sessions for a market
//	@Description  Returns a paginated list of training sessions for the specified market (vn30 or gold). Supports filtering by algorithm and sorting.
//	@Tags         Markets
//	@Produce      json
//	@Param        key        path      string  true   "Market key: vn30 or gold"
//	@Param        page       query     int     false  "Page number (default 1)"
//	@Param        limit      query     int     false  "Page size (1-100, default 20)"
//	@Param        algorithm  query     string  false  "Filter by algorithm key (e.g. lstm_nn)"
//	@Param        sort_by    query     string  false  "Sort field (default started_at)"
//	@Param        sort_dir   query     string  false  "Sort direction: asc or desc (default desc)"
//	@Success      200        {object}  marketTrainingResponse
//	@Failure      404        {object}  ResponseFailure
//	@Failure      500        {object}  ResponseFailure
//	@Router       /api/markets/{key}/training [get]
func GetMarketTraining(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	// Validate market key
	if key != "vn30" && key != "gold" {
		ResponseError(w, http.StatusNotFound, "unknown market key: "+key)
		return
	}

	page, limit := parsePaginationParams(r)
	sortBy := r.URL.Query().Get("sort_by")
	sortDir := r.URL.Query().Get("sort_dir")
	algorithm := r.URL.Query().Get("algorithm")

	if sortBy == "" {
		sortBy = "started_at"
	}
	if sortDir == "" {
		sortDir = "desc"
	}

	store := repository.GetSingleton()
	logs, total, err := store.GetTrainingSessionsByMarket(key, page, limit, algorithm, sortBy, sortDir)
	if err != nil {
		logger.Logger.Errorf("[api/markets/%s/training] DB error: %v", key, err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get training sessions")
		return
	}

	ResponseSuccess(w, http.StatusOK, marketTrainingResponse{
		Market:     key,
		Data:       logs,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages(total, limit),
	})
}
