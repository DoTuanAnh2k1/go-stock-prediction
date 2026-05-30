package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

const (
	yahooFinanceGoldURL        = "https://query1.finance.yahoo.com/v8/finance/chart/GC%3DF?interval=1d&range=1d"
	yahooFinanceGoldHistoryURL = "https://query1.finance.yahoo.com/v8/finance/chart/GC%3DF?interval=1d&range=6mo"
	usdVndRateURL              = "https://open.er-api.com/v6/latest/USD"
	goldTimeout                = 15 * time.Second
)

// yahooChartResponse maps the Yahoo Finance chart API response for a single-day (range=1d) query.
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
}

// yahooHistoricalResponse maps the Yahoo Finance chart API response for multi-day historical queries.
// chart.result[0].timestamp holds Unix timestamps and
// chart.result[0].indicators.quote[0].close holds the daily closing prices.
type yahooHistoricalResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// erAPIResponse maps the open.er-api.com exchange rate response.
type erAPIResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

// CronjobGoldCrawler crawls gold prices: XAU/USD from Yahoo Finance and XAU/VND calculated via USD/VND rate.
func CronjobGoldCrawler() error {
	logger.Logger.Info("[gold-crawler] Starting gold price crawl")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		return fmt.Errorf("[gold-crawler] database store not available")
	}

	client := &http.Client{Timeout: goldTimeout}

	successCount := 0
	errorCount := 0

	// --- XAU/USD from Yahoo Finance ---
	xauUSD, err := crawlXAUUSD(ctx, client)
	if err != nil {
		logger.Logger.Errorf("[gold-crawler] XAU/USD crawl failed: %v", err)
		errorCount++
	} else {
		if upsertErr := store.UpsertGoldPrice(xauUSD); upsertErr != nil {
			logger.Logger.Errorf("[gold-crawler] Failed to save XAU/USD price: %v", upsertErr)
			errorCount++
		} else {
			successCount++
			logger.Logger.Infof("[gold-crawler] Saved XAU/USD: %s USD/oz", xauUSD.BuyPrice.String())
		}

		// --- XAU/VND: calculate from XAU/USD × USD/VND rate ---
		xauVND, err := calcXAUVND(ctx, client, xauUSD.BuyPrice)
		if err != nil {
			logger.Logger.Errorf("[gold-crawler] XAU/VND calculation failed: %v", err)
			errorCount++
		} else {
			if upsertErr := store.UpsertGoldPrice(xauVND); upsertErr != nil {
				logger.Logger.Errorf("[gold-crawler] Failed to save XAU/VND price: %v", upsertErr)
				errorCount++
			} else {
				successCount++
				logger.Logger.Infof("[gold-crawler] Saved XAU/VND: %s VND/oz", xauVND.BuyPrice.String())
			}
		}
	}

	// --- BTMC ---
	btmcPrices, err := crawlBTMC(ctx, client)
	if err != nil {
		logger.Logger.Errorf("[gold-crawler] BTMC crawl failed: %v", err)
		errorCount++
	} else {
		if upsertErr := store.BulkUpsertGoldPrices(btmcPrices); upsertErr != nil {
			logger.Logger.Errorf("[gold-crawler] Failed to save BTMC prices: %v", upsertErr)
			errorCount++
		} else {
			successCount += len(btmcPrices)
			logger.Logger.Infof("[gold-crawler] Saved %d BTMC gold prices", len(btmcPrices))
		}
	}

	// --- BTMH ---
	btmhPrices, err := crawlBTMH(ctx, client)
	if err != nil {
		logger.Logger.Errorf("[gold-crawler] BTMH crawl failed: %v", err)
		errorCount++
	} else {
		if upsertErr := store.BulkUpsertGoldPrices(btmhPrices); upsertErr != nil {
			logger.Logger.Errorf("[gold-crawler] Failed to save BTMH prices: %v", upsertErr)
			errorCount++
		} else {
			successCount += len(btmhPrices)
			logger.Logger.Infof("[gold-crawler] Saved %d BTMH gold prices", len(btmhPrices))
		}
	}

	// --- vang.today ---
	vangTodayPrices, err := crawlVangToday(ctx, client)
	if err != nil {
		logger.Logger.Errorf("[gold-crawler] vang.today crawl failed: %v", err)
		errorCount++
	} else {
		if upsertErr := store.BulkUpsertGoldPrices(vangTodayPrices); upsertErr != nil {
			logger.Logger.Errorf("[gold-crawler] Failed to save vang.today prices: %v", upsertErr)
			errorCount++
		} else {
			successCount += len(vangTodayPrices)
			logger.Logger.Infof("[gold-crawler] Saved %d vang.today gold prices", len(vangTodayPrices))
		}
	}

	// --- Phú Quý ---
	phuQuyPrices, err := crawlPhuQuy(ctx, client)
	if err != nil {
		logger.Logger.Errorf("[gold-crawler] PhuQuy crawl failed: %v", err)
		errorCount++
	} else {
		if upsertErr := store.BulkUpsertGoldPrices(phuQuyPrices); upsertErr != nil {
			logger.Logger.Errorf("[gold-crawler] Failed to save PhuQuy prices: %v", upsertErr)
			errorCount++
		} else {
			successCount += len(phuQuyPrices)
			logger.Logger.Infof("[gold-crawler] Saved %d PhuQuy gold prices", len(phuQuyPrices))
		}
	}

	duration := time.Since(startTime)

	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		DurationMs:   duration.Milliseconds(),
		Source:       "GoldCrawler",
	}
	if errorCount > 0 {
		syncLog.ErrorMessage = fmt.Sprintf("Encountered %d errors during gold crawl", errorCount)
	}
	if logErr := store.CreateSyncLog(syncLog); logErr != nil {
		logger.Logger.Errorf("[gold-crawler] Failed to save sync log: %v", logErr)
	}

	logger.Logger.Infof("[gold-crawler] Completed in %v: %d saved, %d errors", duration, successCount, errorCount)
	return nil
}

