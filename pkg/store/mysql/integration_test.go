//go:build integration

package mysql

import (
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newSQLiteClient creates a Client backed by an in-memory SQLite DB.
func newSQLiteClient(t *testing.T) *Client {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open SQLite: %v", err)
	}
	err = db.AutoMigrate(modelsdb.AllModels...)
	if err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return &Client{Db: db}
}

// ---- Exchange CRUD ----

func TestIntegration_Exchange_CreateAndGet(t *testing.T) {
	c := newSQLiteClient(t)
	ex := &modelsdb.Exchange{Code: "HOSE", Name: "Ho Chi Minh Stock Exchange"}
	if err := c.CreateExchange(ex); err != nil {
		t.Fatalf("CreateExchange: %v", err)
	}
	got, err := c.GetExchangeByCode("HOSE")
	if err != nil {
		t.Fatalf("GetExchangeByCode: %v", err)
	}
	if got.Code != "HOSE" {
		t.Errorf("code = %v, want HOSE", got.Code)
	}
}

func TestIntegration_Exchange_GetByID(t *testing.T) {
	c := newSQLiteClient(t)
	ex := &modelsdb.Exchange{Code: "HNX", Name: "Hanoi Stock Exchange"}
	_ = c.CreateExchange(ex)
	got, err := c.GetExchangeByID(ex.ID)
	if err != nil {
		t.Fatalf("GetExchangeByID: %v", err)
	}
	if got.Code != "HNX" {
		t.Errorf("code = %v, want HNX", got.Code)
	}
}

func TestIntegration_Exchange_GetAll(t *testing.T) {
	c := newSQLiteClient(t)
	_ = c.CreateExchange(&modelsdb.Exchange{Code: "HOSE", Name: "HOSE"})
	_ = c.CreateExchange(&modelsdb.Exchange{Code: "HNX", Name: "HNX"})
	exchanges, err := c.GetAllExchanges()
	if err != nil {
		t.Fatalf("GetAllExchanges: %v", err)
	}
	if len(exchanges) < 2 {
		t.Errorf("expected >= 2 exchanges, got %d", len(exchanges))
	}
}

func TestIntegration_Exchange_Update(t *testing.T) {
	c := newSQLiteClient(t)
	ex := &modelsdb.Exchange{Code: "TEST", Name: "Test Exchange"}
	_ = c.CreateExchange(ex)
	ex.Name = "Updated Exchange"
	if err := c.UpdateExchange(ex); err != nil {
		t.Fatalf("UpdateExchange: %v", err)
	}
	got, _ := c.GetExchangeByID(ex.ID)
	if got.Name != "Updated Exchange" {
		t.Errorf("name = %v, want 'Updated Exchange'", got.Name)
	}
}

func TestIntegration_Exchange_Delete(t *testing.T) {
	c := newSQLiteClient(t)
	ex := &modelsdb.Exchange{Code: "DEL", Name: "Delete Me"}
	_ = c.CreateExchange(ex)
	if err := c.DeleteExchange(ex.ID); err != nil {
		t.Fatalf("DeleteExchange: %v", err)
	}
	// After soft delete, GetExchangeByCode should return error
	_, err := c.GetExchangeByCode("DEL")
	if err == nil {
		t.Error("expected error after deleting exchange, got nil")
	}
}

// ---- Stock CRUD ----

func TestIntegration_Stock_CreateAndGetBySymbol(t *testing.T) {
	c := newSQLiteClient(t)
	stock := &modelsdb.Stock{Symbol: "VIC", CompanyName: "Vingroup", ExchangeID: 1}
	if err := c.CreateStock(stock); err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	got, err := c.GetStockBySymbol("VIC")
	if err != nil {
		t.Fatalf("GetStockBySymbol: %v", err)
	}
	if got.Symbol != "VIC" {
		t.Errorf("symbol = %v, want VIC", got.Symbol)
	}
}

func TestIntegration_Stock_GetByID(t *testing.T) {
	c := newSQLiteClient(t)
	stock := &modelsdb.Stock{Symbol: "VCB", CompanyName: "Vietcombank", ExchangeID: 1}
	_ = c.CreateStock(stock)
	got, err := c.GetStockByID(stock.ID)
	if err != nil {
		t.Fatalf("GetStockByID: %v", err)
	}
	if got.Symbol != "VCB" {
		t.Errorf("symbol = %v, want VCB", got.Symbol)
	}
}

func TestIntegration_Stock_GetAll(t *testing.T) {
	c := newSQLiteClient(t)
	_ = c.CreateStock(&modelsdb.Stock{Symbol: "FPT", CompanyName: "FPT Corp", ExchangeID: 1})
	stocks, err := c.GetAllStocks()
	if err != nil {
		t.Fatalf("GetAllStocks: %v", err)
	}
	if len(stocks) == 0 {
		t.Error("expected at least 1 stock")
	}
}

