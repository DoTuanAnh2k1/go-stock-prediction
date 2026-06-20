package modelsapi

import "time"

// StockDTO - DTO cho stock response
type StockDTO struct {
	ID          uint       `json:"id"`
	Symbol      string     `json:"symbol"`
	CompanyName string     `json:"company_name"`
	ExchangeID  uint       `json:"exchange_id"`
	IsVN100     bool       `json:"is_vn100"`
	Sector      string     `json:"sector"`
	ListingDate *time.Time `json:"listing_date,omitempty"`
}

// CreateStockRequest - request để tạo stock
type CreateStockRequest struct {
	Symbol      string     `json:"symbol" validate:"required,max=10"`
	CompanyName string     `json:"company_name" validate:"required,max=200"`
	ExchangeID  uint       `json:"exchange_id" validate:"required"`
	IsVN100     bool       `json:"is_vn100"`
	Sector      string     `json:"sector,omitempty"`
	ListingDate *time.Time `json:"listing_date,omitempty"`
}

// UpdateStockRequest - request để update stock
type UpdateStockRequest struct {
	CompanyName *string    `json:"company_name,omitempty"`
	IsVN100     *bool      `json:"is_vn100,omitempty"`
	Sector      *string    `json:"sector,omitempty"`
	ListingDate *time.Time `json:"listing_date,omitempty"`
}

// StockListResponse - response cho danh sách stocks
type StockListResponse struct {
	Stocks []StockDTO `json:"stocks"`
	Total  int        `json:"total"`
}
