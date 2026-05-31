package server

import (
	"go-stock-prediction/pkg/logger"
	pb "go-stock-prediction/proto/prediction"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
)

// TriggerStockCrawl godoc
//
//	@Summary      Crawl latest data for a single stock
//	@Description  Triggers the prediction service to crawl and save the latest price data for the given stock symbol.
//	@Tags         Stocks
//	@Accept       json
//	@Produce      json
//	@Param        symbol path string true "Stock symbol (e.g. VCB)"
//	@Success      200 {object} map[string]interface{}
//	@Failure      400 {object} ResponseFailure
//	@Failure      404 {object} ResponseFailure
//	@Failure      500 {object} ResponseFailure
//	@Router       /api/stocks/{symbol}/crawl [post]
func TriggerStockCrawl(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ResponseError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	symbol := strings.ToUpper(pathParts[3])

	if err := validateSymbol(symbol); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify stock exists in DB
	store := repository.GetSingleton()
	_, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Không tìm thấy cổ phiếu: "+symbol)
		return
	}

	logger.Logger.Infof("Triggering crawl for stock: %s", symbol)

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	resp, err := client.TriggerStockCrawl(r.Context(), &pb.StockRequest{Symbol: symbol})
	if err != nil {
		logger.Logger.Errorf("Failed to crawl %s: %v", symbol, err)
		ResponseError(w, http.StatusInternalServerError, "Thu thập dữ liệu thất bại: "+err.Error())
		return
	}

	// Invalidate cache for this stock
	globalCache.Delete("stock_detail:" + symbol)

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"symbol":  resp.GetSymbol(),
		"message": resp.GetMessage(),
	})
}

// TriggerStockPredict godoc
//
//	@Summary      Run all prediction algorithms for a single stock
//	@Description  Triggers the prediction service to run all configured prediction algorithms synchronously for the given stock symbol.
//	@Tags         Stocks
//	@Accept       json
//	@Produce      json
//	@Param        symbol path string true "Stock symbol (e.g. VCB)"
//	@Success      200 {object} map[string]interface{}
//	@Failure      400 {object} ResponseFailure
//	@Failure      404 {object} ResponseFailure
//	@Failure      500 {object} ResponseFailure
//	@Router       /api/stocks/{symbol}/predict [post]
func TriggerStockPredict(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		ResponseError(w, http.StatusBadRequest, "Invalid path")
		return
	}
	symbol := strings.ToUpper(pathParts[3])

	if err := validateSymbol(symbol); err != nil {
		ResponseError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Verify stock exists in DB
	store := repository.GetSingleton()
	_, err := store.GetStockBySymbol(symbol)
	if err != nil {
		ResponseError(w, http.StatusNotFound, "Không tìm thấy cổ phiếu: "+symbol)
		return
	}

	logger.Logger.Infof("Triggering prediction for stock: %s", symbol)

	client := requireGRPCClient(w)
	if client == nil {
		return
	}
	resp, err := client.TriggerStockPredict(r.Context(), &pb.StockRequest{Symbol: symbol})
	if err != nil {
		logger.Logger.Errorf("Failed to predict %s: %v", symbol, err)
		ResponseError(w, http.StatusInternalServerError, "Dự đoán thất bại: "+err.Error())
		return
	}

	// Invalidate cache for this stock
	globalCache.Delete("stock_detail:" + symbol)

	ResponseSuccess(w, http.StatusOK, map[string]interface{}{
		"symbol":            resp.GetSymbol(),
		"message":           resp.GetMessage(),
		"predictions_count": resp.GetPredictionsCount(),
	})
}
