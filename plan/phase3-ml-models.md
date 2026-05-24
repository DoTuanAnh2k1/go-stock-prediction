# Phase 3: ML Models Thực Sự

Thay thế các implementation giả bằng thuật toán thực sự.
Đây là core value của project — accuracy claims hiện tại (93%, 80%, 75%) không có basis.

---

## Bối cảnh hiện tại

Cả 3 algorithms hiện tại đều là **simplified/mock**:

| Algorithm | Vấn đề chính |
|-----------|-------------|
| LSTM Neural Network | Forward pass chỉ là weighted sum + sigmoid, không phải LSTM thực sự |
| Moving Average | Đúng về logic nhưng volume proxy tính sai (dùng price change thay vì real volume) |
| ARIMA-GARCH | Coefficients hardcode, không có MLE fitting thực sự |

**Lựa chọn chiến lược:**
- Option A: Implement Go thuần túy (khó, mất nhiều thời gian, dễ sai)
- Option B: Gọi Python service qua subprocess hoặc gRPC (thực tế hơn)
- **Option C (khuyến nghị): Sửa logic hiện tại để kết quả có ý nghĩa thống kê**, giữ Go-only

Phase này chọn Option C — cải thiện logic mà không thêm external dependencies.

---

## 3.1 — Sửa Moving Average: dùng real volume data

**File:** `pkg/service/predict/moving_average/algo.go`

**Vấn đề:**
Volume proxy hiện tại tính từ price change:
```go
// Fake volume — không có ý nghĩa
volumeProxy := math.Abs(prices[i]-prices[i-1]) / prices[i-1]
```

**Cách sửa:**
`StockData` struct (trong `pkg/models/models_svc/`) đã có field `Volume []string`.
Dùng volume thực thay vì proxy:

```go
func (m *MovingAveragePredictor) calculateVWMA(prices, volumes []float64, period int) float64 {
    if len(prices) < period || len(volumes) < period {
        return m.calculateSMA(prices, period)
    }

    start := len(prices) - period
    totalWeightedPrice := 0.0
    totalVolume := 0.0

    for i := start; i < len(prices); i++ {
        vol := volumes[i]
        if vol <= 0 {
            vol = 1.0 // fallback nếu volume = 0
        }
        totalWeightedPrice += prices[i] * vol
        totalVolume += vol
    }

    if totalVolume == 0 {
        return m.calculateSMA(prices, period)
    }
    return totalWeightedPrice / totalVolume
}
```

Đồng thời cập nhật `Predict()` để parse `data.Volume` cùng lúc với prices.

---

## 3.2 — Sửa ARIMA: implement AR(p) fitting đúng cách

**File:** `pkg/service/predict/arima_garch/algo.go`

**Vấn đề:**
`fitARIMA()` hiện tại dùng coefficients hardcode `[0.3, 0.2, 0.1]` và `[0.1, 0.05]`.

**Cách sửa — dùng OLS (Ordinary Least Squares) cho AR component:**

OLS là cách đơn giản nhất để fit AR(p) mà vẫn có ý nghĩa thống kê:

```go
// fitAR fits AR(p) coefficients using OLS
// Giải phương trình: y[t] = phi[0]*y[t-1] + phi[1]*y[t-2] + ... + phi[p]*y[t-p]
func (a *ARIMAGARCHPredictor) fitAR(returns []float64, p int) []float64 {
    n := len(returns)
    if n <= p {
        return make([]float64, p)
    }

    // Build design matrix X (n-p x p) và response Y (n-p x 1)
    rows := n - p
    X := make([][]float64, rows)
    Y := make([]float64, rows)

    for i := 0; i < rows; i++ {
        X[i] = make([]float64, p)
        for j := 0; j < p; j++ {
            X[i][j] = returns[i+p-j-1]
        }
        Y[i] = returns[i+p]
    }

    // OLS: beta = (X'X)^-1 X'Y
    // Dùng gradient descent thay vì matrix inverse để tránh numerical issues
    coeffs := make([]float64, p)
    lr := 0.001
    for iter := 0; iter < 1000; iter++ {
        gradients := make([]float64, p)
        for i := 0; i < rows; i++ {
            pred := 0.0
            for j := 0; j < p; j++ {
                pred += coeffs[j] * X[i][j]
            }
            residual := Y[i] - pred
            for j := 0; j < p; j++ {
                gradients[j] -= 2 * residual * X[i][j]
            }
        }
        for j := 0; j < p; j++ {
            coeffs[j] -= lr * gradients[j] / float64(rows)
        }
    }
    return coeffs
}
```