func TestIntegration_Stock_BulkCreate(t *testing.T) {
	c := newSQLiteClient(t)
	stocks := []modelsdb.Stock{
		{Symbol: "AA1", CompanyName: "Company A1", ExchangeID: 1},
		{Symbol: "BB2", CompanyName: "Company B2", ExchangeID: 1},
	}
	if err := c.BulkCreateStocks(stocks); err != nil {
		t.Fatalf("BulkCreateStocks: %v", err)
	}
	all, _ := c.GetAllStocks()
	if len(all) < 2 {
		t.Errorf("expected >= 2 stocks, got %d", len(all))
	}
}

func TestIntegration_Stock_GetVN30(t *testing.T) {
	c := newSQLiteClient(t)
	_ = c.CreateStock(&modelsdb.Stock{Symbol: "VN1", CompanyName: "VN30 Stock", ExchangeID: 1, IsVN30: true})
	_ = c.CreateStock(&modelsdb.Stock{Symbol: "OT1", CompanyName: "Other Stock", ExchangeID: 1, IsVN30: false})
	vn30, err := c.GetVN30Stocks()
	if err != nil {
		t.Fatalf("GetVN30Stocks: %v", err)
	}
	for _, s := range vn30 {
		if !s.IsVN30 {
			t.Errorf("stock %v has IsVN30=false in VN30 result", s.Symbol)
		}
	}
}

// ---- StockPrice CRUD ----

func TestIntegration_StockPrice_CreateAndGetByStockID(t *testing.T) {
	c := newSQLiteClient(t)
	price := &modelsdb.StockPrice{
		StockID:     1,
		TradingDate: time.Now().Truncate(24 * time.Hour),
		OpenPrice:   decimal.NewFromFloat(100.0),
		HighPrice:   decimal.NewFromFloat(105.0),
		LowPrice:    decimal.NewFromFloat(99.0),
		ClosePrice:  decimal.NewFromFloat(103.0),
		Volume:      100000,
	}
	if err := c.CreateStockPrice(price); err != nil {
		t.Fatalf("CreateStockPrice: %v", err)
	}
	prices, err := c.GetStockPricesByStockID(1)
	if err != nil {
		t.Fatalf("GetStockPricesByStockID: %v", err)
	}
	if len(prices) == 0 {
		t.Error("expected at least 1 price")
	}
}

func TestIntegration_StockPrice_GetLatestByStockID(t *testing.T) {
	c := newSQLiteClient(t)
	now := time.Now()
	_ = c.CreateStockPrice(&modelsdb.StockPrice{
		StockID:     2,
		TradingDate: now.AddDate(0, 0, -1),
		OpenPrice:   decimal.NewFromFloat(90.0),
		HighPrice:   decimal.NewFromFloat(92.0),
		LowPrice:    decimal.NewFromFloat(89.0),
		ClosePrice:  decimal.NewFromFloat(91.0),
		Volume:      50000,
	})
	_ = c.CreateStockPrice(&modelsdb.StockPrice{
		StockID:     2,
		TradingDate: now,
		OpenPrice:   decimal.NewFromFloat(95.0),
		HighPrice:   decimal.NewFromFloat(97.0),
		LowPrice:    decimal.NewFromFloat(94.0),
		ClosePrice:  decimal.NewFromFloat(96.0),
		Volume:      60000,
	})
	latest, err := c.GetLatestStockPriceByStockID(2)
	if err != nil {
		t.Fatalf("GetLatestStockPriceByStockID: %v", err)
	}
	if latest.ClosePrice.LessThan(decimal.NewFromFloat(95.0)) {
		t.Errorf("latest close price = %v, expected >= 95", latest.ClosePrice)
	}
}

func TestIntegration_StockPrice_GetByDateRange(t *testing.T) {
	c := newSQLiteClient(t)
	now := time.Now()
	for i := 0; i < 5; i++ {
		_ = c.CreateStockPrice(&modelsdb.StockPrice{
			StockID:     3,
			TradingDate: now.AddDate(0, 0, -i),
			OpenPrice:   decimal.NewFromFloat(100.0),
			HighPrice:   decimal.NewFromFloat(102.0),
			LowPrice:    decimal.NewFromFloat(98.0),
			ClosePrice:  decimal.NewFromFloat(101.0),
			Volume:      10000,
		})
	}
	from := now.AddDate(0, 0, -3)
	prices, err := c.GetStockPricesByStockIDAndDateRange(3, from, now)
	if err != nil {
		t.Fatalf("GetStockPricesByStockIDAndDateRange: %v", err)
	}
	if len(prices) == 0 {
		t.Error("expected at least 1 price in date range")
	}
}

