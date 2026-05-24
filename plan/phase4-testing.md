# Phase 4: Testing

Hiện tại project có 0 test files. Mục tiêu: đạt 60%+ coverage ở business logic layer.

---

## Chiến lược testing

Không cần test tất cả — tập trung vào:
1. **Pure functions** (dễ test nhất, ít setup)
2. **Business logic** (repository, prediction algorithms)
3. **HTTP handlers** (integration-level)

Không test: web templates, logging setup, main.go.

---

## 4.1 — Unit tests cho prediction algorithms

**Tạo files:**
- `pkg/service/predict/moving_average/algo_test.go`
- `pkg/service/predict/arima_garch/algo_test.go`
- `pkg/service/predict/lstm_nn/algo_test.go`

**Các case cần test:**

### Moving Average
```go
func TestCalculateSMA(t *testing.T) {
    algo := NewMovingAveragePredictor()
    prices := []float64{10, 20, 30, 40, 50}
    sma := algo.calculateSMA(prices, 3)
    // SMA(3) của [10,20,30,40,50] = mean([30,40,50]) = 40
    assert.InDelta(t, 40.0, sma, 0.001)
}

func TestCalculateVWMA_WithZeroVolume(t *testing.T) {
    // Nếu volume = 0, phải fallback về SMA, không panic
    algo := NewMovingAveragePredictor()
    prices := []float64{10, 20, 30}
    volumes := []float64{0, 0, 0}
    result := algo.calculateVWMA(prices, volumes, 3)
    assert.NotNaN(t, result)
    assert.NotInf(t, result, 0)
}

func TestGenerateSignal_Crossover(t *testing.T) {
    // Short MA > Long MA → BUY
    // Short MA < Long MA → SELL
    // Close → HOLD
}

func TestPredict_InsufficientData(t *testing.T) {
    // Ít hơn 20 data points → error hoặc fallback graceful
    algo := NewMovingAveragePredictor()
    data := modelsvc.StockData{ClosePrices: []string{"100", "101"}}
    ctx := context.Background()
    _, err := algo.Predict(ctx, data)
    // Expect error hoặc low-confidence prediction, không panic
    assert.NotPanics(t, func() { algo.Predict(ctx, data) })
}

func TestRandom_Distribution(t *testing.T) {
    // Kiểm tra random không trả về cùng value liên tiếp
    algo := NewMovingAveragePredictor()
    values := make(map[float64]int)
    for i := 0; i < 100; i++ {
        v := algo.random()
        values[v]++
    }
    // Không có value nào xuất hiện > 10 lần (nếu random đúng cách)
    for _, count := range values {
        assert.LessOrEqual(t, count, 10)
    }
}
```

### ARIMA-GARCH
```go
func TestCalculateReturns(t *testing.T) {
    // Log returns của [100, 110] = ln(110/100) ≈ 0.0953
    algo := &ARIMAGARCHPredictor{}
    prices := []float64{100, 110, 121}
    returns := algo.calculateReturns(prices)
    assert.Len(t, returns, 2)
    assert.InDelta(t, math.Log(110.0/100.0), returns[0], 0.0001)
}

func TestFitAR_StationaryData(t *testing.T) {
    // Với white noise, coefficients phải gần 0
    algo := &ARIMAGARCHPredictor{p: 2}
    noise := generateWhiteNoise(100)
    coeffs := algo.fitAR(noise, 2)
    for _, c := range coeffs {
        assert.InDelta(t, 0.0, c, 0.5) // không quá lớn
    }
}

func TestFitGARCH_VarianceConstraint(t *testing.T) {
    // alpha + beta < 1 (covariance stationarity)
    algo := &ARIMAGARCHPredictor{}
    returns := generateVolatileReturns(100)
    alpha, beta, omega := algo.fitGARCH(returns)
    assert.Less(t, alpha+beta, 1.0)
    assert.Greater(t, omega, 0.0)
}
```

---

## 4.2 — Unit tests cho helper functions

**Tạo file:** `pkg/server/helper_test.go`

