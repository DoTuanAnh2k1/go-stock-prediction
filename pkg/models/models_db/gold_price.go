package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type GoldPrice struct {
	ID          uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	Source      string          `gorm:"size:10;not null;uniqueIndex:idx_gold_source_product_date,priority:1" json:"source"`
	ProductType string          `gorm:"size:20;not null;uniqueIndex:idx_gold_source_product_date,priority:2" json:"product_type"`
	TradingDate time.Time       `gorm:"not null;uniqueIndex:idx_gold_source_product_date,priority:3" json:"trading_date"`
	BuyPrice    decimal.Decimal `gorm:"type:decimal(15,2)" json:"buy_price"`
	SellPrice   decimal.Decimal `gorm:"type:decimal(15,2)" json:"sell_price"`
	Currency    string          `gorm:"size:3;not null" json:"currency"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

func (GoldPrice) TableName() string {
	return "gold_prices"
}
