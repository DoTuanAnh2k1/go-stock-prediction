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
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/store/repository"
)

const (
	vndirectBaseURL = "https://api-finfo.vndirect.com.vn/v4/stock_prices"
	httpTimeout     = 15 * time.Second
	requestDelay    = 300 * time.Millisecond // 300ms giữa các mã
)

// vndirectResponse là struct map JSON trả về từ VNDirect API
type vndirectResponse struct {
	Data []vndirectStockPrice `json:"data"`
}

type vndirectStockPrice struct {
	Code      string  `json:"code"`
	Date      string  `json:"date"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	NmVolume  float64 `json:"nmVolume"`
	NmValue   float64 `json:"nmValue"`
	PtVolume  float64 `json:"ptVolume"`
	PtValue   float64 `json:"ptValue"`
	Change    float64 `json:"change"`
	PctChange float64 `json:"pctChange"`
}

func NewVietStockCrawler() *VietStockCrawler {
	return &VietStockCrawler{
		client: &http.Client{
			Timeout: httpTimeout,
		},
		isRunning: false,
	}
}

// crawlVN30Data crawls all VN30 stocks using VNDirect public API
func (vc *VietStockCrawler) crawlVN30Data(ctx context.Context) (*CrawlResult, error) {
	vc.mutex.Lock()
	if vc.isRunning {
		vc.mutex.Unlock()
		return nil, fmt.Errorf("crawler đang chạy rồi, đợi tí")
	}
	vc.isRunning = true
	vc.mutex.Unlock()
	defer func() {
		vc.mutex.Lock()
		vc.isRunning = false
		vc.mutex.Unlock()
	}()

	logger.Logger.Info("[crawler] Starting VN30 crawl from VNDirect API")
	startTime := time.Now()

	var stocks []modelssvc.VN30Stock
	var errCount int

	for _, symbol := range VN30Symbols {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		stock, err := vc.fetchStockPrice(ctx, symbol)
		if err != nil {
			logger.Logger.Errorf("[crawler] Failed to fetch %s: %v", symbol, err)
			errCount++
			continue
		}

		stocks = append(stocks, *stock)
		logger.Logger.Debugf("[crawler] Fetched %s: close=%.1f vol=%.0f", symbol, stock.Price, float64(stock.Volume))

		// Nghỉ giữa các request tránh rate limit
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(requestDelay):
		}
	}

	duration := time.Since(startTime)
	logger.Logger.Infof("[crawler] Crawl completed in %v: %d stocks, %d errors", duration, len(stocks), errCount)

	result := &CrawlResult{
		Stocks: stocks,
		Count:  len(stocks),
		Source: "VNDirect",
	}

	if len(stocks) == 0 {
		result.Error = fmt.Errorf("no data available")
	}

	return result, nil
}

// fetchStockPrice fetches the latest price for a single stock from VNDirect
func (vc *VietStockCrawler) fetchStockPrice(ctx context.Context, symbol string) (*modelssvc.VN30Stock, error) {
	url := fmt.Sprintf("%s?sort=date&q=code:%s&size=1", vndirectBaseURL, symbol)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := vc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result vndirectResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON parse error: %v", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no data for symbol %s", symbol)
	}

	d := result.Data[0]

	tradeDate, err := time.ParseInLocation("2006-01-02", d.Date, time.Local)
	if err != nil {
		tradeDate = time.Now()
	}

	totalVolume := int64(d.NmVolume + d.PtVolume)
	totalValue := int64(d.NmValue + d.PtValue)

	return &modelssvc.VN30Stock{
		Symbol:        d.Code,
		Price:         d.Close,
		Open:          d.Open,
		High:          d.High,
		Low:           d.Low,
		Change:        d.Change,
		ChangePercent: d.PctChange,
		Volume:        totalVolume,
		Value:         totalValue,
		Timestamp:     tradeDate,
	}, nil
}

// crawlSingleStock fetches data for a single stock (used by service layer)
func (vc *VietStockCrawler) crawlSingleStock(ctx context.Context, symbol string) (*modelssvc.VN30Stock, error) {
	return vc.fetchStockPrice(ctx, symbol)
}

// fetchHistoricalPrices fetches up to `days` trading days of price history for a single stock.
func (vc *VietStockCrawler) fetchHistoricalPrices(ctx context.Context, symbol string, days int) ([]modelssvc.VN30Stock, error) {
	url := fmt.Sprintf("%s?sort=date&q=code:%s&size=%d", vndirectBaseURL, symbol, days)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := vc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result vndirectResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON parse error: %v", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no data for symbol %s", symbol)
	}

	stocks := make([]modelssvc.VN30Stock, 0, len(result.Data))
	for _, d := range result.Data {
		tradeDate, err := time.ParseInLocation("2006-01-02", d.Date, time.Local)
		if err != nil {
			tradeDate = time.Now()
		}
		totalVolume := int64(d.NmVolume + d.PtVolume)
		totalValue := int64(d.NmValue + d.PtValue)
		stocks = append(stocks, modelssvc.VN30Stock{
			Symbol:        d.Code,
			Price:         d.Close,
			Open:          d.Open,
			High:          d.High,
			Low:           d.Low,
			Change:        d.Change,
			ChangePercent: d.PctChange,
			Volume:        totalVolume,
			Value:         totalValue,
			Timestamp:     tradeDate,
		})
	}

	return stocks, nil
}

// crawlHistoricalAll fetches `days` trading days of history for every VN30 stock and
// upserts them into the database. It returns the number of rows saved and skipped (errors).
func (vc *VietStockCrawler) crawlHistoricalAll(ctx context.Context, days int) (saved int, skipped int, err error) {
	store := repository.GetSingleton()
	if store == nil {
		return 0, 0, fmt.Errorf("database store not available")
	}

	exchange, err := getOrCreateHOSEExchange(store)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get exchange: %v", err)
	}

	logger.Logger.Infof("[historical] Starting historical crawl: %d symbols, %d days each", len(VN30Symbols), days)
	startTime := time.Now()

	for _, symbol := range VN30Symbols {
		select {
		case <-ctx.Done():
			return saved, skipped, ctx.Err()
		default:
		}

		history, fetchErr := vc.fetchHistoricalPrices(ctx, symbol, days)
		if fetchErr != nil {
			logger.Logger.Errorf("[historical] Failed to fetch %s: %v", symbol, fetchErr)
			skipped++
			// delay before next symbol even on error
			select {
			case <-ctx.Done():
				return saved, skipped, ctx.Err()
			case <-time.After(requestDelay):
			}
			continue
		}

		// Get or create the stock record once per symbol
		if len(history) == 0 {
			skipped++
			continue
		}

		stock, stockErr := getOrCreateStock(store, history[0], exchange.ID)
		if stockErr != nil {
			logger.Logger.Errorf("[historical] Failed to get/create stock %s: %v", symbol, stockErr)
			skipped += len(history)
			select {
			case <-ctx.Done():
				return saved, skipped, ctx.Err()
			case <-time.After(requestDelay):
			}
			continue
		}

		for _, h := range history {
			sp := &modelsdb.StockPrice{
				StockID:       stock.ID,
				TradingDate:   h.Timestamp,
				OpenPrice:     decimal.NewFromFloat(h.Open),
				HighPrice:     decimal.NewFromFloat(h.High),
				LowPrice:      decimal.NewFromFloat(h.Low),
				ClosePrice:    decimal.NewFromFloat(h.Price),
				Volume:        h.Volume,
				Value:         decimal.NewFromInt(h.Value),
				Change:        decimal.NewFromFloat(h.Change),
				ChangePercent: decimal.NewFromFloat(h.ChangePercent),
			}
			if upsertErr := store.UpsertStockPrice(sp); upsertErr != nil {
				logger.Logger.Errorf("[historical] Upsert failed %s %s: %v", symbol, h.Timestamp.Format("2006-01-02"), upsertErr)
				skipped++
			} else {
				saved++
			}
		}

		logger.Logger.Debugf("[historical] %s: saved %d rows so far", symbol, len(history))

		select {
		case <-ctx.Done():
			return saved, skipped, ctx.Err()
		case <-time.After(requestDelay):
		}
	}

	logger.Logger.Infof("[historical] Completed in %v: saved=%d skipped=%d", time.Since(startTime), saved, skipped)
	return saved, skipped, nil
}

// IsRunning returns whether the crawler is currently running
func (vc *VietStockCrawler) IsRunning() bool {
	vc.mutex.RLock()
	defer vc.mutex.RUnlock()
	return vc.isRunning
}
