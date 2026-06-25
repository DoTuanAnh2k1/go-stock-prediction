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
	Opens       []decimal.Decimal `json:"opens"`
	Highs       []decimal.Decimal `json:"highs"`
	Lows        []decimal.Decimal `json:"lows"`
	Closes      []decimal.Decimal `json:"closes"`
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

	isXAU := source == "XAU"

	if days == 1 {
		from := to.Add(-24 * time.Hour)
		intradayPrices, err := store.GetGoldIntradayByRange(source, from, to)
		if err != nil {
			logger.Logger.Errorf("[api/gold/chart] Failed to get gold intraday prices: %v", err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get gold intraday prices")
			return
		}

		n := len(intradayPrices)
		labels := make([]string, 0, n)
		buyPrices := make([]decimal.Decimal, 0, n)
		sellPrices := make([]decimal.Decimal, 0, n)
		opens := make([]decimal.Decimal, 0, n)
		highs := make([]decimal.Decimal, 0, n)
		lows := make([]decimal.Decimal, 0, n)
		closes := make([]decimal.Decimal, 0, n)
		allNilOHLC := true

		// intraday prices are ordered DESC from DB — reverse for chart (oldest first)
		for i := n - 1; i >= 0; i-- {
			p := intradayPrices[i]
			labels = append(labels, p.Timestamp.Format("2006-01-02 15:04"))
			buyPrices = append(buyPrices, p.BuyPrice)
			sellPrices = append(sellPrices, p.SellPrice)
			// OHLC only available for XAU source; sell_price acts as close
			if isXAU {
				closes = append(closes, p.SellPrice)
				if p.OpenPrice != nil && p.HighPrice != nil && p.LowPrice != nil {
					allNilOHLC = false
					opens = append(opens, *p.OpenPrice)
					highs = append(highs, *p.HighPrice)
					lows = append(lows, *p.LowPrice)
				} else {
					// doji fallback: fill with close to keep alignment
					opens = append(opens, p.SellPrice)
					highs = append(highs, p.SellPrice)
					lows = append(lows, p.SellPrice)
				}
			}
		}

		// non-XAU sources or XAU with all-nil OHLC → return empty OHLC arrays
		if !isXAU || allNilOHLC {
			opens = []decimal.Decimal{}
			highs = []decimal.Decimal{}
			lows = []decimal.Decimal{}
			closes = []decimal.Decimal{}
		}

		ResponseSuccess(w, http.StatusOK, goldChartResponse{
			Labels:      labels,
			BuyPrices:   buyPrices,
			SellPrices:  sellPrices,
			Opens:       opens,
			Highs:       highs,
			Lows:        lows,
			Closes:      closes,
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

	n := len(prices)
	labels := make([]string, 0, n)
	buyPrices := make([]decimal.Decimal, 0, n)
	sellPrices := make([]decimal.Decimal, 0, n)
	opens := make([]decimal.Decimal, 0, n)
	highs := make([]decimal.Decimal, 0, n)
	lows := make([]decimal.Decimal, 0, n)
	closes := make([]decimal.Decimal, 0, n)
	allNilOHLC := true

	// prices are ordered DESC from DB — reverse for chart (oldest first)
	for i := n - 1; i >= 0; i-- {
		p := prices[i]
		labels = append(labels, p.TradingDate.Format("2006-01-02"))
		buyPrices = append(buyPrices, p.BuyPrice)
		sellPrices = append(sellPrices, p.SellPrice)
		// OHLC only available for XAU source; sell_price acts as close
		if isXAU {
			closes = append(closes, p.SellPrice)
			if p.OpenPrice != nil && p.HighPrice != nil && p.LowPrice != nil {
				allNilOHLC = false
				opens = append(opens, *p.OpenPrice)
				highs = append(highs, *p.HighPrice)
				lows = append(lows, *p.LowPrice)
			} else {
				// doji fallback: fill with close to keep alignment
				opens = append(opens, p.SellPrice)
				highs = append(highs, p.SellPrice)
				lows = append(lows, p.SellPrice)
			}
		}
	}

	// non-XAU sources or XAU with all-nil OHLC → return empty OHLC arrays
	if !isXAU || allNilOHLC {
		opens = []decimal.Decimal{}
		highs = []decimal.Decimal{}
		lows = []decimal.Decimal{}
		closes = []decimal.Decimal{}
	}

	ResponseSuccess(w, http.StatusOK, goldChartResponse{
		Labels:      labels,
		BuyPrices:   buyPrices,
		SellPrices:  sellPrices,
		Opens:       opens,
		Highs:       highs,
		Lows:        lows,
		Closes:      closes,
		Granularity: "1d",
	})
}
