package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-stock-prediction/pkg/logger"
)

// statusRecorder wraps http.ResponseWriter to capture the written status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// clientIP returns the first hop from X-Forwarded-For when present,
// falling back to the RemoteAddr host portion.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		// XFF may be a comma-separated list; first entry is the originating client.
		if idx := strings.Index(fwd, ","); idx != -1 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	// Strip port from RemoteAddr (host:port or [::1]:port).
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// noopPaths lists endpoints that are excluded from access logging to reduce noise.
var noopPaths = map[string]bool{
	"/health":        true,
	"/health/simple": true,
	"/health/ready":  true,
}

// AccessLogMiddleware logs one line per completed HTTP request.
// It must run after JWTMiddleware so the authenticated username is available.
func AccessLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if noopPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(rec, r)

		dur := time.Since(start).Milliseconds()
		ip := clientIP(r)

		user := "-"
		if claims := getClaims(r); claims != nil {
			if sub, ok := claims["sub"].(string); ok && sub != "" {
				user = sub
			}
		}

		line := fmt.Sprintf(
			"method=%s path=%s status=%d dur_ms=%d ip=%s user=%s",
			r.Method, r.URL.Path, rec.status, dur, ip, user,
		)

		switch {
		case rec.status >= 500:
			logger.Logger.Errorf("request %s", line)
		case rec.status >= 400:
			logger.Logger.Warnf("request %s", line)
		default:
			logger.Logger.Infof("request %s", line)
		}
	})
}
