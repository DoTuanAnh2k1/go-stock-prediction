package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

const (
	yahooNasdaqDailyURLFmt   = "https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=2d"
	yahooNasdaqHistoryURLFmt = "https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=6mo"
	nasdaqTimeout            = 15 * time.Second
)

// nasdaqSymbols defines the NASDAQ 100 constituent stocks we track.
var nasdaqSymbols = []string{
	"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA",
	"META", "TSLA", "AVGO", "COST", "NFLX",
	"AMD", "ADBE", "QCOM", "INTC", "CSCO",
}

// yahooNasdaqHistoricalResponse maps the full OHLCV Yahoo Finance chart API response.
// Used for both daily and historical fetches.
type yahooNasdaqHistoricalResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// CronjobNasdaqCrawler crawls the latest daily close prices for all tracked NASDAQ symbols.
// Runs on weekday evenings after US market close (10:30 PM VN time = ~10:30 AM EST).
func CronjobNasdaqCrawler() error {
	logger.Logger.Info("[nasdaq-crawler] Starting NASDAQ daily crawl")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		return fmt.Errorf("[nasdaq-crawler] database store not available")
	}

	client := &http.Client{Timeout: nasdaqTimeout}

	successCount := 0
	errorCount := 0

	for _, symbol := range nasdaqSymbols {
		price, err := fetchNasdaqDaily(ctx, client, symbol)
		if err != nil {
			logger.Logger.Errorf("[nasdaq-crawler] Failed to fetch %s: %v", symbol, err)
			errorCount++
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if upsertErr := store.UpsertNasdaqPrice(price); upsertErr != nil {
			logger.Logger.Errorf("[nasdaq-crawler] Failed to save %s: %v", symbol, upsertErr)
			errorCount++
		} else {
			successCount++
			logger.Logger.Debugf("[nasdaq-crawler] Saved %s: %s USD", symbol, price.ClosePrice.String())
		}

		time.Sleep(500 * time.Millisecond)
	}

	duration := time.Since(startTime)

	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		DurationMs:   duration.Milliseconds(),
		Source:       "NasdaqCrawler",
	}
	if errorCount > 0 {
		syncLog.ErrorMessage = fmt.Sprintf("Encountered %d errors during NASDAQ crawl", errorCount)
	}
	if logErr := store.CreateSyncLog(syncLog); logErr != nil {
		logger.Logger.Errorf("[nasdaq-crawler] Failed to save sync log: %v", logErr)
	}

	logger.Logger.Infof("[nasdaq-crawler] Completed in %v: %d saved, %d errors", duration, successCount, errorCount)
	return nil
}

