package modelsapi

// SearchRequest - request cho search
type SearchRequest struct {
	Query    string `json:"query" form:"query" validate:"required,min=1"`
	Category string `json:"category,omitempty" form:"category"` // stock, prediction, etc.
	Limit    int    `json:"limit,omitempty" form:"limit" validate:"max=50"`
}

// SearchResult - kết quả search
type SearchResult struct {
	Type        string      `json:"type"` // stock, prediction, etc.
	ID          uint        `json:"id"`
	Title       string      `json:"title"`       // Stock symbol hoặc company name
	Description string      `json:"description"` // Additional info
	Data        interface{} `json:"data"`        // Actual object
}

// SearchResponse - response cho search
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Query   string         `json:"query"`
}
