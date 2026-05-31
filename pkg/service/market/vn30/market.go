// Package vn30 registers the VN30 market as an AssetMarket.
// To add VN100: create pkg/service/market/vn100/market.go following this pattern.
package vn30

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/service/crawler"
	"go-stock-prediction/pkg/service/market"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
)

// Market implements market.AssetMarket for the VN30 index.
type Market struct{}

func (m *Market) MarketKey() string  { return "VN30" }
func (m *Market) MarketName() string { return "VN30 — Top 30 Vietnam stocks by market cap" }

// GetInstruments returns all VN30 stocks as Instruments.
func (m *Market) GetInstruments(ctx context.Context) ([]market.Instrument, error) {
	stocks, err := repository.GetSingleton().GetVN30Stocks()
	if err != nil {
		return nil, err
	}
	result := make([]market.Instrument, len(stocks))
	for i, s := range stocks {
		result[i] = market.Instrument{
			ID:         s.Symbol,
			Name:       s.CompanyName,
			InternalID: s.ID,
		}
	}
	return result, nil
}

// FetchPrices returns close prices (oldest→newest) for the given stock over the last `months` months.
// DB returns prices in DESC order; this method reverses them to ASC for algorithm input.
func (m *Market) FetchPrices(ctx context.Context, inst market.Instrument, months int) ([]float64, error) {
	from := time.Now().AddDate(0, -months, 0)
	prices, err := repository.GetSingleton().GetStockPricesByStockIDAndDateRange(inst.InternalID, from, time.Now())
	if err != nil {
		return nil, err
	}
	// Reverse DESC → ASC for algorithm input
	result := make([]float64, len(prices))
	for i, p := range prices {
		f, _ := p.ClosePrice.Float64()
		result[len(prices)-1-i] = f
	}
	return result, nil
}

// SavePrediction persists one algorithm's prediction for a stock.
func (m *Market) SavePrediction(ctx context.Context, inst market.Instrument, pred market.PredictionData) error {
	return repository.GetSingleton().CreatePrediction(&modelsdb.Prediction{
		StockID:        inst.InternalID,
		PredictedPrice: decimal.NewFromFloat(pred.PredictedPrice),
		CurrentPrice:   decimal.NewFromFloat(pred.CurrentPrice),
		Confidence:     decimal.NewFromFloat(pred.Confidence),
		AlgorithmName:  pred.AlgorithmName,
		PredictionDate: time.Now(),
		TargetDate:     pred.TargetDate,
	})
}

// Crawl delegates to the VN30 stock crawler.
func (m *Market) Crawl(ctx context.Context) error {
	return crawler.CronjobCrawler()
}

// GetStocks returns the underlying stock records for use by the training service.
// This is a convenience method for the predict package that still needs []modelsdb.Stock.
func (m *Market) GetStocks() ([]modelsdb.Stock, error) {
	return repository.GetSingleton().GetVN30Stocks()
}

func init() {
	market.Register(&Market{})
}
