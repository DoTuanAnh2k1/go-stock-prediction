package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/utils/parse"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
	"golang.org/x/time/rate"
)

func NewVietStockCrawler() *VietStockCrawler {
	// Tạo HTTP client với timeout và custom transport
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 5,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	return &VietStockCrawler{
		scraper:     modelssvc.NewVietStockScraper(),
		rateLimiter: rate.NewLimiter(rate.Every(2*time.Second), 1), // 1 request mỗi 2 giây
		client:      client,
		isRunning:   false,
	}
}

// crawlVN30Data - Crawl toàn bộ data VN30 từ VietStock
func (vc *VietStockCrawler) crawlVN30Data(ctx context.Context) (*CrawlResult, error) {
	vc.mutex.Lock()
	if vc.isRunning {
		vc.mutex.Unlock()
		return nil, fmt.Errorf("crawler đang chạy rồi, đợi tí đi mày")
	}
	vc.isRunning = true
	vc.mutex.Unlock()

	defer func() {
		vc.mutex.Lock()
		vc.isRunning = false
		vc.mutex.Unlock()
	}()

	logger.Logger.Info("[crawler] Starting VN30 crawl from VietStock")
	startTime := time.Now()

	// Tạo collector mới cho mỗi lần crawl
	c := colly.NewCollector(
		colly.Debugger(&debug.LogDebugger{}),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"),
	)

	// Setup anti-detection
	vc.setupAntiDetection(c)

	var stocks []modelssvc.VN30Stock
	var crawlErrors []error
	var mutex sync.Mutex

	// Setup HTML parser cho VN30 table
	c.OnHTML("table", func(e *colly.HTMLElement) {
		// Tìm table chứa data VN30
		if vc.isVN30Table(e) {
			logger.Logger.Info("[crawler] Found VN30 table, parsing...")

			stocksFromTable := vc.parseVN30Table(e)

			mutex.Lock()
			stocks = append(stocks, stocksFromTable...)
			mutex.Unlock()

			logger.Logger.Infof("[crawler] Parsed %d stocks from table", len(stocksFromTable))
		}
	})

	// Setup JSON response handler (nếu site trả về JSON)
	c.OnResponse(func(r *colly.Response) {
		contentType := r.Headers.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			logger.Logger.Info("[crawler] Received JSON response, parsing...")

			stocksFromJSON, err := vc.parseJSONResponse(r.Body)
			if err != nil {
				mutex.Lock()
				crawlErrors = append(crawlErrors, fmt.Errorf("lỗi parse JSON: %v", err))
				mutex.Unlock()
				return
			}

			mutex.Lock()
			stocks = append(stocks, stocksFromJSON...)
			mutex.Unlock()

			logger.Logger.Infof("[crawler] Parsed %d stocks from JSON", len(stocksFromJSON))
		}
	})

	// Error handling
	c.OnError(func(r *colly.Response, err error) {
		mutex.Lock()
		crawlErrors = append(crawlErrors, fmt.Errorf("crawl error cho URL %s: %v", r.Request.URL, err))
		mutex.Unlock()
		logger.Logger.Errorf("[crawler] Crawl error: %v", err)
	})

	// Rate limiting
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*vietstock.vn*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	// URLs to crawl - thử nhiều endpoints
	urls := []string{
		"https://banggia.vietstock.vn/bang-gia/vn30",
		"https://finance.vietstock.vn/du-lieu/danh-sach-ma-chung-khoan?cat=vn30",
		"https://vietstock.vn/chung-khoan/vn30",
	}

	// Crawl từng URL
	for _, url := range urls {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		logger.Logger.Infof("[crawler] Crawling: %s", url)

		// Rate limiting
		if err := vc.rateLimiter.Wait(ctx); err != nil {
			crawlErrors = append(crawlErrors, fmt.Errorf("rate limiter error: %v", err))
			continue
		}

		// Rotate user agent trước khi visit
		vc.rotateHeaders(c)

		err := c.Visit(url)
		if err != nil {
			logger.Logger.Errorf("[crawler] Failed to visit %s: %v", url, err)
			crawlErrors = append(crawlErrors, err)
			continue
		}

		// Chờ một chút trước khi crawl URL tiếp theo
		time.Sleep(1 * time.Second)
	}

	// Wait for all requests to finish
	c.Wait()

	duration := time.Since(startTime)
	logger.Logger.Infof("[crawler] Crawl completed in %v", duration)

	// Tạo result
	result := &CrawlResult{
		Stocks: vc.removeDuplicates(stocks),
		Count:  len(stocks),
		Source: "VietStock",
	}

	if len(crawlErrors) > 0 {
		result.Error = fmt.Errorf("có %d lỗi trong quá trình crawl: %v", len(crawlErrors), crawlErrors[0])
	}

	logger.Logger.Infof("[crawler] Crawled %d VN30 stocks successfully", result.Count)
	return result, nil
}

