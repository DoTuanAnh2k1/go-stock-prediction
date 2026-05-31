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
	coinGeckoSimplePriceURL  = "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd&include_market_cap=true&include_24hr_vol=true"
	coinGeckoMarketChartURL  = "https://api.coingecko.com/api/v3/coins/%s/market_chart?vs_currency=usd&days=180&interval=daily"
	cryptoTimeout            = 30 * time.Second
)

// cryptoCoins defines the cryptocurrencies we track.
var cryptoCoins = []struct {
	CoinID string
	Symbol string
}{
	{"bitcoin", "BTC"},
	{"ethereum", "ETH"},
}

// coinGeckoSimplePriceResponse maps the CoinGecko simple/price API response.
// Format: {"bitcoin": {"usd": 45000, "usd_market_cap": 890000000000, "usd_24h_vol": 25000000000}, ...}
type coinGeckoSimplePriceResponse map[string]struct {
	USD          float64 `json:"usd"`
	USDMarketCap float64 `json:"usd_market_cap"`
	USD24hVol    float64 `json:"usd_24h_vol"`
}

// coinGeckoMarketChartResponse maps the CoinGecko market_chart API response.
// Each entry in Prices/MarketCaps/TotalVolumes is [timestamp_ms, value].
type coinGeckoMarketChartResponse struct {
	Prices       [][]float64 `json:"prices"`
	MarketCaps   [][]float64 `json:"market_caps"`
	TotalVolumes [][]float64 `json:"total_volumes"`
}

// CronjobCryptoCrawler fetches the current prices for BTC and ETH from CoinGecko
// in a single API call and upserts today's record for each coin.
func CronjobCryptoCrawler() error {
	logger.Logger.Info("[crypto-crawler] Starting crypto price crawl")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		return fmt.Errorf("[crypto-crawler] database store not available")
	}

	client := &http.Client{Timeout: cryptoTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coinGeckoSimplePriceURL, nil)
	if err != nil {
		return fmt.Errorf("[crypto-crawler] failed to build request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("[crypto-crawler] CoinGecko request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[crypto-crawler] CoinGecko HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("[crypto-crawler] failed to read response: %v", err)
	}

	var priceResp coinGeckoSimplePriceResponse
	if jsonErr := json.Unmarshal(body, &priceResp); jsonErr != nil {
		return fmt.Errorf("[crypto-crawler] JSON parse error: %v", jsonErr)
	}

	tradingDate := time.Now().Truncate(24 * time.Hour)
	successCount := 0
	errorCount := 0

	for _, coin := range cryptoCoins {
		data, ok := priceResp[coin.CoinID]
		if !ok || data.USD == 0 {
			logger.Logger.Errorf("[crypto-crawler] No price data for %s", coin.CoinID)
			errorCount++
			continue
		}

		record := &modelsdb.CryptoPrice{
			CoinID:      coin.CoinID,
			Symbol:      coin.Symbol,
			ClosePrice:  decimal.NewFromFloat(data.USD),
			MarketCap:   decimal.NewFromFloat(data.USDMarketCap),
			Volume24h:   decimal.NewFromFloat(data.USD24hVol),
			TradingDate: tradingDate,
			Currency:    "USD",
		}

		if upsertErr := store.UpsertCryptoPrice(record); upsertErr != nil {
			logger.Logger.Errorf("[crypto-crawler] Failed to save %s: %v", coin.CoinID, upsertErr)
			errorCount++
		} else {
			successCount++
			logger.Logger.Infof("[crypto-crawler] Saved %s (%s): $%s", coin.CoinID, coin.Symbol, record.ClosePrice.String())
		}
	}

	duration := time.Since(startTime)

	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		DurationMs:   duration.Milliseconds(),
		Source:       "CryptoCrawler",
	}
	if errorCount > 0 {
		syncLog.ErrorMessage = fmt.Sprintf("Encountered %d errors during crypto crawl", errorCount)
	}
	if logErr := store.CreateSyncLog(syncLog); logErr != nil {
		logger.Logger.Errorf("[crypto-crawler] Failed to save sync log: %v", logErr)
	}

	logger.Logger.Infof("[crypto-crawler] Completed in %v: %d saved, %d errors", duration, successCount, errorCount)
	return nil
}

// ImportCryptoHistory backfills 180 days of daily price history for each tracked coin.
// Uses CoinGecko's market_chart endpoint. Sleeps 3 seconds between coins to respect free tier limits.
func ImportCryptoHistory() {
	logger.Logger.Info("[crypto-history] Starting 180-day crypto history backfill")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		logger.Logger.Error("[crypto-history] database store not available")
		return
	}

	client := &http.Client{Timeout: cryptoTimeout}

	savedTotal := 0

	for i, coin := range cryptoCoins {
		if i > 0 {
			// Respect CoinGecko free tier rate limit.
			time.Sleep(3 * time.Second)
		}

		url := fmt.Sprintf(coinGeckoMarketChartURL, coin.CoinID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			logger.Logger.Errorf("[crypto-history] Failed to build request for %s: %v", coin.CoinID, err)
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			logger.Logger.Errorf("[crypto-history] Request failed for %s: %v", coin.CoinID, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Logger.Errorf("[crypto-history] CoinGecko HTTP %d for %s", resp.StatusCode, coin.CoinID)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Logger.Errorf("[crypto-history] Failed to read body for %s: %v", coin.CoinID, err)
			continue
		}

		var chartResp coinGeckoMarketChartResponse
		if jsonErr := json.Unmarshal(body, &chartResp); jsonErr != nil {
			logger.Logger.Errorf("[crypto-history] JSON parse error for %s: %v", coin.CoinID, jsonErr)
			continue
		}

		// Build lookup maps for market cap and volume by timestamp (ms).
		capByTS := make(map[int64]float64, len(chartResp.MarketCaps))
		for _, entry := range chartResp.MarketCaps {
			if len(entry) == 2 {
				capByTS[int64(entry[0])] = entry[1]
			}
		}
		volByTS := make(map[int64]float64, len(chartResp.TotalVolumes))
		for _, entry := range chartResp.TotalVolumes {
			if len(entry) == 2 {
				volByTS[int64(entry[0])] = entry[1]
			}
		}

		savedForCoin := 0
		for _, entry := range chartResp.Prices {
			if len(entry) != 2 || entry[1] == 0 {
				continue
			}
			tsMs := int64(entry[0])
			tradingDate := time.Unix(tsMs/1000, 0).UTC().Truncate(24 * time.Hour)

			record := &modelsdb.CryptoPrice{
				CoinID:      coin.CoinID,
				Symbol:      coin.Symbol,
				ClosePrice:  decimal.NewFromFloat(entry[1]),
				MarketCap:   decimal.NewFromFloat(capByTS[tsMs]),
				Volume24h:   decimal.NewFromFloat(volByTS[tsMs]),
				TradingDate: tradingDate,
				Currency:    "USD",
			}

			if upsertErr := store.UpsertCryptoPrice(record); upsertErr != nil {
				logger.Logger.Errorf("[crypto-history] Failed to upsert %s for %s: %v", coin.CoinID, tradingDate.Format("2006-01-02"), upsertErr)
			} else {
				savedForCoin++
				savedTotal++
			}
		}
		logger.Logger.Infof("[crypto-history] %s (%s): %d days saved", coin.CoinID, coin.Symbol, savedForCoin)
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("[crypto-history] Completed in %v: %d total days saved", duration, savedTotal)
}
