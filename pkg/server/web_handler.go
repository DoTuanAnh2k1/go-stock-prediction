package server

import (
	"encoding/json"
	"fmt"
	"go-stock-prediction/pkg/config"
	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
	"html/template"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// WebHandler handles web dashboard routes
type WebHandler struct {
	templates *template.Template
}

// PageData contains common data for all pages
type PageData struct {
	Title       string
	Description string
	Version     string
	ServerName  string
	Page        string // current page identifier
}

// NewWebHandler creates a new web handler with improved template loading
func NewWebHandler() *WebHandler {
	logger.Logger.Info("🌐 Initializing web handler...")

	// Load all HTML templates from web/templates directory
	tmpl, err := template.ParseGlob("web/templates/*.html")
	if err != nil {
		logger.Logger.Errorf("❌ Failed to parse templates: %v", err)
		logger.Logger.Warn("⚠️ Web interface will not be available")
		return &WebHandler{templates: nil}
	}

	logger.Logger.Info("✅ Successfully loaded HTML templates")
	return &WebHandler{templates: tmpl}
}

// SetupWebRoutes sets up all web routes with improved routing
func SetupWebRoutes(mux *http.ServeMux, webHandler *WebHandler) {
	logger.Logger.Info("🛣️ Setting up web routes...")

	// Serve static files (CSS, JS, images) from web/static/
	fileServer := http.FileServer(http.Dir("web/static/"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	logger.Logger.Info("📁 Static files route: /static/* → web/static/")

	// Dashboard routes
	mux.HandleFunc("/", webHandler.DashboardHandler)
	mux.HandleFunc("/dashboard", webHandler.DashboardHandler)
	logger.Logger.Info("🏠 Dashboard routes: / and /dashboard")

	// Stocks page
	mux.HandleFunc("/stocks", webHandler.StocksHandler)
	logger.Logger.Info("📈 Stocks route: /stocks")

	// Predictions page
	mux.HandleFunc("/predictions", webHandler.PredictionsHandler)
	logger.Logger.Info("🔮 Predictions route: /predictions")

	// Training page
	mux.HandleFunc("/training", webHandler.TrainingHandler)
	logger.Logger.Info("🧠 Training route: /training")

	// API route placeholder (will be handled by API handlers)
	logger.Logger.Info("🔌 API routes will be handled separately")

	logger.Logger.Info("✅ All web routes configured successfully")
}

// DashboardHandler serves the main dashboard page
func (wh *WebHandler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Infof("🏠 Serving dashboard to %s", getClientIP(r))

	data := wh.createPageData("VN Stock Prediction Dashboard", "dashboard")
	wh.renderTemplate(w, "dashboard.html", data)
}

// StocksHandler serves the stocks page
func (wh *WebHandler) StocksHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Infof("📈 Serving stocks page to %s", getClientIP(r))

	data := wh.createPageData("Stocks - VN Stock Dashboard", "stocks")
	wh.renderTemplate(w, "stocks.html", data)
}

// PredictionsHandler serves the predictions page
func (wh *WebHandler) PredictionsHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Infof("🔮 Serving predictions page to %s", getClientIP(r))

	data := wh.createPageData("Predictions - VN Stock Dashboard", "predictions")
	wh.renderTemplate(w, "predictions.html", data)
}

// TrainingHandler serves the training page
func (wh *WebHandler) TrainingHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Infof("🧠 Serving training page to %s", getClientIP(r))

	data := wh.createPageData("ML Training - VN Stock Dashboard", "training")
	wh.renderTemplate(w, "training.html", data)
}

// Helper methods

// createPageData creates common page data
func (wh *WebHandler) createPageData(title, page string) PageData {
	return PageData{
		Title:       title,
		Description: "Real-time Vietnamese stock market analysis with ML predictions",
		Version:     "1.0.0",
		ServerName:  config.GetServerConfig().ServerName,
		Page:        page,
	}
}

// renderTemplate renders a template with error handling
func (wh *WebHandler) renderTemplate(w http.ResponseWriter, templateName string, data PageData) {
	// Check if templates are loaded
	if wh.templates == nil {
		logger.Logger.Error("❌ Templates not loaded")
		http.Error(w, "Templates not available", http.StatusInternalServerError)
		return
	}

	// Set proper headers
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Execute template
	err := wh.templates.ExecuteTemplate(w, templateName, data)
	if err != nil {
		logger.Logger.Errorf("❌ Template execution failed for %s: %v", templateName, err)

		// If it's dashboard, try to serve a basic error page
		if templateName == "dashboard.html" {
			wh.serveErrorPage(w, "Dashboard temporarily unavailable")
		} else {
			// For other pages, redirect to dashboard
			logger.Logger.Infof("↩️ Redirecting to dashboard due to template error")
			http.Redirect(w, &http.Request{}, "/", http.StatusTemporaryRedirect)
		}
	}
}

// serveErrorPage serves a basic error page when templates fail
func (wh *WebHandler) serveErrorPage(w http.ResponseWriter, message string) {
	errorHTML := `
<!DOCTYPE html>
<html>
<head>
    <title>VN Stock Dashboard - Error</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: #f5f5f5; }
        .error-container { background: white; padding: 40px; border-radius: 10px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); display: inline-block; }
        .error-icon { font-size: 48px; margin-bottom: 20px; }
        h1 { color: #e53e3e; margin-bottom: 10px; }
        p { color: #666; margin-bottom: 20px; }
        .retry-btn { background: #4299e1; color: white; padding: 10px 20px; border: none; border-radius: 5px; cursor: pointer; }
        .retry-btn:hover { background: #3182ce; }
    </style>
</head>
<body>
    <div class="error-container">
        <div class="error-icon">😞</div>
        <h1>Oops! Something went wrong</h1>
        <p>` + message + `</p>
        <button class="retry-btn" onclick="window.location.reload()">Retry</button>
        <button class="retry-btn" onclick="window.location.href='/'">Go to Dashboard</button>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(errorHTML))
}

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
func (wh *WebHandler) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	logger.Logger.Debug("❤️ Health check requested from %s", getClientIP(r))

	w.Header().Set("Content-Type", "application/json")

	// Check all services health
	services := checkServicesHealth(wh)
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
func checkServicesHealth(wh *WebHandler) map[string]string {
	services := make(map[string]string)

	// Check templates
	if wh.templates != nil {
		services["templates"] = "healthy"
	} else {
		services["templates"] = "unhealthy"
	}

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

	// You can add more service checks here:
	// - Redis connection
	// - External APIs
	// - File system access
	// - Message queues, etc.

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

// Simple health check for load balancers (just returns 200 OK)
func SimpleHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Ready check - returns 200 when service is ready to accept traffic
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

// SetupHealthRoute adds health check route
func SetupHealthRoute(mux *http.ServeMux, webHandler *WebHandler) {
	mux.HandleFunc("/health", webHandler.HealthCheckHandler)
	logger.Logger.Info("❤️ Health check route: /health")
}
