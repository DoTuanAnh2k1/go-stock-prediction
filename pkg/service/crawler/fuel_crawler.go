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
	fuelPvdateURLFmt = "https://giaxanghomnay.com/api/pvdate/%s"
	fuelChartURL     = "https://giaxanghomnay.com/api/chart"
	fuelTimeout      = 15 * time.Second
	// Minimum records threshold below which ImportFuelHistory is triggered.
	fuelMinRecords = 10
)

// fuelProducts maps the API field letters to our product_type identifiers.
// Keys correspond to JSON fields in FuelChartEntry ("a", "b", "c", "d").
var fuelProducts = map[string]string{
	"a": "ron95_iii",
	"b": "e5_ron92",
	"c": "do_005s",
	"d": "kerosene",
}

// FuelChartEntry maps a single row from either /api/pvdate or /api/chart.
type FuelChartEntry struct {
	Date string  `json:"date"`
	A    float64 `json:"a"`
	B    float64 `json:"b"`
	C    float64 `json:"c"`
	D    float64 `json:"d"`
}

// fuelFieldValues extracts product prices from a FuelChartEntry into a map[letter]price.
func fuelFieldValues(entry FuelChartEntry) map[string]float64 {
	return map[string]float64{
		"a": entry.A,
		"b": entry.B,
		"c": entry.C,
		"d": entry.D,
	}
}

// CronjobFuelCrawler fetches today's retail fuel prices from giaxanghomnay.com
// and upserts 4 product records (RON95-III, E5 RON92, DO 0.05S, kerosene).
// If the API returns an empty array (no change today), the function returns nil silently.
func CronjobFuelCrawler() error {
	logger.Logger.Info("[fuel-crawler] Starting fuel price crawl")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		return fmt.Errorf("[fuel-crawler] database store not available")
	}

	client := &http.Client{Timeout: fuelTimeout}

	today := time.Now().Format("2006-01-02")
	url := fmt.Sprintf(fuelPvdateURLFmt, today)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("[fuel-crawler] failed to build request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("[fuel-crawler] request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("[fuel-crawler] HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("[fuel-crawler] failed to read response: %v", err)
	}

	var entries []FuelChartEntry
	if jsonErr := json.Unmarshal(body, &entries); jsonErr != nil {
		return fmt.Errorf("[fuel-crawler] JSON parse error: %v", jsonErr)
	}

	// Empty array = no price change today — skip quietly.
	if len(entries) == 0 {
		logger.Logger.Info("[fuel-crawler] No price change today, skipping")
		return nil
	}

	prices, err := parseFuelEntries(entries)
	if err != nil {
		return fmt.Errorf("[fuel-crawler] failed to parse entries: %v", err)
	}

	successCount := 0
	errorCount := 0

	for i := range prices {
		if upsertErr := store.UpsertFuelPrice(&prices[i]); upsertErr != nil {
			logger.Logger.Errorf("[fuel-crawler] Failed to save %s: %v", prices[i].ProductType, upsertErr)
			errorCount++
		} else {
			successCount++
		}
	}

	duration := time.Since(startTime)

	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		DurationMs:   duration.Milliseconds(),
		Source:       "FuelCrawler",
	}
	if errorCount > 0 {
		syncLog.ErrorMessage = fmt.Sprintf("Encountered %d errors during fuel crawl", errorCount)
	}
	if logErr := store.CreateSyncLog(syncLog); logErr != nil {
		logger.Logger.Errorf("[fuel-crawler] Failed to save sync log: %v", logErr)
	}

	logger.Logger.Infof("[fuel-crawler] Completed in %v: %d saved, %d errors", duration, successCount, errorCount)
	return nil
}

// ImportFuelHistory fetches all historical fuel prices from giaxanghomnay.com/api/chart
// and bulk-upserts them. Only runs if the DB has fewer than fuelMinRecords records.
func ImportFuelHistory() {
	logger.Logger.Info("[fuel-history] Checking if fuel history backfill is needed")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		logger.Logger.Error("[fuel-history] database store not available")
		return
	}

	// Check existing record count by fetching products — if 4 products exist
	// with recent data we consider history already populated.
	products, err := store.GetFuelProducts()
	if err == nil && len(products) >= 4 {
		// Check if recent data (within last 30 days) exists for any product.
		from := time.Now().AddDate(0, -1, 0)
		for _, pt := range products {
			prices, checkErr := store.GetFuelPricesByDateRange(pt, from, time.Now())
			if checkErr == nil && len(prices) >= fuelMinRecords {
				logger.Logger.Info("[fuel-history] Fuel history already populated, skipping backfill")
				return
			}
		}
	}

	logger.Logger.Info("[fuel-history] Starting full fuel history backfill from giaxanghomnay.com")
	startTime := time.Now()

	client := &http.Client{Timeout: fuelTimeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fuelChartURL, nil)
	if err != nil {
		logger.Logger.Errorf("[fuel-history] Failed to build request: %v", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Logger.Errorf("[fuel-history] Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Logger.Errorf("[fuel-history] HTTP %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Errorf("[fuel-history] Failed to read response: %v", err)
		return
	}

	var entries []FuelChartEntry
	if jsonErr := json.Unmarshal(body, &entries); jsonErr != nil {
		logger.Logger.Errorf("[fuel-history] JSON parse error: %v", jsonErr)
		return
	}

	if len(entries) == 0 {
		logger.Logger.Warn("[fuel-history] API returned empty history")
		return
	}

	prices, err := parseFuelEntries(entries)
	if err != nil {
		logger.Logger.Errorf("[fuel-history] Failed to parse entries: %v", err)
		return
	}

	if bulkErr := store.BulkUpsertFuelPrices(prices); bulkErr != nil {
		logger.Logger.Errorf("[fuel-history] Bulk upsert failed: %v", bulkErr)
		return
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("[fuel-history] Completed in %v: %d records upserted from %d entries", duration, len(prices), len(entries))
}

// parseFuelEntries converts a slice of FuelChartEntry into FuelPrice records.
// Skips entries with zero prices or unparseable dates.
func parseFuelEntries(entries []FuelChartEntry) ([]modelsdb.FuelPrice, error) {
	var prices []modelsdb.FuelPrice

	for _, entry := range entries {
		tradingDate, parseErr := time.ParseInLocation("2006-01-02", entry.Date, time.Local)
		if parseErr != nil {
			logger.Logger.Warnf("[fuel] Cannot parse date %q: %v", entry.Date, parseErr)
			continue
		}
		tradingDate = tradingDate.Truncate(24 * time.Hour)

		fields := fuelFieldValues(entry)
		for letter, productType := range fuelProducts {
			val, ok := fields[letter]
			if !ok || val == 0 {
				continue
			}
			prices = append(prices, modelsdb.FuelPrice{
				ProductType: productType,
				Price:       decimal.NewFromFloat(val),
				TradingDate: tradingDate,
			})
		}
	}

	return prices, nil
}
