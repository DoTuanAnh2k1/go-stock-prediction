package crawler

import (
	"go-stock-prediction/pkg/utils/cron"
)

func Init() {
	crawler = NewVietStockCrawler()
	cron.AddJob("Crawler stock", cron.Daily12PM, CronjobCrawler)
}
