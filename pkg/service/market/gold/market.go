// Package goldmarket registers the gold market as an AssetMarket.
// It covers XAU/spot (Yahoo Finance USD/oz) and BTMC domestic gold products.
package goldmarket

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/market"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

// Market implements market.AssetMarket for gold (SJC and XAU).
type Market struct{}

var goldInstruments = []market.Instrument{
	{ID: "XAU/spot", Name: "XAU Spot (USD/oz)"},
	{ID: "BTMC/sjc", Name: "Vang SJC (BTMC)"},
	{ID: "BTMC/nhan_tron", Name: "Vang Nhan Tron (BTMC)"},
}

func (m *Market) MarketKey() string  { return "GOLD" }
func (m *Market) MarketName() string { return "Gold — SJC & XAU/USD" }

// GetInstruments returns the 3 tracked gold instruments.
func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
	return goldInstruments, nil
}

// FetchPrices returns buy prices (oldest→newest) for the given gold instrument
// over the last `months` months. DB returns prices in DESC order; this method
// reverses them to ASC for algorithm input.
func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
	source, productType, err := parseGoldID(inst.ID)
	if err != nil {
		return nil, err
	}
	from := time.Now().AddDate(0, -months, 0)
	prices, err := repository.GetSingleton().GetGoldPricesByDateRange(source, productType, from, time.Now())
	if err != nil {
		return nil, err
	}
	// Reverse DESC → ASC for algorithm input
	result := make([]float64, len(prices))
	for i, p := range prices {
		f, _ := p.BuyPrice.Float64()
		result[len(prices)-1-i] = f
	}
	return result, nil
}

// SavePrediction persists one algorithm's prediction for a gold instrument.
func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
	source, productType, err := parseGoldID(inst.ID)
	if err != nil {
		return err
	}
	return repository.GetSingleton().CreateGoldPrediction(&modelsdb.GoldPrediction{
		Source:         source,
		ProductType:    productType,
		PredictedPrice: decimal.NewFromFloat(pred.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(pred.CurrentPrice),
		Confidence:     decimal.NewFromFloat(pred.Confidence),
		AlgorithmName:  pred.AlgorithmName,
		PredictionDate: time.Now(),
		TargetDate:     pred.TargetDate,
	})
}

// Crawl delegates to the gold crawler. Gold crawling is also handled by the
// Daily10AM cron job; this method enables on-demand crawling via the orchestrator.
func (m *Market) Crawl(ctx context.Context) error {
	return crawler.CronjobGoldCrawler()
}

// parseGoldID parses an instrument ID of the form "SOURCE/PRODUCT_TYPE".
func parseGoldID(id string) (source, productType string, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid gold instrument ID %q (expected SOURCE/PRODUCT_TYPE)", id)
	}
	return parts[0], parts[1], nil
}

func init() {
	market.Register(&Market{})
}
