package testutil

import (
	"encoding/json"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/shopspring/decimal"
)

// AllFixtures holds all fixture data loaded from testdata/ JSON files.
type AllFixtures struct {
	Exchanges   []modelsdb.Exchange   `json:"exchanges"`
	Stocks      []modelsdb.Stock      `json:"stocks"`
	StockPrices []modelsdb.StockPrice `json:"stock_prices"`
	Predictions []modelsdb.Prediction `json:"predictions"`
	GoldPrices  []modelsdb.GoldPrice  `json:"gold_prices"`
}

// Raw JSON structs for fixtures with string dates/decimals
type rawStockPrice struct {
	StockID       uint   `json:"stock_id"`
	TradingDate   string `json:"trading_date"`
	OpenPrice     string `json:"open_price"`
	HighPrice     string `json:"high_price"`
	LowPrice      string `json:"low_price"`
	ClosePrice    string `json:"close_price"`
	Volume        int64  `json:"volume"`
	Value         string `json:"value"`
	Change        string `json:"change"`
	ChangePercent string `json:"change_percent"`
}

type rawPrediction struct {
	StockID        uint   `json:"stock_id"`
	PredictedPrice string `json:"predicted_price"`
	CurrentPrice   string `json:"current_price"`
	Confidence     string `json:"confidence"`
	AlgorithmName  string `json:"algorithm_name"`
	PredictionDate string `json:"prediction_date"`
	TargetDate     string `json:"target_date"`
}

type rawGoldPrice struct {
	Source      string `json:"source"`
	ProductType string `json:"product_type"`
	TradingDate string `json:"trading_date"`
	BuyPrice    string `json:"buy_price"`
	SellPrice   string `json:"sell_price"`
	Currency    string `json:"currency"`
}

func parseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic("invalid fixture date: " + s)
	}
	return t
}

func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("invalid fixture decimal: " + s)
	}
	return d
}

// testdataDir returns the absolute path to the testdata/ directory
// by walking up from this source file's location.
func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	// pkg/testutil/fixtures.go → project root
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	return filepath.Join(projectRoot, "testdata")
}

// LoadFixtures loads a single JSON fixture file into the target slice.
// target must be a pointer to a slice (e.g., &[]modelsdb.Stock{}).
func LoadFixtures(filename string, target interface{}) error {
	path := filepath.Join(testdataDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// LoadAllFixtures loads all fixture files and returns them as AllFixtures.
// Handles date parsing (YYYY-MM-DD) and decimal conversion from string fields.
func LoadAllFixtures() (*AllFixtures, error) {
	f := &AllFixtures{}

	if err := LoadFixtures("exchanges.json", &f.Exchanges); err != nil {
		return nil, err
	}
	if err := LoadFixtures("stocks.json", &f.Stocks); err != nil {
		return nil, err
	}

	// Stock prices: parse from raw format
	var rawPrices []rawStockPrice
	if err := LoadFixtures("stock_prices.json", &rawPrices); err != nil {
		return nil, err
	}
	for _, r := range rawPrices {
		f.StockPrices = append(f.StockPrices, modelsdb.StockPrice{
			StockID:       r.StockID,
			TradingDate:   parseDate(r.TradingDate),
			OpenPrice:     dec(r.OpenPrice),
			HighPrice:     dec(r.HighPrice),
			LowPrice:      dec(r.LowPrice),
			ClosePrice:    dec(r.ClosePrice),
			Volume:        r.Volume,
			Value:         dec(r.Value),
			Change:        dec(r.Change),
			ChangePercent: dec(r.ChangePercent),
		})
	}

	// Predictions: parse from raw format
	var rawPreds []rawPrediction
	if err := LoadFixtures("predictions.json", &rawPreds); err != nil {
		return nil, err
	}
	for _, r := range rawPreds {
		f.Predictions = append(f.Predictions, modelsdb.Prediction{
			StockID:        r.StockID,
			PredictedPrice: dec(r.PredictedPrice),
			CurrentPrice:   dec(r.CurrentPrice),
			Confidence:     dec(r.Confidence),
			AlgorithmName:  r.AlgorithmName,
			PredictionDate: parseDate(r.PredictionDate),
			TargetDate:     parseDate(r.TargetDate),
		})
	}

	// Gold prices: parse from raw format
	var rawGold []rawGoldPrice
	if err := LoadFixtures("gold_prices.json", &rawGold); err != nil {
		return nil, err
	}
	for _, r := range rawGold {
		f.GoldPrices = append(f.GoldPrices, modelsdb.GoldPrice{
			Source:      r.Source,
			ProductType: r.ProductType,
			TradingDate: parseDate(r.TradingDate),
			BuyPrice:    dec(r.BuyPrice),
			SellPrice:   dec(r.SellPrice),
			Currency:    r.Currency,
		})
	}

	return f, nil
}