func TestIntegration_StockPrice_CountAndTruncate(t *testing.T) {
	c := newSQLiteClient(t)
	_ = c.CreateStockPrice(&modelsdb.StockPrice{
		StockID:     4,
		TradingDate: time.Now(),
		OpenPrice:   decimal.NewFromFloat(100.0),
		HighPrice:   decimal.NewFromFloat(102.0),
		LowPrice:    decimal.NewFromFloat(98.0),
		ClosePrice:  decimal.NewFromFloat(101.0),
		Volume:      10000,
	})
	count, err := c.CountStockPrices()
	if err != nil {
		t.Fatalf("CountStockPrices: %v", err)
	}
	if count == 0 {
		t.Error("expected count > 0")
	}
}

// ---- Prediction CRUD ----

func TestIntegration_Prediction_CreateAndGetByStockID(t *testing.T) {
	c := newSQLiteClient(t)
	pred := &modelsdb.Prediction{
		StockID:        1,
		PredictedPrice: decimal.NewFromFloat(110.0),
		CurrentPrice:   decimal.NewFromFloat(100.0),
		Confidence:     decimal.NewFromFloat(0.75),
		AlgorithmName:  "moving_average",
		PredictionDate: time.Now(),
		TargetDate:     time.Now().Add(24 * time.Hour),
	}
	if err := c.CreatePrediction(pred); err != nil {
		t.Fatalf("CreatePrediction: %v", err)
	}
	preds, err := c.GetPredictionsByStockID(1)
	if err != nil {
		t.Fatalf("GetPredictionsByStockID: %v", err)
	}
	if len(preds) == 0 {
		t.Error("expected at least 1 prediction")
	}
}

func TestIntegration_Prediction_GetLatestByStockID(t *testing.T) {
	c := newSQLiteClient(t)
	for i := 0; i < 3; i++ {
		_ = c.CreatePrediction(&modelsdb.Prediction{
			StockID:        5,
			PredictedPrice: decimal.NewFromFloat(float64(100 + i)),
			CurrentPrice:   decimal.NewFromFloat(100.0),
			AlgorithmName:  "arima_garch",
			PredictionDate: time.Now(),
			TargetDate:     time.Now().Add(24 * time.Hour),
		})
	}
	preds, err := c.GetLatestPredictionsByStockID(5, 2)
	if err != nil {
		t.Fatalf("GetLatestPredictionsByStockID: %v", err)
	}
	if len(preds) > 2 {
		t.Errorf("expected <= 2 predictions with limit=2, got %d", len(preds))
	}
}

func TestIntegration_Prediction_GetByDateRange(t *testing.T) {
	c := newSQLiteClient(t)
	now := time.Now()
	_ = c.CreatePrediction(&modelsdb.Prediction{
		StockID:        6,
		PredictedPrice: decimal.NewFromFloat(105.0),
		CurrentPrice:   decimal.NewFromFloat(100.0),
		AlgorithmName:  "lstm_nn",
		PredictionDate: now,
		TargetDate:     now.Add(24 * time.Hour),
	})
	preds, err := c.GetPredictionsByDateRange(now.Add(-1*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetPredictionsByDateRange: %v", err)
	}
	if len(preds) == 0 {
		t.Error("expected at least 1 prediction in date range")
	}
}

func TestIntegration_Prediction_CountPredictions(t *testing.T) {
	c := newSQLiteClient(t)
	_ = c.CreatePrediction(&modelsdb.Prediction{
		StockID:        7,
		PredictedPrice: decimal.NewFromFloat(100.0),
		CurrentPrice:   decimal.NewFromFloat(100.0),
		AlgorithmName:  "moving_average",
		PredictionDate: time.Now(),
		TargetDate:     time.Now().Add(24 * time.Hour),
	})
	count, err := c.CountPredictions()
	if err != nil {
		t.Fatalf("CountPredictions: %v", err)
	}
	if count == 0 {
		t.Error("expected count > 0")
	}
}

// ---- Utility ----

func TestIntegration_CountStocks(t *testing.T) {
	c := newSQLiteClient(t)
	_ = c.CreateStock(&modelsdb.Stock{Symbol: "CNT", CompanyName: "Count Test", ExchangeID: 1})
	count, err := c.CountStocks()
	if err != nil {
		t.Fatalf("CountStocks: %v", err)
	}
	if count == 0 {
		t.Error("expected count > 0")
	}
}

func TestIntegration_Ping(t *testing.T) {
	c := newSQLiteClient(t)
	// Ping uses sql.DB.Ping() which works on SQLite
	if err := c.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}
