// Package nasdaq100 registers the NASDAQ 100 market as an AssetMarket.
// It tracks a curated list of top US tech stocks from the NASDAQ 100 index.
package nasdaq100

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/market"
	"go-stock-prediction/pkg/store/repository"
)

// Market implements market.AssetMarket for NASDAQ 100 constituent stocks.
type Market struct{}

func (m *Market) MarketKey() string  { return "NASDAQ100" }
func (m *Market) MarketName() string { return "NASDAQ 100 — Top US Tech Stocks" }

// GetInstruments returns all NASDAQ symbols currently present in the DB.
// Falls back to the crawler's static symbol list if the DB is empty.
func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
	symbols, err := repository.GetSingleton().GetNasdaqSymbols()
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		// Fallback: return static list so prediction can run before first crawl.
		staticSymbols := []string{
			"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA",
			"META", "TSLA", "AVGO", "COST", "NFLX",
			"AMD", "ADBE", "QCOM", "INTC", "CSCO",
		}
		instruments := make([]market.Instrument, len(staticSymbols))
		for i, s := range staticSymbols {
			instruments[i] = market.Instrument{ID: s, Name: s}
		}
		return instruments, nil
	}
	instruments := make([]market.Instrument, len(symbols))
	for i, s := range symbols {
		instruments[i] = market.Instrument{ID: s, Name: s}
	}
	return instruments, nil
}

// FetchPrices returns close prices (oldest→newest, ASC) for the given NASDAQ symbol
// over the last `months` months. DB returns DESC order; this method reverses to ASC.
func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
	from := time.Now().AddDate(0, -months, 0)
	prices, err := repository.GetSingleton().GetNasdaqPricesByDateRange(inst.ID, from, time.Now())
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

// SavePrediction persists one algorithm's prediction for a NASDAQ instrument.
func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
	return repository.GetSingleton().CreateNasdaqPrediction(&modelsdb.NasdaqPrediction{
		Symbol:         inst.ID,
		AlgorithmName:  pred.AlgorithmName,
		PredictedPrice: decimal.NewFromFloat(pred.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(pred.CurrentPrice),
		Confidence:     decimal.NewFromFloat(pred.Confidence),
		PredictionDate: time.Now(),
		TargetDate:     pred.TargetDate,
	})
}

// Crawl delegates to the NASDAQ crawler.
// Daily crawling is also managed by the crawler_nasdaq cron job.
func (m *Market) Crawl(ctx context.Context) error {
	return crawler.CronjobNasdaqCrawler()
}

func init() {
	market.Register(&Market{})
}
