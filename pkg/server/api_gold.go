package server

import (
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// goldLatestItem represents a single entry in the /api/gold/latest response.
type goldLatestItem struct {
	Source      string          `json:"source"`
	ProductType string          `json:"product_type"`
	BuyPrice    decimal.Decimal `json:"buy_price"`
	SellPrice   decimal.Decimal `json:"sell_price"`
	Currency    string          `json:"currency"`
	TradingDate string          `json:"trading_date"`
}

type goldLatestResponse struct {
	Data      []goldLatestItem `json:"data"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// GetGoldLatest godoc
//
//	@Summary      Get latest gold prices
//	@Description  Returns the most recent buy/sell price for each gold source and product type
//	@Tags         Gold
//	@Produce      json
//	@Success      200  {object}  goldLatestResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/gold/latest [get]
func GetGoldLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()

	prices, err := store.GetLatestGoldPrices()
	if err != nil {
		logger.Logger.Errorf("[api/gold/latest] Failed to get latest gold prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get latest gold prices")
		return
	}

	items := make([]goldLatestItem, 0, len(prices))
	for _, p := range prices {
		items = append(items, goldLatestItem{
			Source:      p.Source,
			ProductType: p.ProductType,
			BuyPrice:    p.BuyPrice,
			SellPrice:   p.SellPrice,
			Currency:    p.Currency,
			TradingDate: p.TradingDate.Format("2006-01-02"),
		})
	}

	ResponseSuccess(w, http.StatusOK, goldLatestResponse{
		Data:      items,
		UpdatedAt: time.Now().UTC(),
	})
}

// goldPricePoint is a single date entry for historical prices.
type goldPricePoint struct {
	Date      string          `json:"date"`
	BuyPrice  decimal.Decimal `json:"buy_price"`
	SellPrice decimal.Decimal `json:"sell_price"`
}

type goldPricesResponse struct {
	Source      string           `json:"source"`
	ProductType string           `json:"product_type"`
	Data        []goldPricePoint `json:"data"`
}

// GetGoldPrices godoc
//
//	@Summary      Get historical gold prices
//	@Description  Returns buy/sell prices for a given gold source and product type over the specified number of days
//	@Tags         Gold
//	@Produce      json
//	@Param        source        query  string  false  "Gold source (e.g. SJC, BTMC)"
//	@Param        product_type  query  string  false  "Product type (e.g. 1l, nhan_tron)"
//	@Param        days          query  int     false  "Number of days to look back (default 30)"
//	@Success      200           {object}  goldPricesResponse
//	@Failure      500           {object}  ResponseFailure
//	@Router       /api/gold/prices [get]
func GetGoldPrices(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	productType := r.URL.Query().Get("product_type")
	daysStr := r.URL.Query().Get("days")

	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	store := repository.GetSingleton()

	to := time.Now()
	from := to.AddDate(0, 0, -days)
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

	prices, err := store.GetGoldPricesByDateRange(source, productType, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/gold/prices] Failed to get gold prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get gold prices")
		return
	}

	data := make([]goldPricePoint, 0, len(prices))
	for _, p := range prices {
		data = append(data, goldPricePoint{
			Date:      p.TradingDate.Format("2006-01-02"),
			BuyPrice:  p.BuyPrice,
			SellPrice: p.SellPrice,
		})
	}

	ResponseSuccess(w, http.StatusOK, goldPricesResponse{
		Source:      source,
		ProductType: productType,
		Data:        data,
	})
}

type goldChartResponse struct {
	Labels      []string          `json:"labels"`
	BuyPrices   []decimal.Decimal `json:"buy_prices"`
	SellPrices  []decimal.Decimal `json:"sell_prices"`
	Granularity string            `json:"granularity"`
}

// GetGoldChart godoc
//
//	@Summary      Get gold price chart data
//	@Description  Returns chronologically ordered labels and buy/sell price arrays suitable for charting. When days=1, returns hourly intraday data; otherwise returns daily data.
//	@Tags         Gold
//	@Produce      json
//	@Param        source        query  string  false  "Gold source (e.g. SJC, BTMC)"
//	@Param        product_type  query  string  false  "Product type (e.g. 1l, nhan_tron)"
//	@Param        days          query  int     false  "Number of days to look back (default 30); use 1 for intraday hourly data"
//	@Success      200           {object}  goldChartResponse
//	@Failure      500           {object}  ResponseFailure
//	@Router       /api/gold/chart [get]
func GetGoldChart(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	productType := r.URL.Query().Get("product_type")
	daysStr := r.URL.Query().Get("days")

	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	store := repository.GetSingleton()
	to := time.Now()

	if days == 1 {
		from := to.Add(-24 * time.Hour)
		intradayPrices, err := store.GetGoldIntradayByRange(source, from, to)
		if err != nil {
			logger.Logger.Errorf("[api/gold/chart] Failed to get gold intraday prices: %v", err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get gold intraday prices")
			return
		}

		labels := make([]string, 0, len(intradayPrices))
		buyPrices := make([]decimal.Decimal, 0, len(intradayPrices))
		sellPrices := make([]decimal.Decimal, 0, len(intradayPrices))

		// intraday prices are ordered DESC from DB — reverse for chart (oldest first)
		for i := len(intradayPrices) - 1; i >= 0; i-- {
			p := intradayPrices[i]
			labels = append(labels, p.Timestamp.Format("2006-01-02 15:04"))
			buyPrices = append(buyPrices, p.BuyPrice)
			sellPrices = append(sellPrices, p.SellPrice)
		}

		ResponseSuccess(w, http.StatusOK, goldChartResponse{
			Labels:      labels,
			BuyPrices:   buyPrices,
			SellPrices:  sellPrices,
			Granularity: "1h",
		})
		return
	}

	from := to.AddDate(0, 0, -days)
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

	prices, err := store.GetGoldPricesByDateRange(source, productType, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/gold/chart] Failed to get gold prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get gold prices")
		return
	}

	labels := make([]string, 0, len(prices))
	buyPrices := make([]decimal.Decimal, 0, len(prices))
	sellPrices := make([]decimal.Decimal, 0, len(prices))

	// prices are ordered DESC from DB — reverse for chart (oldest first)
	for i := len(prices) - 1; i >= 0; i-- {
		p := prices[i]
		labels = append(labels, p.TradingDate.Format("2006-01-02"))
		buyPrices = append(buyPrices, p.BuyPrice)
		sellPrices = append(sellPrices, p.SellPrice)
	}

	ResponseSuccess(w, http.StatusOK, goldChartResponse{
		Labels:      labels,
		BuyPrices:   buyPrices,
		SellPrices:  sellPrices,
		Granularity: "1d",
	})
}
