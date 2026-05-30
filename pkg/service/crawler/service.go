package crawler

import (
	"context"
	"fmt"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
	"go-stock-prediction/pkg/store/repository"
)

func CrawlerAll() (*CrawlResult, error) {
	return crawler.crawlVN30Data(context.Background())
}

func CrawlerSingleStock(ctx context.Context, symbol string) (*modelssvc.VN30Stock, error) {
	return crawler.crawlSingleStock(ctx, symbol)
}

// CrawlHistoricalAll fetches `days` trading days of price history for every VN30 stock and
// upserts them into the database. Returns (saved, skipped, error).
func CrawlHistoricalAll(ctx context.Context, days int) (int, int, error) {
	return crawler.crawlHistoricalAll(ctx, days)
}

// CrawlAndSaveSingleStock fetches latest price data for a single stock and persists it to the DB.
// Returns the fetched stock data on success.
func CrawlAndSaveSingleStock(ctx context.Context, symbol string) (*modelssvc.VN30Stock, error) {
	stockData, err := crawler.crawlSingleStock(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stock data: %v", err)
	}

	store := repository.GetSingleton()
	if store == nil {
		return nil, fmt.Errorf("database store not available")
	}

	exchange, err := getOrCreateHOSEExchange(store)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange: %v", err)
	}

	if err := processingSingleStock(store, *stockData, exchange.ID); err != nil {
		return nil, fmt.Errorf("failed to save stock data: %v", err)
	}

	return stockData, nil
}
