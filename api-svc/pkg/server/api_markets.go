package server

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// pathToMarketKey normalises a URL market key to the canonical market key used
// for access-control and internal lookups.
func pathToMarketKey(key string) string {
	switch strings.ToUpper(key) {
	case "GOLD":
		return "GOLD"
	case "NASDAQ", "NASDAQ100":
		return "NASDAQ"
	case "CRYPTO":
		return "CRYPTO"
	case "SP500":
		return "SP500"
	default:
		return strings.ToUpper(key)
	}
}

// checkMarketAccess returns true if the authenticated caller may access the given
// market key. super_admin always passes; other roles require the market to appear
// in their accessible_markets JWT claim.
func checkMarketAccess(w http.ResponseWriter, r *http.Request, marketKey string) bool {
	claims := getClaims(r)
	if isSuperAdmin(claims) {
		return true
	}
	for _, m := range getAccessibleMarkets(claims) {
		if m == marketKey {
			return true
		}
	}
	ResponseError(w, http.StatusForbidden, "no access to market: "+marketKey)
	return false
}

// -----------------------------------------------------------------------
// GET /api/markets/{key}/predictions
// -----------------------------------------------------------------------

// GetMarketPredictions godoc
//
//	@Summary      List predictions for a market
//	@Description  Returns a paginated list of predictions for the specified market (gold, nasdaq, sp500, crypto). Supports filtering by algorithm, status, and free-text search. Sortable by any field.
//	@Tags         Markets
//	@Produce      json
//	@Param        key        path      string  true   "Market key: gold, nasdaq, sp500, crypto"
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

	marketKey := pathToMarketKey(key)
	if !checkMarketAccess(w, r, marketKey) {
		return
	}

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
	case "gold":
		preds, total, err := store.GetGoldPredictionsPage(r.Context(), page, limit, search, algorithm, status, sortBy, sortDir)
		if err != nil {
			logger.Ctx(r.Context()).Errorf("[api/markets/%s/predictions] DB error: %v", key, err)
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
				PredictionDate: p.PredictionDate.Format(time.RFC3339),
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
//	@Description  Returns a paginated list of training sessions for the specified market (gold, nasdaq, sp500, crypto). Supports filtering by algorithm and sorting.
//	@Tags         Markets
//	@Produce      json
//	@Param        key        path      string  true   "Market key: gold, nasdaq, sp500, crypto"
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

	marketKey := pathToMarketKey(key)
	if !checkMarketAccess(w, r, marketKey) {
		return
	}

	// Validate market key
	if key != "gold" {
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
	logs, total, err := store.GetTrainingSessionsByMarket(r.Context(), key, page, limit, algorithm, sortBy, sortDir)
	if err != nil {
		logger.Ctx(r.Context()).Errorf("[api/markets/%s/training] DB error: %v", key, err)
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
