# SARIMA

**Key:** `sarima`  
**Class:** `SARIMAPredictor`  
**File:** [prediction/src/algorithms/sarima.py](../prediction/src/algorithms/sarima.py)  
**Stateless:** Fit mới mỗi lần predict

## Tóm tắt

**Seasonal ARIMA** (SARIMA) mở rộng ARIMA bằng cách thêm **seasonal component** — capture được pattern lặp lại theo chu kỳ. Trong codebase dùng seasonal period `s=5` (1 tuần giao dịch) để nắm bắt pattern thứ trong tuần.

Model: `SARIMAX(1,1,1)(1,0,1,5)` tức `SARIMA(p=1,d=1,q=1)(P=1,D=0,Q=1,s=5)`.

## Dữ liệu tối thiểu

60 price points.

## SARIMA — cấu trúc

**Non-seasonal part: ARIMA(1,1,1)**
```
(1 - φ₁L)(1 - L)y_t = (1 + θ₁L)ε_t
```
- `d=1`: differencing bậc 1 (stationarity)
- `p=1`: 1 AR lag
- `q=1`: 1 MA lag

**Seasonal part: (1,0,1)[5]**
```
(1 - Φ₁L⁵)(1 - L⁵)⁰y_t = (1 + Θ₁L⁵)ε_t
```
- `D=0`: không seasonal differencing
- `P=1`: 1 seasonal AR lag (giá cách đây 5 ngày)
- `Q=1`: 1 seasonal MA lag

**Kết hợp:**
```
(1 - φ₁L)(1 - Φ₁L⁵)(1 - L)y_t = (1 + θ₁L)(1 + Θ₁L⁵)ε_t
```

## Seasonal period s=5

`s=5` tương ứng với 5 ngày giao dịch (1 tuần). Điều này có nghĩa:
- Model dùng giá **cùng ngày tuần trước** làm seasonal predictor
- Thứ Hai tuần này phụ thuộc vào Thứ Hai tuần trước
- Pattern cuối tuần → đầu tuần được học

**Phù hợp nhất với:** NASDAQ, SP500 (có pattern rõ theo ngày trong tuần).  
**Kém phù hợp với:** GOLD và CRYPTO (giao dịch 7 ngày/tuần, không có seasonality theo tuần giao dịch).

## Logic dự đoán

### Fit model

```python
model = SARIMAX(
    series,
    order=(1, 1, 1),
    seasonal_order=(1, 0, 1, 5),
    enforce_stationarity=False,
    enforce_invertibility=False,
)
model_fit = model.fit(disp=False, method="lbfgs", maxiter=200)
```

`enforce_stationarity=False` và `enforce_invertibility=False` tránh fail khi data không đáp ứng điều kiện lý thuyết.

### Forecast

```python
forecast_summary = model_fit.get_forecast(steps=1)
forecast = float(forecast_summary.predicted_mean.iloc[0])
```

### Confidence từ standard error

```python
stderr = forecast_summary.summary_frame()["mean_se"].iloc[0]
confidence = clamp(1.0 / (1.0 + stderr / current * 10), 0.3, 0.9)
```

`stderr / current` chuẩn hóa về fraction — giúp confidence ổn định độc lập với mức giá tuyệt đối.

## So sánh với ARIMA-GARCH

| | ARIMA-GARCH | SARIMA |
|--|-------------|--------|
| Cấu trúc | ARIMA(2,1,2) + GARCH(1,1) | SARIMA(1,1,1)(1,0,1,5) |
| Seasonal | Không | Có (s=5) |
| Volatility model | Có (GARCH) | Không |
| Confidence source | GARCH variance | Forecast std error |
| Phù hợp nhất | Tất cả markets | NASDAQ/SP500 |

## Điểm mạnh / yếu

**Mạnh:**
- Capture pattern tuần → phù hợp market có trading calendar (NASDAQ, SP500)
- Đơn giản hơn EGARCH — ít tham số, hội tụ ổn hơn
- Standard error forecast có ý nghĩa thống kê rõ ràng

**Yếu:**
- Giả định seasonal period cố định là 5 — không linh hoạt
- Không capture volatility clustering (không có GARCH component)
- Chậm hơn kỹ thuật analysis (VWMA, EMA), ngang với ARIMA-GARCH
- `D=0` — không seasonal differencing có thể miss seasonal trend dài hạn

## Dependency

- `statsmodels` — SARIMAX — bắt buộc; fallback EMA nếu thiếu
- `numpy`, `pandas`
