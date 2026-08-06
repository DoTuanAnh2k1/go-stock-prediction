package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

// sp500LatestItem represents a single entry in the /api/sp500/latest response.
type sp500LatestItem struct {
	Symbol      string          `json:"symbol"`
	CompanyName string          `json:"company_name"`
	ClosePrice  decimal.Decimal `json:"close_price"`
	TradingDate string          `json:"trading_date"`
	Currency    string          `json:"currency"`
}

type sp500LatestResponse struct {
	Data      []sp500LatestItem `json:"data"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// GetSP500Latest godoc
//
//	@Summary      Get latest S&P 500 prices
//	@Description  Returns the most recent closing price for each tracked S&P 500 symbol
//	@Tags         SP500
//	@Produce      json
//	@Success      200  {object}  sp500LatestResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/sp500/latest [get]
func GetSP500Latest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()

	ctx := r.Context()
	symbols, err := store.GetSP500Symbols(ctx)
	if err != nil {
		logger.Ctx(ctx).Errorf("[api/sp500/latest] Failed to get S&P 500 symbols: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get S&P 500 symbols")
		return
	}

	items := make([]sp500LatestItem, 0, len(symbols))
	for _, sym := range symbols {
		p, err := store.GetLatestSP500Price(ctx, sym)
		if err != nil || p == nil {
			continue
		}
		items = append(items, sp500LatestItem{
			Symbol:      p.Symbol,
			CompanyName: p.CompanyName,
			ClosePrice:  p.ClosePrice,
			TradingDate: p.TradingDate.Format("2006-01-02"),
			Currency:    p.Currency,
		})
	}

	ResponseSuccess(w, http.StatusOK, sp500LatestResponse{
		Data:      items,
		UpdatedAt: time.Now().UTC(),
	})
}

// sp500PricePoint is a single date entry for historical S&P 500 prices.
type sp500PricePoint struct {
	Date       string          `json:"date"`
	OpenPrice  decimal.Decimal `json:"open_price"`
	HighPrice  decimal.Decimal `json:"high_price"`
	LowPrice   decimal.Decimal `json:"low_price"`
	ClosePrice decimal.Decimal `json:"close_price"`
	Volume     int64           `json:"volume"`
}

type sp500PricesResponse struct {
	Symbol string            `json:"symbol"`
	Data   []sp500PricePoint `json:"data"`
}

// GetSP500Prices godoc
//
//	@Summary      Get historical S&P 500 prices
//	@Description  Returns OHLCV price data for a given S&P 500 symbol over the specified number of days
//	@Tags         SP500
//	@Produce      json
//	@Param        symbol  query  string  false  "S&P 500 symbol (e.g. SPY, AAPL)"
//	@Param        days    query  int     false  "Number of days to look back (default 30)"
//	@Success      200     {object}  sp500PricesResponse
//	@Failure      500     {object}  ResponseFailure
//	@Router       /api/sp500/prices [get]
func GetSP500Prices(w http.ResponseWriter, r *http.Request) {
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
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

	ctx := r.Context()
	prices, err := store.GetSP500PricesByDateRange(ctx, symbol, from, to)
	if err != nil {
		logger.Ctx(ctx).Errorf("[api/sp500/prices] Failed to get S&P 500 prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get S&P 500 prices")
		return
	}

	data := make([]sp500PricePoint, 0, len(prices))
	for _, p := range prices {
		data = append(data, sp500PricePoint{
			Date:       p.TradingDate.Format("2006-01-02"),
			OpenPrice:  p.OpenPrice,
			HighPrice:  p.HighPrice,
			LowPrice:   p.LowPrice,
			ClosePrice: p.ClosePrice,
			Volume:     p.Volume,
		})
	}

	ResponseSuccess(w, http.StatusOK, sp500PricesResponse{
		Symbol: symbol,
		Data:   data,
	})
}

type sp500ChartResponse struct {
	Symbol      string            `json:"symbol"`
	Dates       []string          `json:"dates"`
	Prices      []decimal.Decimal `json:"prices"`
	Opens       []decimal.Decimal `json:"opens"`
	Highs       []decimal.Decimal `json:"highs"`
	Lows        []decimal.Decimal `json:"lows"`
	Closes      []decimal.Decimal `json:"closes"`
	Granularity string            `json:"granularity"`
}

// GetSP500Chart godoc
//
//	@Summary      Get S&P 500 price chart data
//	@Description  Returns chronologically ordered date labels and closing prices for charting. When days=1, returns hourly intraday data; otherwise returns daily data.
//	@Tags         SP500
//	@Produce      json
//	@Param        symbol  query  string  false  "S&P 500 symbol (e.g. SPY, AAPL)"
//	@Param        days    query  int     false  "Number of days to look back (default 30); use 1 for intraday hourly data"
//	@Success      200     {object}  sp500ChartResponse
//	@Failure      500     {object}  ResponseFailure
//	@Router       /api/sp500/chart [get]
func GetSP500Chart(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	daysStr := r.URL.Query().Get("days")

	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	store := repository.GetSingleton()
	ctx := r.Context()
	to := time.Now()

	if days == 1 {
		// Use 96h window so weekends show the last trading day (Friday US close = Saturday ~03:00 VN)
		from := to.Add(-96 * time.Hour)
		intradayPrices, err := store.GetSP500IntradayByRange(ctx, symbol, from, to)
		if err != nil {
			logger.Ctx(ctx).Errorf("[api/sp500/chart] Failed to get S&P 500 intraday prices: %v", err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get S&P 500 intraday prices")
			return
		}

		n := len(intradayPrices)
		dates := make([]string, 0, n)
		closePrices := make([]decimal.Decimal, 0, n)
		opens := make([]decimal.Decimal, 0, n)
		highs := make([]decimal.Decimal, 0, n)
		lows := make([]decimal.Decimal, 0, n)
		closes := make([]decimal.Decimal, 0, n)

		// intraday prices are ordered DESC from DB — reverse for chart (oldest first)
		for i := n - 1; i >= 0; i-- {
			p := intradayPrices[i]
			dates = append(dates, p.Timestamp.Format("2006-01-02 15:04"))
			closePrices = append(closePrices, p.ClosePrice)
			opens = append(opens, p.OpenPrice)
			highs = append(highs, p.HighPrice)
			lows = append(lows, p.LowPrice)
			closes = append(closes, p.ClosePrice)
		}

		ResponseSuccess(w, http.StatusOK, sp500ChartResponse{
			Symbol:      symbol,
			Dates:       dates,
			Prices:      closePrices,
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

	prices, err := store.GetSP500PricesByDateRange(ctx, symbol, from, to)
	if err != nil {
		logger.Ctx(ctx).Errorf("[api/sp500/chart] Failed to get S&P 500 prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get S&P 500 prices")
		return
	}

	n := len(prices)
	dates := make([]string, 0, n)
	closePrices := make([]decimal.Decimal, 0, n)
	opens := make([]decimal.Decimal, 0, n)
	highs := make([]decimal.Decimal, 0, n)
	lows := make([]decimal.Decimal, 0, n)
	closes := make([]decimal.Decimal, 0, n)

	// prices are ordered DESC from DB — reverse for chart (oldest first)
	for i := n - 1; i >= 0; i-- {
		p := prices[i]
		dates = append(dates, p.TradingDate.Format("2006-01-02"))
		closePrices = append(closePrices, p.ClosePrice)
		opens = append(opens, p.OpenPrice)
		highs = append(highs, p.HighPrice)
		lows = append(lows, p.LowPrice)
		closes = append(closes, p.ClosePrice)
	}

	ResponseSuccess(w, http.StatusOK, sp500ChartResponse{
		Symbol:      symbol,
		Dates:       dates,
		Prices:      closePrices,
		Opens:       opens,
		Highs:       highs,
		Lows:        lows,
		Closes:      closes,
		Granularity: "1d",
	})
}
