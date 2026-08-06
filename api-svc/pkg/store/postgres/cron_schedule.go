package postgres

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

func (c *Client) GetAllCronSchedules(ctx context.Context) ([]modelsdb.CronSchedule, error) {
	var schedules []modelsdb.CronSchedule
	err := c.db(ctx).Order("job_key ASC").Find(&schedules).Error
	return schedules, err
}

func (c *Client) GetCronScheduleByKey(ctx context.Context, jobKey string) (*modelsdb.CronSchedule, error) {
	var schedule modelsdb.CronSchedule
	err := c.db(ctx).Where("job_key = ?", jobKey).First(&schedule).Error
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

func (c *Client) UpsertCronSchedule(ctx context.Context, s *modelsdb.CronSchedule) error {
	return c.db(ctx).Save(s).Error
}