// crawlXAUUSD fetches the XAU/USD spot price from Yahoo Finance (Gold Futures GC=F).
// Response: {"chart": {"result": [{"meta": {"regularMarketPrice": 3250.50}}]}}
func crawlXAUUSD(ctx context.Context, client *http.Client) (*modelsdb.GoldPrice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, yahooFinanceGoldURL, nil)
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
		return nil, fmt.Errorf("Yahoo Finance HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result yahooChartResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("Yahoo Finance JSON parse error: %v", err)
	}

	if len(result.Chart.Result) == 0 || result.Chart.Result[0].Meta.RegularMarketPrice == 0 {
		return nil, fmt.Errorf("Yahoo Finance returned empty or zero price")
	}

	price := decimal.NewFromFloat(result.Chart.Result[0].Meta.RegularMarketPrice)
	tradingDate := time.Now().Truncate(24 * time.Hour)

	return &modelsdb.GoldPrice{
		Source:      "XAU",
		ProductType: "spot",
		TradingDate: tradingDate,
		BuyPrice:    price,
		SellPrice:   price,
		Currency:    "USD",
	}, nil
}

// calcXAUVND converts the XAU/USD price to VND using the USD/VND rate from open.er-api.com.
// The result is price per troy ounce in VND.
func calcXAUVND(ctx context.Context, client *http.Client, xauUSD decimal.Decimal) (*modelsdb.GoldPrice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usdVndRateURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open.er-api HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rateResp erAPIResponse
	if err := json.Unmarshal(body, &rateResp); err != nil {
		return nil, fmt.Errorf("exchange rate JSON parse error: %v", err)
	}

	vndRate, ok := rateResp.Rates["VND"]
	if !ok || vndRate == 0 {
		return nil, fmt.Errorf("VND rate not found in response")
	}

	usdVnd := decimal.NewFromFloat(vndRate)
	xauVND := xauUSD.Mul(usdVnd).Round(0)
	tradingDate := time.Now().Truncate(24 * time.Hour)

	return &modelsdb.GoldPrice{
		Source:      "XAU_VND",
		ProductType: "spot",
		TradingDate: tradingDate,
		BuyPrice:    xauVND,
		SellPrice:   xauVND,
		Currency:    "VND",
	}, nil
}

