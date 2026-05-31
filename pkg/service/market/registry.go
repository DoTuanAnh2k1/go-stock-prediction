package market

import "sync"

var (
	mu      sync.RWMutex
	markets = map[string]AssetMarket{}
)

// Register adds a market. Call from your market's init() function.
func Register(m AssetMarket) {
	mu.Lock()
	defer mu.Unlock()
	markets[m.MarketKey()] = m
}

// Get returns the market for the given key.
func Get(key string) (AssetMarket, bool) {
	mu.RLock()
	defer mu.RUnlock()
	m, ok := markets[key]
	return m, ok
}

// All returns all registered markets.
func All() []AssetMarket {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]AssetMarket, 0, len(markets))
	for _, m := range markets {
		result = append(result, m)
	}
	return result
}
