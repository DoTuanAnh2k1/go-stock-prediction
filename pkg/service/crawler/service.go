package crawler

import (
	"context"
	modelssvc "go-stock-prediction/pkg/models/models_svc"
)

func CrawlerAll() (*CrawlResult, error) {
	return crawler.crawlVN30Data(context.Background())
}

func CrawlerSingleStock(ctx context.Context, symbol string) (*modelssvc.VN30Stock, error) {
	return crawler.crawlSingleStock(ctx, symbol)
}