Đối với GARCH, dùng moment matching (MLE đơn giản hóa):
```go
func (a *ARIMAGARCHPredictor) fitGARCH(residuals []float64) (alpha, beta, omega float64) {
    // Unconditional variance
    variance := 0.0
    for _, r := range residuals {
        variance += r * r
    }
    variance /= float64(len(residuals))

    // Method of moments estimates
    // Giả sử alpha=0.1, beta=0.8 (empirically common values for VN market)
    alpha = 0.1
    beta = 0.8
    omega = variance * (1 - alpha - beta)
    if omega < 0 {
        omega = variance * 0.1
        alpha = 0.05
        beta = 0.85
    }
    return alpha, beta, omega
}
```

---

## 3.3 — Sửa LSTM: thay bằng thuật toán linear regression proper

**File:** `pkg/service/predict/lstm_nn/algo.go`

**Vấn đề:**
Tên là "LSTM" nhưng không phải LSTM thực. Thay vì giả vờ là LSTM, hãy implement
**Ridge Regression** — linear model với regularization, phù hợp với Go-only, interpretable.

**Lý do chọn Ridge Regression:**
- Implement được trong Go thuần, không cần external lib
- Kết quả có ý nghĩa thống kê, không phải black box
- Regularization tránh overfitting
- Honest về khả năng của model

**Đổi tên file/struct (nếu muốn rename algorithm):**
Hoặc giữ tên "lstm_nn" cho backward compatibility nhưng implement Ridge bên trong.

**Implement Ridge Regression với features:**
```go
type LinearPredictor struct {
    weights    []float64
    bias       float64
    lambda     float64 // regularization
    features   int
    trained    bool
    lastTrained time.Time
}

// Train dùng gradient descent với L2 regularization
func (l *LinearPredictor) train(X [][]float64, Y []float64) {
    n := len(X)
    if n == 0 { return }
    l.features = len(X[0])
    l.weights = make([]float64, l.features)
    l.bias = 0

    lr := 0.001
    epochs := 500

    for epoch := 0; epoch < epochs; epoch++ {
        // Forward pass
        wGrad := make([]float64, l.features)
        bGrad := 0.0
        totalLoss := 0.0

        for i := 0; i < n; i++ {
            pred := l.bias
            for j := 0; j < l.features; j++ {
                pred += l.weights[j] * X[i][j]
            }
            err := pred - Y[i]
            totalLoss += err * err
            bGrad += err
            for j := 0; j < l.features; j++ {
                wGrad[j] += err * X[i][j]
            }
        }

        // Update với L2 regularization
        l.bias -= lr * bGrad / float64(n)
        for j := 0; j < l.features; j++ {
            l.weights[j] -= lr * (wGrad[j]/float64(n) + l.lambda*l.weights[j])
        }

        _ = totalLoss
    }
    l.trained = true
    l.lastTrained = time.Now()
}
```

**Features sử dụng (giống cũ nhưng tính đúng):**
1. Price normalized (close/close[0])
2. 5-day return
3. 20-day return
4. RSI(14) — giữ nguyên logic hiện có
5. Price/20-day SMA ratio
6. Volatility 10-day (std of returns)

---

## 3.4 — Implement backtest để validate accuracy

