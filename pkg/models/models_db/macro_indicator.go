package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
)

type MacroIndicator struct {
	ID            uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	IndicatorName string          `gorm:"size:30;not null;uniqueIndex:idx_macro_name_date,priority:1" json:"indicator_name"`
	IndicatorDate time.Time       `gorm:"not null;uniqueIndex:idx_macro_name_date,priority:2" json:"indicator_date"`
	Value         decimal.Decimal `gorm:"type:decimal(20,6);not null" json:"value"`
	Source        string          `gorm:"size:50" json:"source"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func (MacroIndicator) TableName() string {
	return "macro_indicators"
}
