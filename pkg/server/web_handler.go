package server

import (
	"encoding/json"
	"fmt"
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP if multiple are present
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Use RemoteAddr as fallback
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx] // Remove port
	}

	return ip
}

// HealthStatus represents the health check response structure
type HealthStatus struct {
	Status     string            `json:"status"`
	Timestamp  string            `json:"timestamp"`
	Version    string            `json:"version"`
	ServerName string            `json:"server_name"`
	Services   map[string]string `json:"services"`
	System     SystemInfo        `json:"system"`
	Uptime     string            `json:"uptime"`
}

// SystemInfo contains system information
type SystemInfo struct {
	GoVersion    string      `json:"go_version"`
	NumGoroutine int         `json:"num_goroutine"`
	MemStats     MemoryStats `json:"memory"`
}

// MemoryStats contains memory usage information
type MemoryStats struct {
	Alloc      uint64 `json:"alloc_mb"`
	TotalAlloc uint64 `json:"total_alloc_mb"`
	Sys        uint64 `json:"sys_mb"`
}

var startTime = time.Now()

// HealthCheckHandler provides a comprehensive health check endpoint
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Debugf("Health check requested from %s", getClientIP(r))

	w.Header().Set("Content-Type", "application/json")

	// Check all services health
	services := checkServicesHealth()
	overallStatus := determineOverallStatus(services)

	// Get system info
	var memStats runtime.MemStats
	runtime.GC() // Force garbage collection for accurate stats
	runtime.ReadMemStats(&memStats)

	systemInfo := SystemInfo{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		MemStats: MemoryStats{
			Alloc:      memStats.Alloc / 1024 / 1024,      // Convert to MB
			TotalAlloc: memStats.TotalAlloc / 1024 / 1024, // Convert to MB
			Sys:        memStats.Sys / 1024 / 1024,        // Convert to MB
		},
	}

	// Create health status
	health := HealthStatus{
		Status:     overallStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Version:    "1.0.0",
		ServerName: config.GetServerConfig().ServerName,
		Services:   services,
		System:     systemInfo,
		Uptime:     formatUptime(time.Since(startTime)),
	}

	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)

	// Marshal and send JSON response
	response, err := json.Marshal(health)
	if err != nil {
		logger.Logger.Errorf("Failed to marshal health check response: %v", err)
		http.Error(w, `{"status":"error","message":"Failed to generate health status"}`, http.StatusInternalServerError)
		return
	}

	w.Write(response)
}

// checkServicesHealth checks the health of all services
func checkServicesHealth() map[string]string {
	services := make(map[string]string)

	// Check database
	store := repository.GetSingleton()
	if store != nil {
		err := store.Ping()
		if err != nil {
			services["database"] = "unhealthy"
			logger.Logger.Debugf("Database health check failed: %v", err)
		} else {
			services["database"] = "healthy"
		}
	} else {
		services["database"] = "unavailable"
	}

	// Check config
	cfg := config.Get()
	if cfg != nil {
		services["config"] = "healthy"
	} else {
		services["config"] = "unhealthy"
	}

	return services
}

// determineOverallStatus determines the overall system status
func determineOverallStatus(services map[string]string) string {
	unhealthyCount := 0
	totalServices := len(services)

	for _, status := range services {
		if status != "healthy" {
			unhealthyCount++
		}
	}

	// If more than 50% of services are unhealthy, system is unhealthy
	if float64(unhealthyCount)/float64(totalServices) > 0.5 {
		return "unhealthy"
	}

	// If any critical service is down, system is degraded
	if services["database"] != "healthy" || services["config"] != "healthy" {
		return "degraded"
	}

	// If some non-critical services are down, system is degraded
	if unhealthyCount > 0 {
		return "degraded"
	}

	return "healthy"
}

// formatUptime formats duration to human readable string
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// SimpleHealthHandler is a simple health check for load balancers (just returns 200 OK)
func SimpleHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ReadyHandler returns 200 when the service is ready to accept traffic
func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if critical services are ready
	store := repository.GetSingleton()
	if store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("NOT READY"))
		return
	}

	err := store.Ping()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("NOT READY"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("READY"))
}
