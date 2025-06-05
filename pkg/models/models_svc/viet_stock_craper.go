package modelssvc

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"golang.org/x/time/rate"
)

type VietStockScraper struct {
	collector   *colly.Collector
	rateLimiter *rate.Limiter
	userAgents  []string
	proxies     []string
	mutex       sync.Mutex
}

func NewVietStockScraper() *VietStockScraper {
	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
	)

	// Giới hạn request để tránh bị block
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*vietstock.vn*",
		Parallelism: 1,
		Delay:       2 * time.Second, // Chờ 2 giây giữa các request
	})

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	}

	return &VietStockScraper{
		collector:   c,
		rateLimiter: rate.NewLimiter(rate.Every(3*time.Second), 1),
		userAgents:  userAgents,
	}
}

// Setup chống phát hiện với rotating headers
func (vs *VietStockScraper) RotateUserAgent() {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	userAgent := vs.userAgents[rand.Intn(len(vs.userAgents))]
	vs.collector.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", userAgent)
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Cache-Control", "max-age=0")
	})
}

// Retry mechanism với exponential backoff
func (vs *VietStockScraper) ScrapeWithRetry(url string, maxRetries int) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Chờ lâu hơn sau mỗi lần thất bại
			waitTime := time.Duration(attempt*attempt) * time.Second
			time.Sleep(waitTime)
		}

		ctx := context.Background()
		if err := vs.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter lỗi: %v", err)
		}

		vs.RotateUserAgent()
		err := vs.collector.Visit(url)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("thất bại sau %d lần thử", maxRetries)
}
