package server

import (
	"net/http"
	"strings"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

// directionAccuracyAlgoDTO holds per-algorithm direction accuracy data.
type directionAccuracyAlgoDTO struct {
	Algorithm         string  `json:"algorithm"`
	DirectionAccuracy float64 `json:"direction_accuracy"`
	Total             int64   `json:"total"`
	Correct           int64   `json:"correct"`
}

// directionAccuracyResponseDTO is the JSON envelope returned by the endpoint.
type directionAccuracyResponseDTO struct {
	Market     string                      `json:"market"`
	Algorithms []directionAccuracyAlgoDTO  `json:"algorithms"`
}

// GetDirectionAccuracy returns per-algorithm direction accuracy stats for a market.
//
// @Summary      Direction accuracy per algorithm
// @Description  Returns the fraction of predictions where the price movement direction was correct, grouped by algorithm, for the specified market.
// @Tags         predictions
// @Produce      json
// @Param        market  query  string  true  "Market key"  Enums(GOLD,NASDAQ,CRYPTO,SP500)
// @Success      200  {object}  directionAccuracyResponseDTO
// @Failure      400  {object}  ResponseFailure  "missing or unknown market"
// @Failure      500  {object}  ResponseFailure
// @Router       /api/predictions/direction-accuracy [get]
func GetDirectionAccuracy(w http.ResponseWriter, r *http.Request) {
	market := strings.TrimSpace(r.URL.Query().Get("market"))
	if market == "" {
		ResponseError(w, http.StatusBadRequest, "query param 'market' is required (GOLD, NASDAQ, CRYPTO, SP500)")
		return
	}

	db := repository.GetSingleton()
	rows, err := db.GetDirectionAccuracy(market)
	if err != nil {
		logger.Logger.Errorf("GetDirectionAccuracy(%s): %v", market, err)
		// Distinguish "unknown market" (user error) from real DB errors.
		if strings.Contains(err.Error(), "unknown market") {
			ResponseError(w, http.StatusBadRequest, err.Error())
			return
		}
		ResponseError(w, http.StatusInternalServerError, "failed to query direction accuracy")
		return
	}

	algos := make([]directionAccuracyAlgoDTO, 0, len(rows))
	for _, row := range rows {
		var acc float64
		if row.Total > 0 {
			acc = float64(row.Correct) / float64(row.Total)
		}
		algos = append(algos, directionAccuracyAlgoDTO{
			Algorithm:         row.Algorithm,
			DirectionAccuracy: acc,
			Total:             row.Total,
			Correct:           row.Correct,
		})
	}

	resp := directionAccuracyResponseDTO{
		Market:     strings.ToUpper(market),
		Algorithms: algos,
	}
	ResponseSuccess(w, http.StatusOK, resp)
}
