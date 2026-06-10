package server

import (
	"go-stock-prediction/pkg/logger"
	modelsapi "go-stock-prediction/pkg/models/models_api"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strings"
	"time"
)

// GetStockWatchlist godoc
//
//	@Summary      Get current price data for a watchlist of stocks
//	@Description  Returns current price, change, volume and market status for a comma-separated list of stock symbols.
//	@Tags         Stocks
//	@Produce      json
//	@Param        symbols query string true "Comma-separated stock symbols (e.g. VCB,VIC,FPT)"
//	@Success      200 {object} modelsapi.StockWatchlistDTO
//	@Failure      400 {object} ResponseFailure
//	@Router       /api/stocks/watchlist [get]
func GetStockWatchlist(w http.ResponseWriter, r *http.Request) {
	symbolsParam := r.URL.Query().Get("symbols")
	if symbolsParam == "" {
		ResponseError(w, http.StatusBadRequest, "Symbols parameter required")
		return
	}

	symbols := strings.Split(strings.ToUpper(symbolsParam), ",")
	logger.Logger.Infof("👀 Getting watchlist for symbols: %v", symbols)

	store := repository.GetSingleton()
	var watchlistStocks []modelsapi.StockCurrentPriceDTO

	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		// Get stock info
		stock, err := store.GetStockBySymbol(symbol)
		if err != nil {
			logger.Logger.Warnf("Stock %s not found", symbol)
			continue
		}

		// Get latest price
		latestPrice, err := store.GetLatestStockPriceByStockID(stock.ID)
		if err != nil {
			logger.Logger.Warnf("No price data for %s", symbol)
			continue
		}

		currentPrice := modelsapi.StockCurrentPriceDTO{
			Stock: modelsapi.StockDTO{
				ID:          stock.ID,
				Symbol:      stock.Symbol,
				CompanyName: stock.CompanyName,
				ExchangeID:  stock.ExchangeID,
				IsVN30:      stock.IsVN30,
				IsVN100:     stock.IsVN100,
				Sector:      stock.Sector,
			},
			CurrentPrice:  latestPrice.ClosePrice,
			Change:        latestPrice.Change,
			ChangePercent: latestPrice.ChangePercent,
			Volume:        latestPrice.Volume,
			Value:         latestPrice.Value,
			High:          latestPrice.HighPrice,
			Low:           latestPrice.LowPrice,
			Open:          latestPrice.OpenPrice,
			TradingDate:   latestPrice.TradingDate,
			LastUpdated:   latestPrice.UpdatedAt,
			MarketStatus:  getMarketStatus(),
		}

		watchlistStocks = append(watchlistStocks, currentPrice)
	}

	watchlist := &modelsapi.StockWatchlistDTO{
		Stocks:      watchlistStocks,
		Total:       len(watchlistStocks),
		LastUpdated: time.Now(),
	}

	ResponseSuccess(w, http.StatusOK, watchlist)
}
