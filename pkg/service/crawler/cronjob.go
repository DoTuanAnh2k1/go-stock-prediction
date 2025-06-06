package crawler

import (
	"context"
	"fmt"
	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/store/repository"
	"time"

	"github.com/shopspring/decimal"
)

func CronjobCrawler() error {
	logger.Logger.Info("🚀 Starting VN30 stocks crawler cronjob...")
	startTime := time.Now()

	// Initialize context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Get database store instance
	store := repository.GetSingleton()
	if store == nil {
		logger.Logger.Error("❌ Failed to connect to database store")
		return fmt.Errorf("database store not available")
	}

	// Start crawling VN30 data
	result, err := crawler.crawlVN30Data(ctx)
	if err != nil {
		logger.Logger.Errorf("❌ Failed to crawl VN30 data: %v", err)
		// Log error to sync_log
		logSyncError(store, err, startTime)
		return err
	}

	if result == nil || len(result.Stocks) == 0 {
		logger.Logger.Warn("⚠️ No data crawled from source")
		logSyncError(store, fmt.Errorf("no data available"), startTime)
		return fmt.Errorf("no data crawled")
	}

	logger.Logger.Infof("📊 Successfully crawled %d VN30 stocks", len(result.Stocks))

	// Process and save data to database
	successCount, errorCount := processAndSaveStockData(store, result.Stocks)

	// Log sync results
	duration := time.Since(startTime)
	err = logSyncResult(store, successCount, errorCount, duration, result.Source)
	if err != nil {
		logger.Logger.Errorf("❌ Failed to log sync result: %v", err)
	}

	logger.Logger.Infof("✅ Crawler cronjob completed: %d successful, %d errors in %v",
		successCount, errorCount, duration)

	return nil
}

// processAndSaveStockData processes and saves stock data to database
func processAndSaveStockData(store repository.DatabaseStore, stocks []modelssvc.VN30Stock) (int, int) {
	successCount := 0
	errorCount := 0

	// Get or create HOSE exchange (default for VN30)
	exchange, err := getOrCreateHOSEExchange(store)
	if err != nil {
		logger.Logger.Errorf("❌ Failed to get/create HOSE exchange: %v", err)
		return 0, len(stocks)
	}

	for _, stockData := range stocks {
		err := processingSingleStock(store, stockData, exchange.ID)
		if err != nil {
			logger.Logger.Errorf("❌ Failed to process stock %s: %v", stockData.Symbol, err)
			errorCount++
			continue
		}

		successCount++
		logger.Logger.Debugf("✅ Successfully saved stock %s: price %.0f", stockData.Symbol, stockData.Price)
	}

	return successCount, errorCount
}

// processingSingleStock processes and saves a single stock to database
func processingSingleStock(store repository.DatabaseStore, stockData modelssvc.VN30Stock, exchangeID uint) error {
	// 1. Get or create stock record
	stock, err := getOrCreateStock(store, stockData, exchangeID)
	if err != nil {
		return fmt.Errorf("failed to get/create stock: %v", err)
	}

	// 2. Create stock price record
	stockPrice := &modelsdb.StockPrice{
		StockID:       stock.ID,
		TradingDate:   stockData.Timestamp,
		OpenPrice:     decimal.NewFromFloat(stockData.Open),
		HighPrice:     decimal.NewFromFloat(stockData.High),
		LowPrice:      decimal.NewFromFloat(stockData.Low),
		ClosePrice:    decimal.NewFromFloat(stockData.Price),
		Volume:        stockData.Volume,
		Value:         decimal.NewFromInt(stockData.Value),
		Change:        decimal.NewFromFloat(stockData.Change),
		ChangePercent: decimal.NewFromFloat(stockData.ChangePercent),
	}

	// 3. Upsert stock price (update if exists, create if not)
	err = store.UpsertStockPrice(stockPrice)
	if err != nil {
		return fmt.Errorf("failed to upsert stock price: %v", err)
	}

	return nil
}

