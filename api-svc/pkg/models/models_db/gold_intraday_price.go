package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
)

// GoldIntradayPrice stores hourly intraday price data for gold products.
type GoldIntradayPrice struct {
	ID          uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	Source      string           `gorm:"type:varchar(50);not null;uniqueIndex:idx_gold_intraday_src_prod_ts,priority:1" json:"source"`
	ProductType string           `gorm:"type:varchar(50);not null;uniqueIndex:idx_gold_intraday_src_prod_ts,priority:2" json:"product_type"`
	Timestamp   time.Time        `gorm:"type:datetime(0);not null;uniqueIndex:idx_gold_intraday_src_prod_ts,priority:3" json:"timestamp"`
	OpenPrice   *decimal.Decimal `gorm:"type:numeric(15,2)" json:"open_price"`
	HighPrice   *decimal.Decimal `gorm:"type:numeric(15,2)" json:"high_price"`
	LowPrice    *decimal.Decimal `gorm:"type:numeric(15,2)" json:"low_price"`
	BuyPrice    decimal.Decimal  `gorm:"type:decimal(15,2)" json:"buy_price"`
	SellPrice   decimal.Decimal  `gorm:"type:decimal(15,2)" json:"sell_price"`
	Currency    string           `gorm:"type:varchar(3);not null" json:"currency"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

func (GoldIntradayPrice) TableName() string { return "gold_intraday_prices" }
