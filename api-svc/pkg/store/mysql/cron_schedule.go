package mysql

import modelsdb "go-stock-prediction/pkg/models/models_db"

func (c *Client) GetAllCronSchedules() ([]modelsdb.CronSchedule, error) {
	var schedules []modelsdb.CronSchedule
	err := c.Db.Order("job_key ASC").Find(&schedules).Error
	return schedules, err
}

func (c *Client) GetCronScheduleByKey(jobKey string) (*modelsdb.CronSchedule, error) {
	var schedule modelsdb.CronSchedule
	err := c.Db.Where("job_key = ?", jobKey).First(&schedule).Error
	if err != nil {
		return nil, err
	}
	return &schedule, nil
}

func (c *Client) UpsertCronSchedule(s *modelsdb.CronSchedule) error {
	return c.Db.Save(s).Error
}
