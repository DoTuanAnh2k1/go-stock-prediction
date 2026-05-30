package modelsdb

import (
	"time"

	"gorm.io/gorm"
)

// Stock - Bảng stocks
type Stock struct {
	ID          uint           `gorm:"type:int unsigned;primaryKey;autoIncrement" json:"id"`
	Symbol      string         `gorm:"uniqueIndex;size:10;not null" json:"symbol"` // VCB, VIC, FPT
	CompanyName string         `gorm:"size:200;not null" json:"company_name"`
	ExchangeID  uint           `gorm:"type:int unsigned;not null;index" json:"exchange_id"`
	IsVN30      bool           `gorm:"default:false;index" json:"is_vn30"`
	IsVN100     bool           `gorm:"default:false" json:"is_vn100"`
	ListingDate *time.Time     `json:"listing_date"`
	Sector      string         `gorm:"size:100" json:"sector"` // Ngân hàng, BĐS, etc.
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Stock) TableName() string {
	return "stocks"
}
