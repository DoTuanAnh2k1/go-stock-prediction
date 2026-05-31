// Package cryptomarket registers the Cryptocurrency market as an AssetMarket.
// It tracks Bitcoin (BTC) and Ethereum (ETH) via CoinGecko.
package cryptomarket

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/market"
	"go-stock-prediction/pkg/store/repository"
)

// Market implements market.AssetMarket for BTC and ETH.
type Market struct{}

func (m *Market) MarketKey() string  { return "CRYPTO" }
func (m *Market) MarketName() string { return "Cryptocurrency — BTC & ETH" }

// cryptoInstruments defines the tracked crypto assets.
// ID matches the CoinGecko coin_id for consistent lookups.
var cryptoInstruments = []market.Instrument{
	{ID: "bitcoin", Name: "Bitcoin (BTC)"},
	{ID: "ethereum", Name: "Ethereum (ETH)"},
}

// GetInstruments returns the two tracked cryptocurrency instruments.
func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
	return cryptoInstruments, nil
}

// FetchPrices returns close prices (oldest→newest, ASC) for the given coin
// over the last `months` months. DB returns DESC order; this method reverses to ASC.
func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
	from := time.Now().AddDate(0, -months, 0)
	prices, err := repository.GetSingleton().GetCryptoPricesByDateRange(inst.ID, from, time.Now())
	if err != nil {
		return nil, err
	}
	// Reverse DESC → ASC for algorithm consumption.
	result := make([]float64, len(prices))
	for i, p := range prices {
		f, _ := p.ClosePrice.Float64()
		result[len(prices)-1-i] = f
	}
	return result, nil
}

// SavePrediction persists one algorithm's prediction for a crypto instrument.
func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
	// Resolve symbol from the static instrument list.
	symbol := inst.ID
	for _, ci := range cryptoInstruments {
		if ci.ID == inst.ID {
			// Extract symbol from Name: "Bitcoin (BTC)" → "BTC"
			name := ci.Name
			start := len(name) - 4 // "(BTC" starts 4 chars before ")"
			if start > 0 && name[start] == '(' && name[len(name)-1] == ')' {
				symbol = name[start+1 : len(name)-1]
			}
			break
		}
	}

	return repository.GetSingleton().CreateCryptoPrediction(&modelsdb.CryptoPrediction{
		CoinID:         inst.ID,
		Symbol:         symbol,
		AlgorithmName:  pred.AlgorithmName,
		PredictedPrice: decimal.NewFromFloat(pred.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(pred.CurrentPrice),
		Confidence:     decimal.NewFromFloat(pred.Confidence),
		PredictionDate: time.Now(),
		TargetDate:     pred.TargetDate,
	})
}

// Crawl delegates to the crypto crawler.
// Periodic crawling is also managed by the crawler_crypto cron job.
func (m *Market) Crawl(ctx context.Context) error {
	return crawler.CronjobCryptoCrawler()
}

func init() {
	market.Register(&Market{})
}
