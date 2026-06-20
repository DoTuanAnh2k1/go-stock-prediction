package server

import (
	"context"
	"sync"
	"time"

	"go-stock-prediction/pkg/logger"
	modelsdb "go-stock-prediction/pkg/models/models_db"
	"go-stock-prediction/pkg/store/repository"
	"go-stock-prediction/pkg/utils/cron"
)

// backupJobKey is the cron_schedules row that drives the scheduled database
// backup. The schedule is owned by the API service (this scheduler) and remains
// editable from the Settings page — the watcher below polls the DB for changes.
const backupJobKey = "daily_backup"

// backupDefaultCron is the default schedule (6-field, with seconds): 3AM daily.
const backupDefaultCron = "0 0 3 * * *"

// backupScheduler runs the database backup on the schedule stored in the
// cron_schedules table and reschedules itself when that row changes, without a
// restart. It mirrors the polling behaviour of the Python scheduler but owns
// only the backup job.
type backupScheduler struct {
	store   repository.DatabaseStore
	cm      *cron.CronManager
	mu      sync.Mutex
	hasJob  bool
	curCron string
	stopCh  chan struct{}
}

var backupSched *backupScheduler

// StartBackupScheduler seeds the daily_backup schedule if absent, registers the
// backup cron job, and starts a goroutine that polls the DB every 60s so edits
// made via the Settings page take effect without a restart.
func StartBackupScheduler(store repository.DatabaseStore) {
	if store == nil {
		logger.Logger.Errorf("backup scheduler: nil store, not starting")
		return
	}

	bs := &backupScheduler{
		store:  store,
		cm:     cron.NewCronManager(),
		stopCh: make(chan struct{}),
	}
	backupSched = bs

	bs.seedDefault()
	bs.sync() // initial registration based on current DB state

	if err := bs.cm.Start(); err != nil {
		logger.Logger.Errorf("backup scheduler: failed to start cron: %v", err)
		return
	}
	logger.Logger.Infof("✅ Backup scheduler started (job=%s)", backupJobKey)

	go bs.watch()
}

// StopBackupScheduler halts the backup cron scheduler.
func StopBackupScheduler() {
	if backupSched == nil {
		return
	}
	close(backupSched.stopCh)
	backupSched.cm.Stop()
}

// seedDefault inserts the daily_backup row if it does not already exist
// (insert-if-not-exists — never overwrites a user-edited schedule).
func (bs *backupScheduler) seedDefault() {
	if existing, err := bs.store.GetCronScheduleByKey(backupJobKey); err == nil && existing != nil {
		return
	}
	row := &modelsdb.CronSchedule{
		JobKey:         backupJobKey,
		JobName:        "Backup database (3AM hàng ngày)",
		CronExpression: backupDefaultCron,
		Enabled:        true,
	}
	if err := bs.store.UpsertCronSchedule(row); err != nil {
		logger.Logger.Errorf("backup scheduler: failed to seed default schedule: %v", err)
	}
}

// sync reconciles the running cron job with the daily_backup row in the DB.
func (bs *backupScheduler) sync() {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	sched, err := bs.store.GetCronScheduleByKey(backupJobKey)
	if err != nil || sched == nil {
		return
	}

	// Disabled → ensure no job is registered.
	if !sched.Enabled {
		if bs.hasJob {
			_ = bs.cm.RemoveJob(backupJobKey)
			bs.hasJob = false
			bs.curCron = ""
			logger.Logger.Infof("backup scheduler: job disabled")
		}
		return
	}

	// Enabled and not yet registered → add.
	if !bs.hasJob {
		if err := bs.cm.AddJob(backupJobKey, sched.CronExpression, bs.runJob); err != nil {
			logger.Logger.Errorf("backup scheduler: add job failed (%s): %v", sched.CronExpression, err)
			return
		}
		bs.hasJob = true
		bs.curCron = sched.CronExpression
		logger.Logger.Infof("backup scheduler: job registered (cron=%s)", sched.CronExpression)
		return
	}

	// Enabled and schedule changed → reschedule.
	if sched.CronExpression != bs.curCron {
		if err := bs.cm.RescheduleJob(backupJobKey, sched.CronExpression); err != nil {
			logger.Logger.Errorf("backup scheduler: reschedule failed (%s): %v", sched.CronExpression, err)
			return
		}
		bs.curCron = sched.CronExpression
		logger.Logger.Infof("backup scheduler: rescheduled (cron=%s)", sched.CronExpression)
	}
}

// runJob is the cron callback that performs the backup.
func (bs *backupScheduler) runJob() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, _, err := runBackup(ctx); err != nil {
		logger.Logger.Errorf("backup scheduler: scheduled backup failed: %v", err)
		return err
	}
	return nil
}

// watch polls the DB every 60s for schedule changes and reconciles.
func (bs *backupScheduler) watch() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-bs.stopCh:
			return
		case <-ticker.C:
			bs.sync()
		}
	}
}
