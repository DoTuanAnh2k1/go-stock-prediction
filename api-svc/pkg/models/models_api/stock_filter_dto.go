package modelsapi

import "github.com/shopspring/decimal"

type StockFilterDTO struct {
	Symbols   []string         `json:"symbols,omitempty"`
	Sector    string           `json:"sector,omitempty"`
	Exchange  string           `json:"exchange,omitempty"`
	IsVN100   *bool            `json:"is_vn100,omitempty"`
	MinPrice  *decimal.Decimal `json:"min_price,omitempty"`
	MaxPrice  *decimal.Decimal `json:"max_price,omitempty"`
	MinVolume *int64           `json:"min_volume,omitempty"`
	MinChange *decimal.Decimal `json:"min_change,omitempty"`
	MaxChange *decimal.Decimal `json:"max_change,omitempty"`
	SortBy    string           `json:"sort_by,omitempty"`    // "price", "change", "volume", "value"
	SortOrder string           `json:"sort_order,omitempty"` // "asc", "desc"
	Limit     int              `json:"limit,omitempty"`
}

type StockSearchDTO struct {
	Query   string                 `json:"query"`
	Results []StockCurrentPriceDTO `json:"results"`
	Total   int                    `json:"total"`
}