// fetchUSDVNDRate returns the current USD/VND exchange rate from open.er-api.com.
func fetchUSDVNDRate(ctx context.Context, client *http.Client) (decimal.Decimal, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usdVndRateURL, nil)
	if err != nil {
		return decimal.Zero, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return decimal.Zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return decimal.Zero, fmt.Errorf("open.er-api HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return decimal.Zero, err
	}

	var rateResp erAPIResponse
	if err := json.Unmarshal(body, &rateResp); err != nil {
		return decimal.Zero, fmt.Errorf("exchange rate JSON parse error: %v", err)
	}

	vndRate, ok := rateResp.Rates["VND"]
	if !ok || vndRate == 0 {
		return decimal.Zero, fmt.Errorf("VND rate not found in response")
	}

	return decimal.NewFromFloat(vndRate), nil
}

// ImportXAUHistory fetches 6 months of daily XAU/USD closing prices from Yahoo
// Finance and upserts each day into the DB. XAU/VND values are calculated using
// the CURRENT USD/VND rate (fetched once). Days that already exist are silently
// overwritten by the upsert. This is designed to backfill history on startup.
func ImportXAUHistory() {
	logger.Logger.Info("[gold-history] Starting XAU historical import (6 months)")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		logger.Logger.Error("[gold-history] database store not available")
		return
	}

	client := &http.Client{Timeout: goldTimeout}

	// Fetch the current USD/VND rate once; used for all VND conversions.
	usdVnd, err := fetchUSDVNDRate(ctx, client)
	if err != nil {
		logger.Logger.Errorf("[gold-history] Failed to fetch USD/VND rate: %v", err)
		return
	}
	logger.Logger.Infof("[gold-history] USD/VND rate: %s", usdVnd.String())

	// Fetch 6-month historical XAU/USD data from Yahoo Finance.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, yahooFinanceGoldHistoryURL, nil)
	if err != nil {
		logger.Logger.Errorf("[gold-history] Failed to build request: %v", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		logger.Logger.Errorf("[gold-history] Yahoo Finance request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Logger.Errorf("[gold-history] Yahoo Finance HTTP %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Logger.Errorf("[gold-history] Failed to read response body: %v", err)
		return
	}

	var historical yahooHistoricalResponse
	if err := json.Unmarshal(body, &historical); err != nil {
		logger.Logger.Errorf("[gold-history] JSON parse error: %v", err)
		return
	}

	if len(historical.Chart.Result) == 0 {
		logger.Logger.Error("[gold-history] Yahoo Finance returned empty result")
		return
	}

	result := historical.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		logger.Logger.Error("[gold-history] Yahoo Finance returned no quote data")
		return
	}

	timestamps := result.Timestamp
	closes := result.Indicators.Quote[0].Close

	if len(timestamps) != len(closes) {
		logger.Logger.Errorf("[gold-history] Timestamp/close length mismatch: %d vs %d", len(timestamps), len(closes))
		return
	}

	savedUSD := 0
	savedVND := 0
	skipped := 0

	for i, ts := range timestamps {
		closePrice := closes[i]
		// Yahoo Finance sometimes emits null (0) for non-trading days.
		if closePrice == 0 {
			skipped++
			continue
		}

		tradingDate := time.Unix(ts, 0).UTC().Truncate(24 * time.Hour)
		xauUSDPrice := decimal.NewFromFloat(closePrice)

		// Upsert XAU/USD record.
		xauUSD := &modelsdb.GoldPrice{
			Source:      "XAU",
			ProductType: "spot",
			TradingDate: tradingDate,
			BuyPrice:    xauUSDPrice,
			SellPrice:   xauUSDPrice,
			Currency:    "USD",
		}
		if upsertErr := store.UpsertGoldPrice(xauUSD); upsertErr != nil {
			logger.Logger.Errorf("[gold-history] Failed to upsert XAU/USD for %s: %v", tradingDate.Format("2006-01-02"), upsertErr)
		} else {
			savedUSD++
		}

		// Upsert XAU/VND record using the current exchange rate.
		xauVNDPrice := xauUSDPrice.Mul(usdVnd).Round(0)
		xauVND := &modelsdb.GoldPrice{
			Source:      "XAU_VND",
			ProductType: "spot",
			TradingDate: tradingDate,
			BuyPrice:    xauVNDPrice,
			SellPrice:   xauVNDPrice,
			Currency:    "VND",
		}
		if upsertErr := store.UpsertGoldPrice(xauVND); upsertErr != nil {
			logger.Logger.Errorf("[gold-history] Failed to upsert XAU/VND for %s: %v", tradingDate.Format("2006-01-02"), upsertErr)
		} else {
			savedVND++
		}
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("[gold-history] Completed in %v: %d XAU/USD saved, %d XAU/VND saved, %d skipped (null/zero)", duration, savedUSD, savedVND, skipped)
}

// --- BTMC types and helpers ---

const btmcAPIURL = "http://api.btmc.vn/api/BTMCAPI/getpricebtmc?key=3kd8ub1llcg9t45hnoh8hmn7t5kc2v"

// btmcJSONResponse maps the top-level JSON response from the BTMC API.
// Each element in Data is a map of dynamic keys like "@n_7", "@pb_7", "@ps_7", "@d_7".
type btmcJSONResponse struct {
	DataList struct {
		Data []map[string]string `json:"Data"`
	} `json:"DataList"`
}

// --- BTMH helpers ---

const btmhURL = "https://giavang.org/trong-nuoc/bao-tin-manh-hai/"

// btmhProductMap maps substrings in the product-name column to our ProductType
// identifiers. Checked in order; first match wins.
var btmhProductMap = []struct {
	keyword     string
	productType string
}{
	{"sjc", "sjc"},
	{"trang sức 24k", "trang_suc_24k"},
	{"nhẫn", "nhan_tron"},
}

// crawlBTMC fetches gold prices from the BTMC (Bảo Tín Minh Châu) public JSON
// API. Prices in the API are per chỉ; they are multiplied by 10 before saving
// so that all domestic VND prices are stored per lượng.
// The API returns each product multiple times (different timestamps); only the
// first occurrence of each product type is kept (newest entry).
func crawlBTMC(ctx context.Context, client *http.Client) ([]modelsdb.GoldPrice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, btmcAPIURL, nil)
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
		return nil, fmt.Errorf("BTMC API HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result btmcJSONResponse
	if jsonErr := json.Unmarshal(body, &result); jsonErr != nil {
		return nil, fmt.Errorf("BTMC JSON parse error: %v", jsonErr)
	}

	ten := decimal.NewFromInt(10)
	defaultDate := time.Now().Truncate(24 * time.Hour)

	// seen tracks which product types have already been recorded (first = newest).
	seen := make(map[string]bool)
	var prices []modelsdb.GoldPrice

	for _, item := range result.DataList.Data {
		row := strings.TrimSpace(item["@row"])
		if row == "" {
			continue
		}

		name := strings.TrimSpace(item["@n_"+row])
		nameLower := strings.ToLower(name)

		var productType string
		switch {
		case strings.Contains(nameLower, "sjc"):
			productType = "sjc"
		case strings.Contains(name, "NHẪN TRÒN"):
			productType = "nhan_tron"
		case strings.Contains(name, "VRTL"):
			productType = "vrtl"
		case strings.Contains(name, "TRANG SỨC"):
			productType = "trang_suc"
		default:
			continue
		}

		if seen[productType] {
			continue
		}

		buyRaw := strings.TrimSpace(item["@pb_"+row])
		sellRaw := strings.TrimSpace(item["@ps_"+row])
		dateRaw := strings.TrimSpace(item["@d_"+row])

		buyPerChi, parseErr := decimal.NewFromString(buyRaw)
		if parseErr != nil || buyPerChi.IsZero() {
			logger.Logger.Warnf("[gold-crawler] BTMC skipping %q (row %s): invalid buy price %q", name, row, buyRaw)
			continue
		}
		sellPerChi, parseErr := decimal.NewFromString(sellRaw)
		if parseErr != nil || sellPerChi.IsZero() {
			logger.Logger.Warnf("[gold-crawler] BTMC skipping %q (row %s): invalid sell price %q", name, row, sellRaw)
			continue
		}

		// Convert from per chỉ → per lượng (1 lượng = 10 chỉ).
		buyPerLuong := buyPerChi.Mul(ten)
		sellPerLuong := sellPerChi.Mul(ten)

		tradingDate := defaultDate
		if dateRaw != "" {
			if t, tErr := time.ParseInLocation("02/01/2006 15:04", dateRaw, time.Local); tErr == nil {
				tradingDate = t.Truncate(24 * time.Hour)
			}
		}

		seen[productType] = true
		prices = append(prices, modelsdb.GoldPrice{
			Source:      "BTMC",
			ProductType: productType,
			TradingDate: tradingDate,
			BuyPrice:    buyPerLuong,
			SellPrice:   sellPerLuong,
			Currency:    "VND",
		})
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("BTMC returned no recognised products")
	}
	return prices, nil
}

// crawlBTMH scrapes gold prices for Bảo Tín Mạnh Hải from giavang.org using
// Colly. The page contains an HTML table with columns:
//
//	Loại vàng | Mua vào | Bán ra
//
// Prices are in VND per lượng and stored as-is.
func crawlBTMH(_ context.Context, _ *http.Client) ([]modelsdb.GoldPrice, error) {
	tradingDate := time.Now().Truncate(24 * time.Hour)
	var prices []modelsdb.GoldPrice
	var crawlErr error

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	// Each data row has a <th> for product name and two <td> cells for buy/sell.
	// Prices are in x1000đ/lượng, so multiply by 1000 to get full VND value.
	thousand := decimal.NewFromInt(1000)
	c.OnHTML("table tr", func(e *colly.HTMLElement) {
		name := strings.TrimSpace(e.ChildText("th"))
		if name == "" {
			return
		}
		nameLower := strings.ToLower(name)

		var productType string
		for _, m := range btmhProductMap {
			if strings.Contains(nameLower, m.keyword) {
				productType = m.productType
				break
			}
		}
		if productType == "" {
			return
		}

		cells := e.ChildTexts("td")
		if len(cells) < 2 {
			return
		}

		buyPrice, bErr := parsePriceVND(cells[0])
		if bErr != nil {
			logger.Logger.Warnf("[gold-crawler] BTMH skipping %q: invalid buy price %q: %v", name, cells[0], bErr)
			return
		}

		// Sell price may be "-"; treat as 0 rather than skipping the whole row.
		var sellPrice decimal.Decimal
		if sp, sErr := parsePriceVND(cells[1]); sErr == nil {
			sellPrice = sp.Mul(thousand)
		}

		prices = append(prices, modelsdb.GoldPrice{
			Source:      "BTMH",
			ProductType: productType,
			TradingDate: tradingDate,
			BuyPrice:    buyPrice.Mul(thousand),
			SellPrice:   sellPrice,
			Currency:    "VND",
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		crawlErr = fmt.Errorf("BTMH HTTP %d: %v", r.StatusCode, err)
	})

	if reqErr := c.Request("GET", btmhURL, nil, colly.NewContext(), nil); reqErr != nil {
		return nil, fmt.Errorf("BTMH request failed: %v", reqErr)
	}

	// Wait for all async Colly callbacks to complete.
	c.Wait()

	if crawlErr != nil {
		return nil, crawlErr
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("BTMH returned no recognised products")
	}
	return prices, nil
}

// --- vang.today types and helpers ---

const (
	vangTodayPricesURL  = "https://www.vang.today/api/prices"
	vangTodayHistoryURL = "https://www.vang.today/api/prices?type=%s&days=30"
)

// vangTodayPricesResponse maps the top-level JSON from /api/prices.
type vangTodayPricesResponse struct {
	Success     bool                   `json:"success"`
	CurrentTime int64                  `json:"current_time"`
	Data        []vangTodayPriceItem   `json:"data"`
}

type vangTodayPriceItem struct {
	TypeCode   string  `json:"type_code"`
	Buy        float64 `json:"buy"`
	Sell       float64 `json:"sell"`
	ChangeBuy  float64 `json:"change_buy"`
	ChangeSell float64 `json:"change_sell"`
	UpdateTime int64   `json:"update_time"`
}

// vangTodayHistoryResponse maps the JSON from /api/prices?type=X&days=30.
type vangTodayHistoryResponse struct {
	Success bool                       `json:"success"`
	Days    int                        `json:"days"`
	Type    string                     `json:"type"`
	History []vangTodayHistoryDay      `json:"history"`
}

type vangTodayHistoryDay struct {
	Date   string                              `json:"date"`
	Prices map[string]vangTodayHistoryProduct  `json:"prices"`
}

type vangTodayHistoryProduct struct {
	Name string  `json:"name"`
	Buy  float64 `json:"buy"`
	Sell float64 `json:"sell"`
}

// vangTodayTypeMap defines the 4 type_codes we track, mapped to DB source and
// product_type. Prices from vang.today are already full VND per lượng.
var vangTodayTypeMap = []struct {
	typeCode    string
	source      string
	productType string
}{
	{"SJL1L10", "SJC", "sjc"},
	{"SJ9999", "SJC", "nhan_tron"},
	{"DOHNL", "DOJI", "sjc"},
	{"PQHNVM", "PNJ", "nhan_tron"},
	{"BTSJC", "BTMC", "sjc"},
	{"BT9999NTT", "BTMC", "nhan_tron"},
}

// crawlVangToday fetches current gold prices from vang.today and returns
// records for the 4 configured type_codes. Prices are stored as-is (full VND
// per lượng). Type_codes not in vangTodayTypeMap are silently ignored.
func crawlVangToday(ctx context.Context, client *http.Client) ([]modelsdb.GoldPrice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, vangTodayPricesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vang.today API HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result vangTodayPricesResponse
	if jsonErr := json.Unmarshal(body, &result); jsonErr != nil {
		return nil, fmt.Errorf("vang.today JSON parse error: %v", jsonErr)
	}
	if !result.Success {
		return nil, fmt.Errorf("vang.today API returned success=false")
	}

	// Build a lookup from type_code → mapping config.
	lookup := make(map[string]struct {
		source      string
		productType string
	}, len(vangTodayTypeMap))
	for _, m := range vangTodayTypeMap {
		lookup[m.typeCode] = struct {
			source      string
			productType string
		}{m.source, m.productType}
	}

	tradingDate := time.Now().Truncate(24 * time.Hour)
	var prices []modelsdb.GoldPrice

	for _, item := range result.Data {
		mapping, ok := lookup[item.TypeCode]
		if !ok {
			continue
		}
		if item.Buy == 0 {
			logger.Logger.Warnf("[gold-crawler] vang.today skipping %s: buy price is zero", item.TypeCode)
			continue
		}

		buyPrice := decimal.NewFromFloat(item.Buy)
		sellPrice := decimal.NewFromFloat(item.Sell)

		prices = append(prices, modelsdb.GoldPrice{
			Source:      mapping.source,
			ProductType: mapping.productType,
			TradingDate: tradingDate,
			BuyPrice:    buyPrice,
			SellPrice:   sellPrice,
			Currency:    "VND",
		})
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("vang.today returned no recognised type_codes")
	}
	return prices, nil
}

// ImportVangTodayHistory fetches 30 days of historical prices for each
// configured vang.today type_code and upserts them into the DB. This is
// intended as a one-time backfill called on startup (non-blocking goroutine).
func ImportVangTodayHistory() {
	logger.Logger.Info("[vangtoday-history] Starting vang.today historical import (30 days per type)")
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	store := repository.GetSingleton()
	if store == nil {
		logger.Logger.Error("[vangtoday-history] database store not available")
		return
	}

	client := &http.Client{Timeout: goldTimeout}

	savedTotal := 0
	skippedTotal := 0
	errorTotal := 0

	for _, mapping := range vangTodayTypeMap {
		url := fmt.Sprintf(vangTodayHistoryURL, mapping.typeCode)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			logger.Logger.Errorf("[vangtoday-history] Failed to build request for %s: %v", mapping.typeCode, err)
			errorTotal++
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			logger.Logger.Errorf("[vangtoday-history] Request failed for %s: %v", mapping.typeCode, err)
			errorTotal++
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Logger.Errorf("[vangtoday-history] HTTP %d for %s", resp.StatusCode, mapping.typeCode)
			errorTotal++
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Logger.Errorf("[vangtoday-history] Failed to read body for %s: %v", mapping.typeCode, err)
			errorTotal++
			continue
		}

		var historical vangTodayHistoryResponse
		if jsonErr := json.Unmarshal(body, &historical); jsonErr != nil {
			logger.Logger.Errorf("[vangtoday-history] JSON parse error for %s: %v", mapping.typeCode, jsonErr)
			errorTotal++
			continue
		}
		if !historical.Success {
			logger.Logger.Errorf("[vangtoday-history] API returned success=false for %s", mapping.typeCode)
			errorTotal++
			continue
		}

		savedForType := 0
		for _, day := range historical.History {
			product, ok := day.Prices[mapping.typeCode]
			if !ok {
				skippedTotal++
				continue
			}
			if product.Buy == 0 {
				skippedTotal++
				continue
			}

			tradingDate, parseErr := time.ParseInLocation("2006-01-02", day.Date, time.Local)
			if parseErr != nil {
				logger.Logger.Warnf("[vangtoday-history] Cannot parse date %q for %s: %v", day.Date, mapping.typeCode, parseErr)
				skippedTotal++
				continue
			}
			tradingDate = tradingDate.Truncate(24 * time.Hour)

			record := &modelsdb.GoldPrice{
				Source:      mapping.source,
				ProductType: mapping.productType,
				TradingDate: tradingDate,
				BuyPrice:    decimal.NewFromFloat(product.Buy),
				SellPrice:   decimal.NewFromFloat(product.Sell),
				Currency:    "VND",
			}
			if upsertErr := store.UpsertGoldPrice(record); upsertErr != nil {
				logger.Logger.Errorf("[vangtoday-history] Failed to upsert %s/%s for %s: %v",
					mapping.source, mapping.productType, day.Date, upsertErr)
				errorTotal++
			} else {
				savedForType++
				savedTotal++
			}
		}
		logger.Logger.Infof("[vangtoday-history] %s (%s/%s): %d days saved", mapping.typeCode, mapping.source, mapping.productType, savedForType)
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("[vangtoday-history] Completed in %v: %d saved, %d skipped, %d errors",
		duration, savedTotal, skippedTotal, errorTotal)
}

// --- Phú Quý types and helpers ---

const phuQuyURL = "https://gold.phuquy.com.vn"

// phuQuyProductMap maps substrings in the product-name column to our
// ProductType identifiers. Checked in order; first match per product_type wins.
var phuQuyProductMap = []struct {
	keyword     string
	productType string
}{
	{"sjc", "sjc"},
	{"nhẫn tròn", "nhan_tron"},
}

// crawlPhuQuy scrapes gold prices from gold.phuquy.com.vn using Colly.
// The page has a price table whose rows contain product name + buy/sell columns.
// Prices on the page are in VND per chỉ; they are multiplied by 10 before
// saving so all domestic VND prices are stored per lượng. First match per
// product_type wins (deduplication).
func crawlPhuQuy(_ context.Context, _ *http.Client) ([]modelsdb.GoldPrice, error) {
	tradingDate := time.Now().Truncate(24 * time.Hour)
	var prices []modelsdb.GoldPrice
	var crawlErr error
	ten := decimal.NewFromInt(10)

	// seen tracks which product_types have been captured (first match wins).
	seen := make(map[string]bool)

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"),
	)

	// The Phú Quý table uses <tr> rows with <td> cells.
	// Typical layout: td[0]=product name, td[1]=buy price, td[2]=sell price.
	c.OnHTML("table tr", func(e *colly.HTMLElement) {
		cells := e.ChildTexts("td")
		if len(cells) < 3 {
			return
		}

		name := strings.TrimSpace(cells[0])
		if name == "" {
			return
		}
		nameLower := strings.ToLower(name)

		var productType string
		for _, m := range phuQuyProductMap {
			if strings.Contains(nameLower, m.keyword) {
				productType = m.productType
				break
			}
		}
		if productType == "" || seen[productType] {
			return
		}

		buyPerChi, bErr := parsePriceVND(cells[1])
		if bErr != nil {
			logger.Logger.Warnf("[gold-crawler] PhuQuy skipping %q: invalid buy price %q: %v", name, cells[1], bErr)
			return
		}

		// Sell price may be absent or dashed; treat zero as zero rather than
		// skipping the whole row.
		var sellPerChi decimal.Decimal
		if sp, sErr := parsePriceVND(cells[2]); sErr == nil {
			sellPerChi = sp
		}

		// Convert per chỉ → per lượng.
		seen[productType] = true
		prices = append(prices, modelsdb.GoldPrice{
			Source:      "PHUQUY",
			ProductType: productType,
			TradingDate: tradingDate,
			BuyPrice:    buyPerChi.Mul(ten),
			SellPrice:   sellPerChi.Mul(ten),
			Currency:    "VND",
		})
	})

	c.OnError(func(r *colly.Response, err error) {
		crawlErr = fmt.Errorf("PhuQuy HTTP %d: %v", r.StatusCode, err)
	})

	if reqErr := c.Request("GET", phuQuyURL, nil, colly.NewContext(), nil); reqErr != nil {
		return nil, fmt.Errorf("PhuQuy request failed: %v", reqErr)
	}

	c.Wait()

	if crawlErr != nil {
		return nil, crawlErr
	}
	if len(prices) == 0 {
		return nil, fmt.Errorf("PhuQuy returned no recognised products")
	}
	return prices, nil
}

// parsePriceVND strips thousands separators (commas, dots used as separators)
// and whitespace from a Vietnamese price string such as "155,500,000" and
// returns the value as a Decimal. Returns an error for empty or zero values.
func parsePriceVND(raw string) (decimal.Decimal, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if cleaned == "" || cleaned == "-" {
		return decimal.Zero, fmt.Errorf("empty price")
	}
	d, err := decimal.NewFromString(cleaned)
	if err != nil {
		return decimal.Zero, err
	}
	if d.IsZero() {
		return decimal.Zero, fmt.Errorf("zero price")
	}
	return d, nil
}
