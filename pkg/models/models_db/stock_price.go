package modelsdb

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// StockPrice - Bảng stock_prices
type StockPrice struct {
	ID            uint            `gorm:"primaryKey;autoIncrement" json:"id"`
	StockID       uint            `gorm:"not null;index:idx_stock_date" json:"stock_id"`
	TradingDate   time.Time       `gorm:"not null;index:idx_stock_date" json:"trading_date"`
	OpenPrice     decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"open_price"`
	HighPrice     decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"high_price"`
	LowPrice      decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"low_price"`
	ClosePrice    decimal.Decimal `gorm:"type:decimal(15,2);not null" json:"close_price"`
	Volume        int64           `gorm:"not null" json:"volume"`
	Value         decimal.Decimal `gorm:"type:decimal(20,2)" json:"value"`         // Giá trị GD (VNĐ)
	ForeignBuy    int64           `gorm:"default:0" json:"foreign_buy"`            // KL nước ngoài mua
	ForeignSell   int64           `gorm:"default:0" json:"foreign_sell"`           // KL nước ngoài bán
	Change        decimal.Decimal `gorm:"type:decimal(15,2)" json:"change"`        // Thay đổi giá
	ChangePercent decimal.Decimal `gorm:"type:decimal(5,4)" json:"change_percent"` // % thay đổi
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"deleted_at,omitempty"`
}

func (StockPrice) TableName() string {
	return "stock_prices"
}
