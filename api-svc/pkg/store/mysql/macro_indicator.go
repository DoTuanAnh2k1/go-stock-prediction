package mysql

import (
	"context"
	modelsdb "go-stock-prediction/pkg/models/models_db"
)

// GetMacroIndicators returns macro indicators filtered by name, ordered by indicator_date DESC.
func (c *Client) GetMacroIndicators(_ context.Context, name string, limit int) ([]modelsdb.MacroIndicator, error) {
	var indicators []modelsdb.MacroIndicator
	query := c.Db.Order("indicator_date DESC")
	if name != "" {
		query = query.Where("indicator_name = ?", name)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&indicators).Error
	return indicators, err
}

// GetLatestMacroIndicator returns the most recent macro indicator for a given name.
func (c *Client) GetLatestMacroIndicator(_ context.Context, name string) (*modelsdb.MacroIndicator, error) {
	var indicator modelsdb.MacroIndicator
	err := c.Db.Where("indicator_name = ?", name).Order("indicator_date DESC").First(&indicator).Error
	if err != nil {
		return nil, err
	}
	return &indicator, nil
}

// UpsertMacroIndicator creates or updates a macro indicator record.
func (c *Client) UpsertMacroIndicator(_ context.Context, indicator *modelsdb.MacroIndicator) error {
	return c.Db.Where("indicator_name = ? AND indicator_date = ?",
		indicator.IndicatorName, indicator.IndicatorDate).
		Assign(indicator).
		FirstOrCreate(indicator).Error
}
