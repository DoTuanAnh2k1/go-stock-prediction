package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"go-stock-prediction/pkg/logger"
	"go-stock-prediction/pkg/store/repository"
)

// fuelLatestItem represents a single entry in the /api/fuel/latest response.
type fuelLatestItem struct {
	ProductType string          `json:"product_type"`
	Price       decimal.Decimal `json:"price"`
	TradingDate string          `json:"trading_date"`
}

type fuelLatestResponse struct {
	Data      []fuelLatestItem `json:"data"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// fuelProducts is the canonical list of Vietnamese retail fuel product types.
var fuelProducts = []string{"ron95_iii", "ron95_v", "diezel_005s", "dau_hoa"}

// GetFuelLatest handles GET /api/fuel/latest
func GetFuelLatest(w http.ResponseWriter, r *http.Request) {
	store := repository.GetSingleton()

	// Try GetFuelProducts first; fall back to hard-coded list if not yet populated.
	products, err := store.GetFuelProducts()
	if err != nil || len(products) == 0 {
		products = fuelProducts
	}

	items := make([]fuelLatestItem, 0, len(products))
	for _, pt := range products {
		p, err := store.GetLatestFuelPrice(pt)
		if err != nil || p == nil {
			continue
		}
		items = append(items, fuelLatestItem{
			ProductType: p.ProductType,
			Price:       p.Price,
			TradingDate: p.TradingDate.Format("2006-01-02"),
		})
	}

	ResponseSuccess(w, http.StatusOK, fuelLatestResponse{
		Data:      items,
		UpdatedAt: time.Now().UTC(),
	})
}

// fuelPricePoint is a single date entry for historical fuel prices.
type fuelPricePoint struct {
	Date  string          `json:"date"`
	Price decimal.Decimal `json:"price"`
}

type fuelPricesResponse struct {
	ProductType string           `json:"product_type"`
	Data        []fuelPricePoint `json:"data"`
}

// GetFuelPrices handles GET /api/fuel/prices?product=ron95_iii&days=180
func GetFuelPrices(w http.ResponseWriter, r *http.Request) {
	productType := r.URL.Query().Get("product")
	daysStr := r.URL.Query().Get("days")

	days := 180
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	store := repository.GetSingleton()
	to := time.Now()
	from := to.AddDate(0, 0, -days)

	prices, err := store.GetFuelPricesByDateRange(productType, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/fuel/prices] Failed to get fuel prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get fuel prices")
		return
	}

	data := make([]fuelPricePoint, 0, len(prices))
	for _, p := range prices {
		data = append(data, fuelPricePoint{
			Date:  p.TradingDate.Format("2006-01-02"),
			Price: p.Price,
		})
	}

	ResponseSuccess(w, http.StatusOK, fuelPricesResponse{
		ProductType: productType,
		Data:        data,
	})
}

type fuelChartResponse struct {
	ProductType string            `json:"product_type"`
	Dates       []string          `json:"dates"`
	Prices      []decimal.Decimal `json:"prices"`
}

// GetFuelChart handles GET /api/fuel/chart?product=ron95_iii&days=180
func GetFuelChart(w http.ResponseWriter, r *http.Request) {
	productType := r.URL.Query().Get("product")
	daysStr := r.URL.Query().Get("days")

	days := 180
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	store := repository.GetSingleton()
	to := time.Now()
	from := to.AddDate(0, 0, -days)

	prices, err := store.GetFuelPricesByDateRange(productType, from, to)
	if err != nil {
		logger.Logger.Errorf("[api/fuel/chart] Failed to get fuel prices: %v", err)
		ResponseError(w, http.StatusInternalServerError, "Failed to get fuel prices")
		return
	}

	dates := make([]string, 0, len(prices))
	fuelPrices := make([]decimal.Decimal, 0, len(prices))

	// prices are ordered DESC from DB — reverse for chart (oldest first)
	for i := len(prices) - 1; i >= 0; i-- {
		p := prices[i]
		dates = append(dates, p.TradingDate.Format("2006-01-02"))
		fuelPrices = append(fuelPrices, p.Price)
	}

	ResponseSuccess(w, http.StatusOK, fuelChartResponse{
		ProductType: productType,
		Dates:       dates,
		Prices:      fuelPrices,
	})
}