// crawlSingleStock - Crawl data của 1 mã cụ thể
func (vc *VietStockCrawler) crawlSingleStock(ctx context.Context, symbol string) (*modelssvc.VN30Stock, error) {
	logger.Logger.Infof("[crawler] Crawling single stock: %s", symbol)

	if err := vc.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %v", err)
	}

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)

	vc.setupAntiDetection(c)

	var stock *modelssvc.VN30Stock
	var stockError error

	// Parse single stock data
	c.OnHTML("div.stock-info, .stock-detail, .quote-summary", func(e *colly.HTMLElement) {
		parsedStock := vc.parseSingleStockData(e, symbol)
		if parsedStock != nil {
			stock = parsedStock
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		stockError = fmt.Errorf("lỗi crawl %s: %v", symbol, err)
	})

	// URLs cho single stock
	urls := []string{
		fmt.Sprintf("https://finance.vietstock.vn/%s/overview", strings.ToLower(symbol)),
		fmt.Sprintf("https://banggia.vietstock.vn/gia-%s", strings.ToLower(symbol)),
	}

	for _, url := range urls {
		vc.rotateHeaders(c)
		err := c.Visit(url)
		if err != nil {
			logger.Logger.Warnf("[crawler] Cannot crawl from %s: %v", url, err)
			continue
		}

		if stock != nil {
			break // Đã có data rồi thì thôi
		}

		time.Sleep(500 * time.Millisecond)
	}

	c.Wait()

	if stockError != nil {
		return nil, stockError
	}

	if stock == nil {
		return nil, fmt.Errorf("không tìm thấy data cho mã %s", symbol)
	}

	logger.Logger.Infof("[crawler] Crawled %s: price=%.0f", symbol, stock.Price)
	return stock, nil
}

// crawlWithRetry wraps crawlSingleStock with exponential backoff retry logic
func (vc *VietStockCrawler) crawlWithRetry(ctx context.Context, symbol string, maxRetries int) (*modelssvc.VN30Stock, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			logger.Logger.Warnf("[crawler] Retrying %s (attempt %d/%d)", symbol, attempt+1, maxRetries)
		}

		stock, err := vc.crawlSingleStock(ctx, symbol)
		if err != nil {
			lastErr = err
			logger.Logger.Warnf("[crawler] Crawl attempt %d failed for %s: %v", attempt+1, symbol, err)
			continue
		}
		return stock, nil
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// Setup anti-detection measures
func (vc *VietStockCrawler) setupAntiDetection(c *colly.Collector) {
	// Random delays
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*vietstock.vn*",
		Parallelism: 1,
		Delay:       time.Duration(1+rand.Intn(2)) * time.Second,
	})

	// Setup request interceptor
	c.OnRequest(func(r *colly.Request) {
		// Add common headers
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
		r.Headers.Set("Accept-Encoding", "gzip, deflate, br")
		r.Headers.Set("DNT", "1")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Cache-Control", "max-age=0")

		// Add referer
		r.Headers.Set("Referer", "https://vietstock.vn/")
	})
}

// Rotate headers to avoid detection
func (vc *VietStockCrawler) rotateHeaders(c *colly.Collector) {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
	}

	ua := userAgents[rand.Intn(len(userAgents))]
	c.UserAgent = ua
}

// Check if table contains VN30 data
func (vc *VietStockCrawler) isVN30Table(e *colly.HTMLElement) bool {
	// Check table headers or class names
	tableText := strings.ToLower(e.Text)

	// Kiểm tra có chứa VN30 symbols không
	symbolCount := 0
	for _, symbol := range VN30Symbols[:5] { // Check first 5 symbols
		if strings.Contains(tableText, strings.ToLower(symbol)) {
			symbolCount++
		}
	}

	// Nếu có ít nhất 3 symbols trong VN30 thì coi như đúng table
	return symbolCount >= 3
}

// Parse VN30 table data
func (vc *VietStockCrawler) parseVN30Table(e *colly.HTMLElement) []modelssvc.VN30Stock {
	var stocks []modelssvc.VN30Stock

	// Parse từng row trong table
	e.ForEach("tr", func(i int, row *colly.HTMLElement) {
		// Skip header row
		if i == 0 || row.ChildText("th") != "" {
			return
		}

		stock := vc.parseTableRow(row)
		if stock != nil && vc.isValidVN30Symbol(stock.Symbol) {
			stocks = append(stocks, *stock)
		}
	})

	return stocks
}

