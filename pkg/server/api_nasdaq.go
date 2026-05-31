package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

// nasdaqLatestItem represents a single entry in the /api/nasdaq/latest response.
type nasdaqLatestItem struct {
	Symbol      string          `json:"symbol"`
	CompanyName string          `json:"company_name"`
	ClosePrice  decimal.Decimal `json:"close_price"`
	TradingDate string          `json:"trading_date"`
	Currency    string          `json:"currency"`
}

type nasdaqLatestResponse struct {
	Data      []nasdaqLatestItem `json:"data"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// GetNasdaqLatest godoc
//
//	@Summary      Get latest NASDAQ prices
//	@Description  Returns the most recent closing price for each tracked NASDAQ symbol
//	@Tags         NASDAQ
//	@Produce      json
//	@Success      200  {object}  nasdaqLatestResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/nasdaq/latest [get]
func GetNasdaqLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()

	symbols, err := store.GetNasdaqSymbols()
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/latest] Failed to get NASDAQ symbols: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get NASDAQ symbols")
		return
	}

	items := make([]nasdaqLatestItem, 0, len(symbols))
	for _, sym := range symbols {
		p, err := store.GetLatestNasdaqPrice(sym)
		if err != nil || p == nil {
			continue
		}
		items = append(items, nasdaqLatestItem{
			Symbol:      p.Symbol,
			CompanyName: p.CompanyName,
			ClosePrice:  p.ClosePrice,
			TradingDate: p.TradingDate.Format("2006-01-02"),
			Currency:    p.Currency,
		})
	}

	ResponseSuccess(w, http.StatusOK, nasdaqLatestResponse{
		Data:      items,
		UpdatedAt: time.Now().UTC(),
	})
}

// nasdaqPricePoint is a single date entry for historical NASDAQ prices.
type nasdaqPricePoint struct {
	Date       string          `json:"date"`
	OpenPrice  decimal.Decimal `json:"open_price"`
	HighPrice  decimal.Decimal `json:"high_price"`
	LowPrice   decimal.Decimal `json:"low_price"`
	ClosePrice decimal.Decimal `json:"close_price"`
	Volume     int64           `json:"volume"`
}

type nasdaqPricesResponse struct {
	Symbol string             `json:"symbol"`
	Data   []nasdaqPricePoint `json:"data"`
}

// GetNasdaqPrices godoc
//
//	@Summary      Get historical NASDAQ prices
//	@Description  Returns OHLCV price data for a given NASDAQ symbol over the specified number of days
//	@Tags         NASDAQ
//	@Produce      json
//	@Param        symbol  query  string  false  "NASDAQ symbol (e.g. AAPL, MSFT)"
//	@Param        days    query  int     false  "Number of days to look back (default 30)"
//	@Success      200     {object}  nasdaqPricesResponse
//	@Failure      500     {object}  ResponseFailure
//	@Router       /api/nasdaq/prices [get]
func GetNasdaqPrices(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
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

	prices, err := store.GetNasdaqPricesByDateRange(symbol, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/prices] Failed to get NASDAQ prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get NASDAQ prices")
		return
	}

	data := make([]nasdaqPricePoint, 0, len(prices))
	for _, p := range prices {
		data = append(data, nasdaqPricePoint{
			Date:       p.TradingDate.Format("2006-01-02"),
			OpenPrice:  p.OpenPrice,
			HighPrice:  p.HighPrice,
			LowPrice:   p.LowPrice,
			ClosePrice: p.ClosePrice,
			Volume:     p.Volume,
		})
	}

	ResponseSuccess(w, http.StatusOK, nasdaqPricesResponse{
		Symbol: symbol,
		Data:   data,
	})
}

type nasdaqChartResponse struct {
	Symbol string            `json:"symbol"`
	Dates  []string          `json:"dates"`
	Prices []decimal.Decimal `json:"prices"`
}

// GetNasdaqChart godoc
//
//	@Summary      Get NASDAQ price chart data
//	@Description  Returns chronologically ordered date labels and closing prices for charting
//	@Tags         NASDAQ
//	@Produce      json
//	@Param        symbol  query  string  false  "NASDAQ symbol (e.g. AAPL, MSFT)"
//	@Param        days    query  int     false  "Number of days to look back (default 30)"
//	@Success      200     {object}  nasdaqChartResponse
//	@Failure      500     {object}  ResponseFailure
//	@Router       /api/nasdaq/chart [get]
func GetNasdaqChart(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
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

	prices, err := store.GetNasdaqPricesByDateRange(symbol, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/nasdaq/chart] Failed to get NASDAQ prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get NASDAQ prices")
		return
	}

	dates := make([]string, 0, len(prices))
	closePrices := make([]decimal.Decimal, 0, len(prices))

	// prices are ordered DESC from DB — reverse for chart (oldest first)
	for i := len(prices) - 1; i >= 0; i-- {
		p := prices[i]
		dates = append(dates, p.TradingDate.Format("2006-01-02"))
		closePrices = append(closePrices, p.ClosePrice)
	}

	ResponseSuccess(w, http.StatusOK, nasdaqChartResponse{
		Symbol: symbol,
		Dates:  dates,
		Prices: closePrices,
	})
}
