package modelsapi

import "time"

// HealthCheckResponse - response cho health check
type HealthCheckResponse struct {
	Status    string            `json:"status"` // healthy, unhealthy
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"` // database: ok, crawler: ok
	Uptime    string            `json:"uptime"`
}

// SystemStatsDTO - system statistics
type SystemStatsDTO struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	Goroutines  int     `json:"goroutines"`
}

// CrawlerStatusDTO - crawler status
type CrawlerStatusDTO struct {
	IsRunning     bool      `json:"is_running"`
	LastCrawlTime time.Time `json:"last_crawl_time"`
	NextCrawlTime time.Time `json:"next_crawl_time"`
	SuccessCount  int       `json:"success_count"`
	ErrorCount    int       `json:"error_count"`
	CrawlDuration int64     `json:"crawl_duration_ms"`
}
