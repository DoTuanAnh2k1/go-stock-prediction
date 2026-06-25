package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

// cryptoLatestItem represents a single entry in the /api/crypto/latest response.
type cryptoLatestItem struct {
	CoinID      string          `json:"coin_id"`
	Symbol      string          `json:"symbol"`
	ClosePrice  decimal.Decimal `json:"close_price"`
	MarketCap   decimal.Decimal `json:"market_cap"`
	Volume24h   decimal.Decimal `json:"volume_24h"`
	TradingDate string          `json:"trading_date"`
	Currency    string          `json:"currency"`
}

type cryptoLatestResponse struct {
	Data      []cryptoLatestItem `json:"data"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// GetCryptoLatest godoc
//
//	@Summary      Get latest crypto prices
//	@Description  Returns the most recent price, market cap, and 24h volume for each tracked cryptocurrency
//	@Tags         Crypto
//	@Produce      json
//	@Success      200  {object}  cryptoLatestResponse
//	@Failure      500  {object}  ResponseFailure
//	@Router       /api/crypto/latest [get]
func GetCryptoLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()

	coins, err := store.GetCryptoCoins()
	if err != nil {
		logger.Logger.Errorf("[api/crypto/latest] Failed to get crypto coins: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get crypto coins")
		return
	}

	items := make([]cryptoLatestItem, 0, len(coins))
	for _, c := range coins {
		p, err := store.GetLatestCryptoPrice(c.CoinID)
		if err != nil || p == nil {
			continue
		}
		items = append(items, cryptoLatestItem{
			CoinID:      p.CoinID,
			Symbol:      p.Symbol,
			ClosePrice:  p.ClosePrice,
			MarketCap:   p.MarketCap,
			Volume24h:   p.Volume24h,
			TradingDate: p.TradingDate.Format("2006-01-02"),
			Currency:    p.Currency,
		})
	}

	ResponseSuccess(w, http.StatusOK, cryptoLatestResponse{
		Data:      items,
		UpdatedAt: time.Now().UTC(),
	})
}

// cryptoPricePoint is a single date entry for historical crypto prices.
type cryptoPricePoint struct {
	Date       string          `json:"date"`
	ClosePrice decimal.Decimal `json:"close_price"`
	MarketCap  decimal.Decimal `json:"market_cap"`
	Volume24h  decimal.Decimal `json:"volume_24h"`
}

type cryptoPricesResponse struct {
	CoinID string             `json:"coin_id"`
	Data   []cryptoPricePoint `json:"data"`
}

// GetCryptoPrices godoc
//
//	@Summary      Get historical crypto prices
//	@Description  Returns close price, market cap, and 24h volume for a given coin over the specified number of days
//	@Tags         Crypto
//	@Produce      json
//	@Param        coin  query  string  false  "Coin ID (e.g. bitcoin, ethereum)"
//	@Param        days  query  int     false  "Number of days to look back (default 30)"
//	@Success      200   {object}  cryptoPricesResponse
//	@Failure      500   {object}  ResponseFailure
//	@Router       /api/crypto/prices [get]
func GetCryptoPrices(w http.ResponseWriter, r *http.Request) {
	coinID := r.URL.Query().Get("coin")
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

	prices, err := store.GetCryptoPricesByDateRange(coinID, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/crypto/prices] Failed to get crypto prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get crypto prices")
		return
	}

	data := make([]cryptoPricePoint, 0, len(prices))
	for _, p := range prices {
		data = append(data, cryptoPricePoint{
			Date:       p.TradingDate.Format("2006-01-02"),
			ClosePrice: p.ClosePrice,
			MarketCap:  p.MarketCap,
			Volume24h:  p.Volume24h,
		})
	}

	ResponseSuccess(w, http.StatusOK, cryptoPricesResponse{
		CoinID: coinID,
		Data:   data,
	})
}

type cryptoChartResponse struct {
	CoinID      string            `json:"coin_id"`
	Dates       []string          `json:"dates"`
	Prices      []decimal.Decimal `json:"prices"`
	Opens       []decimal.Decimal `json:"opens"`
	Highs       []decimal.Decimal `json:"highs"`
	Lows        []decimal.Decimal `json:"lows"`
	Closes      []decimal.Decimal `json:"closes"`
	Granularity string            `json:"granularity"`
}

// GetCryptoChart godoc
//
//	@Summary      Get crypto price chart data
//	@Description  Returns chronologically ordered date labels and closing prices for charting. When days=1, returns hourly intraday data; otherwise returns daily data.
//	@Tags         Crypto
//	@Produce      json
//	@Param        coin  query  string  false  "Coin ID (e.g. bitcoin, ethereum)"
//	@Param        days  query  int     false  "Number of days to look back (default 30); use 1 for intraday hourly data"
//	@Success      200   {object}  cryptoChartResponse
//	@Failure      500   {object}  ResponseFailure
//	@Router       /api/crypto/chart [get]
func GetCryptoChart(w http.ResponseWriter, r *http.Request) {
	coinID := r.URL.Query().Get("coin")
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
		intradayPrices, err := store.GetCryptoIntradayByRange(coinID, from, to)
		if err != nil {
			logger.Logger.Errorf("[api/crypto/chart] Failed to get crypto intraday prices: %v", err)
			ResponseError(w, http.StatusInternalServerError, "Failed to get crypto intraday prices")
			return
		}

		n := len(intradayPrices)
		dates := make([]string, 0, n)
		closePrices := make([]decimal.Decimal, 0, n)
		opens := make([]decimal.Decimal, 0, n)
		highs := make([]decimal.Decimal, 0, n)
		lows := make([]decimal.Decimal, 0, n)
		closes := make([]decimal.Decimal, 0, n)
		allNilOHLC := true

		// intraday prices are ordered DESC from DB — reverse for chart (oldest first)
		for i := n - 1; i >= 0; i-- {
			p := intradayPrices[i]
			dates = append(dates, p.Timestamp.Format("2006-01-02 15:04"))
			closePrices = append(closePrices, p.Price)
			closes = append(closes, p.Price)
			if p.OpenPrice != nil && p.HighPrice != nil && p.LowPrice != nil {
				allNilOHLC = false
				opens = append(opens, *p.OpenPrice)
				highs = append(highs, *p.HighPrice)
				lows = append(lows, *p.LowPrice)
			} else {
				// doji fallback: fill with close to keep alignment
				opens = append(opens, p.Price)
				highs = append(highs, p.Price)
				lows = append(lows, p.Price)
			}
		}

		// if every point lacked OHLC, return empty arrays so frontend hides candle toggle
		if allNilOHLC {
			opens = []decimal.Decimal{}
			highs = []decimal.Decimal{}
			lows = []decimal.Decimal{}
			closes = []decimal.Decimal{}
		}

		ResponseSuccess(w, http.StatusOK, cryptoChartResponse{
			CoinID:      coinID,
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

	prices, err := store.GetCryptoPricesByDateRange(coinID, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/crypto/chart] Failed to get crypto prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get crypto prices")
		return
	}

	n := len(prices)
	dates := make([]string, 0, n)
	closePrices := make([]decimal.Decimal, 0, n)
	opens := make([]decimal.Decimal, 0, n)
	highs := make([]decimal.Decimal, 0, n)
	lows := make([]decimal.Decimal, 0, n)
	closes := make([]decimal.Decimal, 0, n)
	allNilOHLC := true

	// prices are ordered DESC from DB — reverse for chart (oldest first)
	for i := n - 1; i >= 0; i-- {
		p := prices[i]
		dates = append(dates, p.TradingDate.Format("2006-01-02"))
		closePrices = append(closePrices, p.ClosePrice)
		closes = append(closes, p.ClosePrice)
		if p.OpenPrice != nil && p.HighPrice != nil && p.LowPrice != nil {
			allNilOHLC = false
			opens = append(opens, *p.OpenPrice)
			highs = append(highs, *p.HighPrice)
			lows = append(lows, *p.LowPrice)
		} else {
			// doji fallback: fill with close to keep alignment
			opens = append(opens, p.ClosePrice)
			highs = append(highs, p.ClosePrice)
			lows = append(lows, p.ClosePrice)
		}
	}

	// if every point lacked OHLC, return empty arrays so frontend hides candle toggle
	if allNilOHLC {
		opens = []decimal.Decimal{}
		highs = []decimal.Decimal{}
		lows = []decimal.Decimal{}
		closes = []decimal.Decimal{}
	}

	ResponseSuccess(w, http.StatusOK, cryptoChartResponse{
		CoinID:      coinID,
		Dates:       dates,
		Prices:      closePrices,
		Opens:       opens,
		Highs:       highs,
		Lows:        lows,
		Closes:      closes,
		Granularity: "1d",
	})
}
