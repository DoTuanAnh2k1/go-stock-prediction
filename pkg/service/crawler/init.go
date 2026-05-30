package crawler

import (
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/utils/cron"
)

func Init() {
	crawler = NewVietStockCrawler()
	cron.AddJob("Crawler stock", cron.Daily12PM, CronjobCrawler)

	err := cron.AddJob("Crawler gold", cron.Daily10AM, CronjobGoldCrawler)
	if err != nil {
		logger.Logger.Errorf("Failed to add gold crawler cronjob: %v", err)
	} else {
		logger.Logger.Info("Gold crawler cronjob registered - every day 10 AM")
	}

	// Backfill 6 months of XAU/USD history on startup (non-blocking).
	go func() {
		logger.Logger.Info("Starting XAU history backfill in background")
		ImportXAUHistory()
	}()

	// Backfill 30 days of vang.today history on startup (non-blocking).
	go func() {
		logger.Logger.Info("Starting vang.today history backfill in background")
		ImportVangTodayHistory()
	}()
}
