package modelsdb

import "time"

// CronSchedule persists per-job cron configuration.
type CronSchedule struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	JobKey         string    `gorm:"uniqueIndex;size:100;not null"`
	JobName        string    `gorm:"size:200;not null"`
	CronExpression string    `gorm:"size:100;not null"`
	Enabled        bool      `gorm:"default:true"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}
