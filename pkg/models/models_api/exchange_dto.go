package modelsapi

// ExchangeDTO - DTO cho exchange response
type ExchangeDTO struct {
	ID       uint   `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

// CreateExchangeRequest - request để tạo exchange
type CreateExchangeRequest struct {
	Code     string `json:"code" validate:"required,max=10"`
	Name     string `json:"name" validate:"required,max=100"`
	Timezone string `json:"timezone,omitempty"`
}

// UpdateExchangeRequest - request để update exchange
type UpdateExchangeRequest struct {
	Name     *string `json:"name,omitempty"`
	Timezone *string `json:"timezone,omitempty"`
}
