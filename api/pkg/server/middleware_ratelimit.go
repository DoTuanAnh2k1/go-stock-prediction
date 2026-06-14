package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.Mutex
	r        rate.Limit
	b        int
}

var globalLimiter = &ipRateLimiter{
	limiters: make(map[string]*rate.Limiter),
	r:        rate.Limit(60), // 60 req/s — browser loads ~38 parallel requests on page load
	b:        120,            // burst 120 to handle initial sparkline + gold chart batch
}

var loginLimiter = &ipRateLimiter{
	limiters: make(map[string]*rate.Limiter),
	r:        rate.Limit(5.0 / 60.0), // 5 attempts per minute
	b:        5,
}

func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	limiter, exists := l.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(l.r, l.b)
		l.limiters[ip] = limiter
	}
	return limiter
}

// getRealIP extracts the real client IP, respecting X-Real-IP set by Nginx.
func getRealIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.SplitN(forwarded, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			globalLimiter.mu.Lock()
			globalLimiter.limiters = make(map[string]*rate.Limiter)
			globalLimiter.mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getRealIP(r)
		limiter := globalLimiter.getLimiter(ip)
		if !limiter.Allow() {
			ResponseError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoginRateLimitMiddleware applies a strict per-IP rate limit to login attempts (5/min).
func LoginRateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			loginLimiter.mu.Lock()
			loginLimiter.limiters = make(map[string]*rate.Limiter)
			loginLimiter.mu.Unlock()
		}
	}()

	return func(w http.ResponseWriter, r *http.Request) {
		ip := getRealIP(r)
		limiter := loginLimiter.getLimiter(ip)
		if !limiter.Allow() {
			ResponseError(w, http.StatusTooManyRequests, "Too many login attempts. Please try again later.")
			return
		}
		next(w, r)
	}
}
