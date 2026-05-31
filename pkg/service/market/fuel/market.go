// Package fuelmarket registers the Vietnamese fuel price market as an AssetMarket.
// It tracks four regulated petroleum products.
package fuelmarket

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/market"
	"go-stock-prediction/pkg/store/repository"
)

// Market implements market.AssetMarket for Vietnamese retail fuel prices.
type Market struct{}

func (m *Market) MarketKey() string  { return "FUEL" }
func (m *Market) MarketName() string { return "Gia Xang Dau Viet Nam" }

// fuelInstruments defines the four regulated fuel products tracked.
var fuelInstruments = []market.Instrument{
	{ID: "ron95_iii", Name: "Xang RON 95-III"},
	{ID: "e5_ron92", Name: "Xang E5 RON 92-II"},
	{ID: "do_005s", Name: "Dau DO 0,05S-II"},
	{ID: "kerosene", Name: "Dau hoa 2-K"},
}

// GetInstruments returns the four tracked fuel product instruments.
func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
	return fuelInstruments, nil
}

// FetchPrices returns prices (oldest→newest, ASC) for the given fuel product
// over the last `months` months. DB returns DESC order; this method reverses to ASC.
func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
	from := time.Now().AddDate(0, -months, 0)
	prices, err := repository.GetSingleton().GetFuelPricesByDateRange(inst.ID, from, time.Now())
	if err != nil {
		return nil, err
	}
	// Reverse DESC → ASC for algorithm consumption.
	result := make([]float64, len(prices))
	for i, p := range prices {
		f, _ := p.Price.Float64()
		result[len(prices)-1-i] = f
	}
	return result, nil
}

// SavePrediction persists one algorithm's prediction for a fuel product.
func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
	return repository.GetSingleton().CreateFuelPrediction(&modelsdb.FuelPrediction{
		ProductType:    inst.ID,
		AlgorithmName:  pred.AlgorithmName,
		PredictedPrice: decimal.NewFromFloat(pred.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(pred.CurrentPrice),
		Confidence:     decimal.NewFromFloat(pred.Confidence),
		PredictionDate: time.Now(),
		TargetDate:     pred.TargetDate,
	})
}

// Crawl delegates to the fuel crawler.
// Daily crawling is also managed by the crawler_fuel cron job.
func (m *Market) Crawl(ctx context.Context) error {
	return crawler.CronjobFuelCrawler()
}

func init() {
	market.Register(&Market{})
}