// Parse single table row
func (vc *VietStockCrawler) parseTableRow(row *colly.HTMLElement) *modelssvc.VN30Stock {
	cells := row.ChildTexts("td")
	if len(cells) < 6 {
		return nil // Không đủ dữ liệu
	}

	symbol := strings.TrimSpace(cells[0])
	if symbol == "" {
		return nil
	}

	stock := &modelssvc.VN30Stock{
		Symbol:    symbol,
		Price:     vc.parseFloatValue(cells[1]),
		Change:    vc.parseFloatValue(cells[2]),
		Timestamp: time.Now(),
	}

	// Parse thêm fields nếu có
	if len(cells) > 3 {
		stock.ChangePercent = vc.parseFloatValue(cells[3])
	}
	if len(cells) > 4 {
		stock.Volume = vc.parseInt64Value(cells[4])
	}
	if len(cells) > 5 {
		stock.Value = vc.parseInt64Value(cells[5])
	}

	// Parse OHLC nếu có
	if len(cells) > 6 {
		stock.High = vc.parseFloatValue(cells[6])
	}
	if len(cells) > 7 {
		stock.Low = vc.parseFloatValue(cells[7])
	}
	if len(cells) > 8 {
		stock.Open = vc.parseFloatValue(cells[8])
	}

	return stock
}

// Parse JSON response
func (vc *VietStockCrawler) parseJSONResponse(body []byte) ([]modelssvc.VN30Stock, error) {
	var stocks []modelssvc.VN30Stock

	// Try different JSON structures
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("không parse được JSON: %v", err)
	}

	// Look for array of stocks in different keys
	stockArrayKeys := []string{"data", "stocks", "result", "items", "list"}

	for _, key := range stockArrayKeys {
		if stockArray, ok := data[key].([]interface{}); ok {
			for _, item := range stockArray {
				if stockMap, ok := item.(map[string]interface{}); ok {
					stock := vc.parseJSONStock(stockMap)
					if stock != nil && vc.isValidVN30Symbol(stock.Symbol) {
						stocks = append(stocks, *stock)
					}
				}
			}
			break
		}
	}

	return stocks, nil
}

// Parse single stock from JSON
func (vc *VietStockCrawler) parseJSONStock(stockMap map[string]interface{}) *modelssvc.VN30Stock {
	symbol, _ := stockMap["symbol"].(string)
	if symbol == "" {
		symbol, _ = stockMap["code"].(string)
	}

	if symbol == "" {
		return nil
	}

	stock := &modelssvc.VN30Stock{
		Symbol:    strings.ToUpper(symbol),
		Timestamp: time.Now(),
	}

	// Parse price fields
	if price, ok := stockMap["price"].(float64); ok {
		stock.Price = price
	} else if priceStr, ok := stockMap["price"].(string); ok {
		stock.Price = vc.parseFloatValue(priceStr)
	}

	// Parse change
	if change, ok := stockMap["change"].(float64); ok {
		stock.Change = change
	}

	// Parse volume
	if volume, ok := stockMap["volume"].(float64); ok {
		stock.Volume = int64(volume)
	}

	return stock
}

// Parse single stock data from HTML
func (vc *VietStockCrawler) parseSingleStockData(e *colly.HTMLElement, symbol string) *modelssvc.VN30Stock {
	stock := &modelssvc.VN30Stock{
		Symbol:    strings.ToUpper(symbol),
		Timestamp: time.Now(),
	}

	// Try to find price in different selectors
	priceSelectors := []string{
		".price, .current-price, .quote-price",
		".stock-price, .last-price",
		"[data-field='price'], [data-field='lastPrice']",
	}

	for _, selector := range priceSelectors {
		priceText := e.ChildText(selector)
		if priceText != "" {
			stock.Price = vc.parseFloatValue(priceText)
			break
		}
	}

	// Parse other fields similarly
	changeText := e.ChildText(".change, .price-change, [data-field='change']")
	stock.Change = vc.parseFloatValue(changeText)

	volumeText := e.ChildText(".volume, .trade-volume, [data-field='volume']")
	stock.Volume = vc.parseInt64Value(volumeText)

	return stock
}

// Utility functions
func (vc *VietStockCrawler) parseFloatValue(s string) float64 {
	return parse.ParseFloat(s)
}

func (vc *VietStockCrawler) parseInt64Value(s string) int64 {
	return parse.ParseInt64(s)
}

func (vc *VietStockCrawler) isValidVN30Symbol(symbol string) bool {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	for _, vn30Symbol := range VN30Symbols {
		if symbol == vn30Symbol {
			return true
		}
	}
	return false
}

// Remove duplicate stocks
func (vc *VietStockCrawler) removeDuplicates(stocks []modelssvc.VN30Stock) []modelssvc.VN30Stock {
	seen := make(map[string]bool)
	var result []modelssvc.VN30Stock

	for _, stock := range stocks {
		if !seen[stock.Symbol] {
			seen[stock.Symbol] = true
			result = append(result, stock)
		}
	}

	return result
}

// Get crawler status
func (vc *VietStockCrawler) IsRunning() bool {
	vc.mutex.RLock()
	defer vc.mutex.RUnlock()
	return vc.isRunning
}
