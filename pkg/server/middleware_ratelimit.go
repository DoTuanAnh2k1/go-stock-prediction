package server

import (
	"net"
	"net/http"
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
	r:        rate.Limit(10),
	b:        30,
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
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		limiter := globalLimiter.getLimiter(ip)
		if !limiter.Allow() {
			ResponseError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