// getOrCreateStock gets or creates a stock record
func getOrCreateStock(store repository.DatabaseStore, stockData modelssvc.VN30Stock, exchangeID uint) (*modelsdb.Stock, error) {
	// Try to get existing stock
	stock, err := store.GetStockBySymbol(stockData.Symbol)
	if err == nil {
		// Stock exists, update info if needed
		return stock, nil
	}

	// Stock doesn't exist, create new one
	companyName := VN30SymbolNames[stockData.Symbol]
	if companyName == "" {
		companyName = stockData.Symbol + " Company" // Fallback name
	}

	newStock := &modelsdb.Stock{
		Symbol:      stockData.Symbol,
		CompanyName: companyName,
		ExchangeID:  exchangeID,
		IsVN30:      true, // We're crawling VN30
		IsVN100:     true, // VN30 is also part of VN100
		Sector:      determineSector(stockData.Symbol),
	}

	err = store.CreateStock(newStock)
	if err != nil {
		return nil, fmt.Errorf("failed to create stock: %v", err)
	}

	logger.Logger.Infof("📝 Created new stock: %s - %s", newStock.Symbol, newStock.CompanyName)
	return newStock, nil
}

// getOrCreateHOSEExchange gets or creates HOSE exchange
func getOrCreateHOSEExchange(store repository.DatabaseStore) (*modelsdb.Exchange, error) {
	// Try to get HOSE exchange
	exchange, err := store.GetExchangeByCode("HOSE")
	if err == nil {
		return exchange, nil
	}

	// HOSE doesn't exist, create new one
	newExchange := &modelsdb.Exchange{
		Code:     "HOSE",
		Name:     "Ho Chi Minh Stock Exchange",
		Timezone: "Asia/Ho_Chi_Minh",
	}

	err = store.CreateExchange(newExchange)
	if err != nil {
		return nil, fmt.Errorf("failed to create HOSE exchange: %v", err)
	}

	logger.Logger.Info("📝 Created new HOSE exchange")
	return newExchange, nil
}

// determineSector xác định sector của stock dựa vào symbol
func determineSector(symbol string) string {
	bankingSymbols := map[string]bool{
		"ACB": true, "BID": true, "CTG": true, "HDB": true,
		"MBB": true, "SHB": true, "SSB": true, "STB": true,
		"TCB": true, "TPB": true, "VCB": true, "VPB": true,
	}

	realEstateSymbols := map[string]bool{
		"VHM": true, "VIC": true, "VRE": true, "BCM": true,
	}

	techSymbols := map[string]bool{
		"FPT": true, "VTI": true,
	}

	energySymbols := map[string]bool{
		"GAS": true, "GVR": true, "PLX": true, "POW": true,
	}

	consumerSymbols := map[string]bool{
		"MSN": true, "MWG": true, "SAB": true, "VNM": true,
	}

	switch {
	case bankingSymbols[symbol]:
		return "Banking"
	case realEstateSymbols[symbol]:
		return "Real Estate"
	case techSymbols[symbol]:
		return "Technology"
	case energySymbols[symbol]:
		return "Energy"
	case consumerSymbols[symbol]:
		return "Consumer Goods"
	default:
		return "Others"
	}
}

// logSyncResult logs sync results to database
func logSyncResult(store repository.DatabaseStore, successCount, errorCount int, duration time.Duration, source string) error {
	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		DurationMs:   duration.Milliseconds(),
		Source:       source,
	}

	if errorCount > 0 {
		syncLog.ErrorMessage = fmt.Sprintf("Encountered %d errors during crawling process", errorCount)
	}

	return store.CreateSyncLog(syncLog)
}

// logSyncError logs sync errors to database
func logSyncError(store repository.DatabaseStore, err error, startTime time.Time) {
	duration := time.Since(startTime)
	syncLog := &modelsdb.SyncLog{
		SyncDate:     time.Now(),
		SuccessCount: 0,
		ErrorCount:   1,
		DurationMs:   duration.Milliseconds(),
		Source:       "VietStock",
		ErrorMessage: err.Error(),
	}

	logErr := store.CreateSyncLog(syncLog)
	if logErr != nil {
		logger.Logger.Errorf("❌ Failed to log sync error: %v", logErr)
	}
}
