package modelsapi

import "time"

// PaginationRequest - request cho pagination
type PaginationRequest struct {
	Page     int `json:"page" form:"page" validate:"min=1"`
	PageSize int `json:"page_size" form:"page_size" validate:"min=1,max=100"`
}

// PaginationResponse - response cho pagination
type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// DateRangeRequest - request cho date range queries
type DateRangeRequest struct {
	FromDate time.Time `json:"from_date" form:"from_date"`
	ToDate   time.Time `json:"to_date" form:"to_date"`
}

// FilterRequest - request cho filtering
type FilterRequest struct {
	Symbol     string `json:"symbol,omitempty" form:"symbol"`
	ExchangeID *uint  `json:"exchange_id,omitempty" form:"exchange_id"`
	Sector     string `json:"sector,omitempty" form:"sector"`
	IsVN100    *bool  `json:"is_vn100,omitempty" form:"is_vn100"`
}

// SortRequest - request cho sorting
type SortRequest struct {
	SortBy    string `json:"sort_by" form:"sort_by"`       // field name
	SortOrder string `json:"sort_order" form:"sort_order"` // asc, desc
}

// ApiResponse - generic API response wrapper
type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ErrorResponse - API error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code"`
}

// SuccessResponse - API success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}
