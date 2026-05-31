package crawler

import (
	"time"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"go-stock-prediction/pkg/utils/cron"
)

// Default schedules — used to seed DB on first run.
var crawlerDefaults = []modelsdb.CronSchedule{
	{JobKey: "crawler_stock", JobName: "Crawler Stock (VN30)", CronExpression: cron.Daily12PM, Enabled: true},
	{JobKey: "crawler_gold", JobName: "Crawler Gold (SJC/XAU)", CronExpression: cron.Daily10AM, Enabled: true},
}

func Init() {
	crawler = NewVietStockCrawler()

	store := repository.GetSingleton()

	// Seed defaults and register cron jobs
	for _, def := range crawlerDefaults {
		s := loadOrSeedCrawlerSchedule(store, def)
		jobFn := getCrawlerJobFn(s.JobKey)
		if s.Enabled {
			if err := cron.AddJob(s.JobKey, s.CronExpression, jobFn); err != nil {
				logger.Logger.Errorf("Failed to add cron job %s: %v", s.JobKey, err)
			} else {
				logger.Logger.Infof("Registered cron job %s: %s", s.JobKey, s.CronExpression)
			}
		} else {
			logger.Logger.Infof("Cron job %s is disabled", s.JobKey)
		}
	}

	// Watch for schedule changes
	go watchCrawlerSchedules(store)

	// Backfill on startup (non-blocking)
	go func() {
		logger.Logger.Info("Starting XAU history backfill in background")
		ImportXAUHistory()
	}()
	go func() {
		logger.Logger.Info("Starting vang.today history backfill in background")
		ImportVangTodayHistory()
	}()
}

// loadOrSeedCrawlerSchedule gets the schedule from DB, or seeds it from default if not present.
func loadOrSeedCrawlerSchedule(store repository.DatabaseStore, def modelsdb.CronSchedule) modelsdb.CronSchedule {
	s, err := store.GetCronScheduleByKey(def.JobKey)
	if err != nil {
		// Not in DB yet — seed default
		if err2 := store.UpsertCronSchedule(&def); err2 != nil {
			logger.Logger.Errorf("Failed to seed schedule %s: %v", def.JobKey, err2)
		}
		return def
	}
	return *s
}

func getCrawlerJobFn(jobKey string) cron.JobFunc {
	switch jobKey {
	case "crawler_gold":
		return CronjobGoldCrawler
	default:
		return CronjobCrawler
	}
}

// watchCrawlerSchedules checks DB every minute for schedule changes and reschedules jobs.
func watchCrawlerSchedules(store repository.DatabaseStore) {
	// Track current state: jobKey → {expr, enabled}
	type state struct {
		expr    string
		enabled bool
	}
	current := make(map[string]state)

	// Initialize with what's running
	for _, def := range crawlerDefaults {
		s, err := store.GetCronScheduleByKey(def.JobKey)
		if err != nil {
			current[def.JobKey] = state{expr: def.CronExpression, enabled: def.Enabled}
		} else {
			current[def.JobKey] = state{expr: s.CronExpression, enabled: s.Enabled}
		}
	}

	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		for _, def := range crawlerDefaults {
			s, err := store.GetCronScheduleByKey(def.JobKey)
			if err != nil {
				continue
			}
			prev := current[def.JobKey]
			if s.CronExpression == prev.expr && s.Enabled == prev.enabled {
				continue
			}
			// Something changed — apply
			jobFn := getCrawlerJobFn(def.JobKey)
			if s.Enabled {
				if prev.enabled {
					if err := cron.RescheduleJob(def.JobKey, s.CronExpression); err != nil {
						logger.Logger.Errorf("Failed to reschedule %s: %v", def.JobKey, err)
						continue
					}
				} else {
					if err := cron.EnableJob(def.JobKey, s.CronExpression, jobFn); err != nil {
						logger.Logger.Errorf("Failed to enable %s: %v", def.JobKey, err)
						continue
					}
				}
				logger.Logger.Infof("Rescheduled %s → %s", def.JobKey, s.CronExpression)
			} else if prev.enabled {
				cron.DisableJob(def.JobKey)
				logger.Logger.Infof("Disabled cron job %s", def.JobKey)
			}
			current[def.JobKey] = state{expr: s.CronExpression, enabled: s.Enabled}
		}
	}
}
