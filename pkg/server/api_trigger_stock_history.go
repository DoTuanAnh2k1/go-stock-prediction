package server

import (
	"net/http"
	"strconv"

	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
)

const (
	defaultHistoricalDays = 365
	maxHistoricalDays     = 1000
)

// TriggerStockHistoryHandler godoc
//
//	@Summary      Trigger historical stock price crawl
//	@Description  Starts a historical VN30 stock price crawl in the background. Accepts an optional days query parameter (default 365, max 1000).
//	@Tags         Triggers
//	@Accept       json
//	@Produce      json
//	@Param        days  query     int  false  "Number of historical days to crawl (default 365, max 1000)"
//	@Success      202   {object}  map[string]interface{}
//	@Failure      400   {object}  ResponseFailure
//	@Failure      401   {object}  ResponseFailure
//	@Failure      500   {object}  ResponseFailure
//	@Failure      503   {object}  ResponseFailure
//	@Security     BearerAuth
//	@Router       /api/trigger/stock-history [post]
func TriggerStockHistoryHandler(w http.ResponseWriter, r *http.Request) {
	days := defaultHistoricalDays

	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			ResponseError(w, http.StatusBadRequest, "invalid days parameter: must be a positive integer")
			return
		}
		if n > maxHistoricalDays {
			n = maxHistoricalDays
		}
		days = n
	}

	logger.Logger.Infof("[trigger] stock-history requested: days=%d", days)

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	_, err := client.TriggerStockHistory(r.Context(), &pb.StockHistoryRequest{Days: int32(days)})
	if err != nil {
		logger.Logger.Errorf("[trigger] stock-history failed: %v", err)
		ResponseError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(w, http.StatusAccepted, map[string]interface{}{
		"status":  "started",
		"days":    days,
		"message": "Historical stock price crawl started in background",
	})
}