```go
func TestValidateSymbol(t *testing.T) {
    tests := []struct {
        symbol  string
        wantErr bool
    }{
        {"VIC", false},
        {"VHM", false},
        {"HPG", false},
        {"", true},         // empty
        {"TOOLONGSYMBOL", true}, // too long
        {"VIC!", true},     // invalid char
        {"vic", false},     // lowercase (should pass or fail based on decision)
    }
    for _, tt := range tests {
        err := validateSymbol(tt.symbol)
        if tt.wantErr {
            assert.Error(t, err, "symbol=%s", tt.symbol)
        } else {
            assert.NoError(t, err, "symbol=%s", tt.symbol)
        }
    }
}

func TestValidateLimit(t *testing.T) {
    limit, err := validateLimit("50", 20, 100)
    assert.NoError(t, err)
    assert.Equal(t, 50, limit)

    _, err = validateLimit("-1", 20, 100)
    assert.Error(t, err)

    _, err = validateLimit("999", 20, 100)
    assert.Error(t, err) // exceeds max

    limit, err = validateLimit("", 20, 100)
    assert.NoError(t, err)
    assert.Equal(t, 20, limit) // default
}

func TestCalculateVolatility(t *testing.T) {
    // Volatility của constant series = 0
    changes := []float64{0, 0, 0, 0, 0}
    assert.InDelta(t, 0.0, calculateVolatility(changes), 0.0001)

    // Volatility của [1, -1, 1, -1] > 0
    changes = []float64{1, -1, 1, -1}
    assert.Greater(t, calculateVolatility(changes), 0.0)
}

func TestGetPredictionStatus(t *testing.T) {
    future := time.Now().Add(24 * time.Hour)
    status := getPredictionStatus(future, 0)
    assert.Equal(t, "pending", status)

    past := time.Now().Add(-24 * time.Hour)
    status = getPredictionStatus(past, 100.0) // has actual price
    assert.Equal(t, "confirmed", status)
}
```

---

## 4.3 — Integration tests cho database layer

**Tạo file:** `pkg/store/mysql/integration_test.go`

Dùng Docker test container hoặc SQLite in-memory (qua GORM) để test DB layer
mà không cần MySQL thực.

**Dùng SQLite in-memory cho tests:**
```go
//go:build integration

package mysql_test

import (
    "testing"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupTestDB(t *testing.T) *Client {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    db.AutoMigrate(
        &modelsdb.Exchange{},
        &modelsdb.Stock{},
        &modelsdb.StockPrice{},
        &modelsdb.Prediction{},
        &modelsdb.SyncLog{},
    )
    return &Client{db: db}
}

func TestGetLatestStockPricesForVN30(t *testing.T) {
    client := setupTestDB(t)

    // Insert test data
    exchange := modelsdb.Exchange{Code: "HOSE"}
    client.db.Create(&exchange)

    stock := modelsdb.Stock{Symbol: "VIC", IsVN30: true, ExchangeID: exchange.ID}
    client.db.Create(&stock)

    // Insert 2 prices — chỉ lấy price mới nhất
    price1 := modelsdb.StockPrice{StockID: stock.ID, TradeDate: time.Now().AddDate(0,0,-1), Close: 100}
    price2 := modelsdb.StockPrice{StockID: stock.ID, TradeDate: time.Now(), Close: 110}
    client.db.Create(&price1)
    client.db.Create(&price2)

    prices, err := client.GetLatestStockPricesForVN30(context.Background())
    require.NoError(t, err)
    require.Len(t, prices, 1)
    assert.Equal(t, price2.Close, prices[0].Close) // phải lấy cái mới nhất
}
```

**Thêm dependency:** `gorm.io/driver/sqlite` cho test chỉ.

---

## 4.4 — HTTP handler tests

**Tạo file:** `pkg/server/handlers_test.go`

```go
func TestGetPredictions_InvalidSymbol(t *testing.T) {
    server := setupTestServer()
    req := httptest.NewRequest("GET", "/api/predictions?symbol=INVALID!!!&limit=abc", nil)
    w := httptest.NewRecorder()
    server.GetPredictions(w, req)
    assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetPredictions_ValidRequest(t *testing.T) {
    server := setupTestServer()
    req := httptest.NewRequest("GET", "/api/predictions?symbol=VIC&limit=10", nil)
    w := httptest.NewRecorder()
    server.GetPredictions(w, req)
    assert.Equal(t, http.StatusOK, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.NotNil(t, response["data"])
}

func TestTriggerEndpoints_RequireAuth(t *testing.T) {
    // Không có API key → 401
    req := httptest.NewRequest("POST", "/api/trigger/crawler", nil)
    w := httptest.NewRecorder()
    // ...
    assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

---

## 4.5 — Setup test infrastructure

**Tạo file:** `Makefile` (hoặc thêm vào nếu đã có)
```makefile
test:
	go test ./... -v -count=1

test-unit:
	go test ./... -v -count=1 -short

test-integration:
	go test ./... -v -count=1 -tags integration

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total
```

**Target coverage tối thiểu:**
- `pkg/service/predict/*/` — 70%+
- `pkg/server/helper.go` — 80%+
- `pkg/store/mysql/` — 60%+ (integration tests)

---

## Checklist Phase 4

- [ ] 4.1 Tạo test files cho 3 prediction algorithms (pure function tests)
- [ ] 4.2 Tạo `pkg/server/helper_test.go` (validation, status functions)
- [ ] 4.3 Tạo integration tests cho DB layer (dùng SQLite in-memory)
- [ ] 4.4 Tạo HTTP handler tests (dùng `httptest`)
- [ ] 4.5 Thêm `Makefile` với test targets + coverage report

**Thêm dependency cho tests:**
```bash
go get gorm.io/driver/sqlite
go get github.com/stretchr/testify
```

**Thời gian ước tính:** 1–2 ngày
**Mục tiêu:** ≥60% overall coverage, đặc biệt cho business logic
