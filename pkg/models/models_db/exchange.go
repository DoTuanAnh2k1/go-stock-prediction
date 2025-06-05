package modelsdb

import (
	"time"

	"gorm.io/gorm"
)

// Exchange - Bảng exchanges
type Exchange struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Code      string         `gorm:"uniqueIndex;size:10;not null" json:"code"` // HOSE, HNX, UPCOM
	Name      string         `gorm:"size:100;not null" json:"name"`
	Timezone  string         `gorm:"size:50;default:'Asia/Ho_Chi_Minh'" json:"timezone"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Exchange) TableName() string {
	return "exchanges"
}