// ImportNasdaqHistory backfills 6 months of daily history for all NASDAQ symbols.
// Skips symbols that already have data in DB.
func ImportNasdaqHistory() {
	logger.Logger.Info("[nasdaq-history] Starting NASDAQ 6-month history backfill")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		logger.Logger.Error("[nasdaq-history] database store not available")
		return
	}

	client := &http.Client{Timeout: nasdaqTimeout}

	savedTotal := 0
	skippedSymbols := 0

	for _, symbol := range nasdaqSymbols {
		// Check if symbol already has recent data.
		latest, _ := store.GetLatestNasdaqPrice(symbol)
		if latest != nil && time.Since(latest.TradingDate) < 7*24*time.Hour {
			logger.Logger.Debugf("[nasdaq-history] %s already has recent data, skipping", symbol)
			skippedSymbols++
			continue
		}

		prices, err := fetchNasdaqHistory(ctx, client, symbol)
		if err != nil {
			logger.Logger.Errorf("[nasdaq-history] Failed to fetch history for %s: %v", symbol, err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		savedForSymbol := 0
		for i := range prices {
			if upsertErr := store.UpsertNasdaqPrice(&prices[i]); upsertErr != nil {
				logger.Logger.Errorf("[nasdaq-history] Failed to upsert %s/%s: %v", symbol, prices[i].TradingDate.Format("2006-01-02"), upsertErr)
			} else {
				savedForSymbol++
				savedTotal++
			}
		}
		logger.Logger.Infof("[nasdaq-history] %s: %d days saved", symbol, savedForSymbol)

		time.Sleep(500 * time.Millisecond)
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("[nasdaq-history] Completed in %v: %d total days saved, %d symbols skipped", duration, savedTotal, skippedSymbols)
}

// fetchNasdaqDaily fetches the most recent trading day's OHLCV data for a symbol.
func fetchNasdaqDaily(ctx context.Context, client *http.Client, symbol string) (*modelsdb.NasdaqPrice, error) {
	url := fmt.Sprintf(yahooNasdaqDailyURLFmt, symbol)
	result, err := doYahooNasdaqRequest(ctx, client, url)
	if err != nil {
		return nil, err
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("no result for %s", symbol)
	}

	r := result.Chart.Result[0]
	if len(r.Timestamp) == 0 || len(r.Indicators.Quote) == 0 {
		// Fall back to regularMarketPrice if no OHLCV data in range
		if r.Meta.RegularMarketPrice == 0 {
			return nil, fmt.Errorf("no price data for %s", symbol)
		}
		price := decimal.NewFromFloat(r.Meta.RegularMarketPrice)
		return &modelsdb.NasdaqPrice{
			Symbol:      symbol,
			ClosePrice:  price,
			OpenPrice:   price,
			HighPrice:   price,
			LowPrice:    price,
			TradingDate: time.Now().Truncate(24 * time.Hour),
			Currency:    "USD",
		}, nil
	}

	q := r.Indicators.Quote[0]
	// Use the last available data point (most recent trading day).
	idx := len(r.Timestamp) - 1
	closePrice := q.Close[idx]
	if closePrice == 0 {
		// Walk backwards to find last non-zero close
		for idx > 0 && closePrice == 0 {
			idx--
			closePrice = q.Close[idx]
		}
		if closePrice == 0 {
			return nil, fmt.Errorf("all close prices are zero for %s", symbol)
		}
	}

	tradingDate := time.Unix(r.Timestamp[idx], 0).UTC().Truncate(24 * time.Hour)

	np := &modelsdb.NasdaqPrice{
		Symbol:      symbol,
		ClosePrice:  decimal.NewFromFloat(closePrice),
		TradingDate: tradingDate,
		Currency:    "USD",
	}
	if len(q.Open) > idx && q.Open[idx] != 0 {
		np.OpenPrice = decimal.NewFromFloat(q.Open[idx])
	}
	if len(q.High) > idx && q.High[idx] != 0 {
		np.HighPrice = decimal.NewFromFloat(q.High[idx])
	}
	if len(q.Low) > idx && q.Low[idx] != 0 {
		np.LowPrice = decimal.NewFromFloat(q.Low[idx])
	}
	if len(q.Volume) > idx && q.Volume[idx] != 0 {
		np.Volume = int64(q.Volume[idx])
	}

	return np, nil
}

// fetchNasdaqHistory fetches 6 months of daily OHLCV data for a symbol.
func fetchNasdaqHistory(ctx context.Context, client *http.Client, symbol string) ([]modelsdb.NasdaqPrice, error) {
	url := fmt.Sprintf(yahooNasdaqHistoryURLFmt, symbol)
	result, err := doYahooNasdaqRequest(ctx, client, url)
	if err != nil {
		return nil, err
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("no result for %s", symbol)
	}

	r := result.Chart.Result[0]
	if len(r.Timestamp) == 0 || len(r.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no OHLCV data for %s", symbol)
	}

	q := r.Indicators.Quote[0]
	var prices []modelsdb.NasdaqPrice

	for i, ts := range r.Timestamp {
		if i >= len(q.Close) || q.Close[i] == 0 {
			continue
		}

		tradingDate := time.Unix(ts, 0).UTC().Truncate(24 * time.Hour)
		np := modelsdb.NasdaqPrice{
			Symbol:      symbol,
			ClosePrice:  decimal.NewFromFloat(q.Close[i]),
			TradingDate: tradingDate,
			Currency:    "USD",
		}
		if i < len(q.Open) && q.Open[i] != 0 {
			np.OpenPrice = decimal.NewFromFloat(q.Open[i])
		}
		if i < len(q.High) && q.High[i] != 0 {
			np.HighPrice = decimal.NewFromFloat(q.High[i])
		}
		if i < len(q.Low) && q.Low[i] != 0 {
			np.LowPrice = decimal.NewFromFloat(q.Low[i])
		}
		if i < len(q.Volume) && q.Volume[i] != 0 {
			np.Volume = int64(q.Volume[i])
		}
		prices = append(prices, np)
	}

	return prices, nil
}

// doYahooNasdaqRequest executes a GET request to Yahoo Finance and parses the chart response.
func doYahooNasdaqRequest(ctx context.Context, client *http.Client, url string) (*yahooNasdaqHistoricalResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yahoo Finance HTTP %d for URL %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result yahooNasdaqHistoricalResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Yahoo Finance JSON parse error: %v", err)
	}

	if result.Chart.Error != nil {
		return nil, fmt.Errorf("Yahoo Finance API error: %s — %s", result.Chart.Error.Code, result.Chart.Error.Description)
	}

	return &result, nil
}