**Tạo file mới:** `pkg/service/predict/backtest/backtest.go`

**Vấn đề:**
Accuracy claims hoàn toàn tự đặt. Cần backtest thực sự để biết model hoạt động
tốt đến đâu trên historical data.

```go
package backtest

import (
    "context"
    "math"
)

type BacktestResult struct {
    Algorithm      string
    TotalPredictions int
    MAE            float64 // Mean Absolute Error
    RMSE           float64 // Root Mean Squared Error
    MAPE           float64 // Mean Absolute Percentage Error (accuracy proxy)
    DirectionalAccuracy float64 // % lần đoán đúng hướng tăng/giảm
}

// Run thực hiện walk-forward backtest:
// - Dùng data tháng 1-6 để train
// - Predict tháng 7, so sánh với actual
// - Dịch chuyển window 1 tháng, repeat
func Run(ctx context.Context, algo PredictionAlgorithm, data StockData) BacktestResult {
    const trainWindow = 120 // 6 months
    const testWindow = 20   // 1 month

    var errors, pctErrors []float64
    correct := 0
    total := 0

    prices := data.ClosePrices
    for start := 0; start+trainWindow+testWindow <= len(prices); start += testWindow {
        trainData := slice(data, start, start+trainWindow)
        testData := slice(data, start+trainWindow, start+trainWindow+testWindow)

        pred, err := algo.Predict(ctx, trainData)
        if err != nil { continue }

        actual := testData.ClosePrices[0]
        predicted := pred.PredictedPrice

        absErr := math.Abs(actual - predicted)
        pctErr := absErr / actual * 100

        errors = append(errors, absErr*absErr) // for RMSE
        pctErrors = append(pctErrors, pctErr)

        // Directional accuracy
        lastTrainPrice := trainData.ClosePrices[len(trainData.ClosePrices)-1]
        if (predicted > lastTrainPrice) == (actual > lastTrainPrice) {
            correct++
        }
        total++
    }

    mae := mean(sqrtEach(errors))
    rmse := math.Sqrt(mean(errors))
    mape := mean(pctErrors)
    dirAcc := float64(correct) / float64(total) * 100

    return BacktestResult{
        Algorithm: algo.GetName(),
        MAE:  mae,
        RMSE: rmse,
        MAPE: mape,
        DirectionalAccuracy: dirAcc,
    }
}
```

**Thêm endpoint:** `GET /api/algorithms/backtest?symbol=VIC` để chạy backtest on-demand.

---

## 3.5 — Cập nhật accuracy values dựa trên backtest thực

**File:** Sau khi có backtest, cập nhật `GetAccuracy()` trong mỗi algo:

```go
// Thay hardcode:
func (m *MovingAveragePredictor) GetAccuracy() float64 { return 0.75 }

// Bằng dynamic accuracy từ backtest:
type MovingAveragePredictor struct {
    ...
    backtestAccuracy float64
}

func (m *MovingAveragePredictor) GetAccuracy() float64 {
    if m.backtestAccuracy == 0 {
        return 0.0 // unknown until backtest runs
    }
    return m.backtestAccuracy
}
```

---

## Checklist Phase 3

- [ ] 3.1 Sửa Moving Average VWMA dùng real volume từ `data.Volume`
- [ ] 3.2 Sửa ARIMA fitAR() dùng OLS gradient descent, fitGARCH() dùng moment matching
- [ ] 3.3 Thay LSTM mock bằng Ridge Regression với proper training
- [ ] 3.4 Tạo `pkg/service/predict/backtest/backtest.go` + endpoint
- [ ] 3.5 Cập nhật `GetAccuracy()` dùng backtest results thay vì hardcode

**Thời gian ước tính:** 2–3 ngày (phần lớn là 3.3 và 3.4)
**Rủi ro nếu bỏ qua:** Predictions không có giá trị thực, accuracy claims là misleading
