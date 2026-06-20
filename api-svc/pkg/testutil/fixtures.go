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
	GoldPrices []modelsdb.GoldPrice `json:"gold_prices"`
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
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	return filepath.Join(projectRoot, "testdata")
}

// LoadFixtures loads a single JSON fixture file into the target slice.
func LoadFixtures(filename string, target interface{}) error {
	path := filepath.Join(testdataDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// LoadAllFixtures loads fixture files and returns them as AllFixtures.
func LoadAllFixtures() (*AllFixtures, error) {
	f := &AllFixtures{}

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
