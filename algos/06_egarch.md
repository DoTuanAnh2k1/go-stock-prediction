# EGARCH

**Key:** `egarch`  
**Class:** `EGARCHPredictor`  
**File:** [prediction/src/algorithms/egarch.py](../prediction/src/algorithms/egarch.py)  
**Stateless:** Fit mới mỗi lần predict

## Tóm tắt

**Exponential GARCH** (EGARCH) là mô hình volatility nâng cao hơn GARCH(1,1). Điểm khác biệt chính: EGARCH dùng **logarithm của variance** và capture được **leverage effect** — giá xuống thường tạo ra volatility lớn hơn giá lên cùng biên độ.

Model trong codebase: `EGARCH(p=1, o=1, q=1)` với **HARX mean component** (thay cho ARIMA).

## Dữ liệu tối thiểu

60 price points (nhiều hơn ARIMA-GARCH do HARX cần 22 lags).

## EGARCH — phương trình

**Variance equation (log scale):**
```
log(σ²_t) = ω + Σ αᵢ·g(z_{t-i}) + Σ βⱼ·log(σ²_{t-j})
```

Trong đó:
```
g(z_t) = θ·z_t + γ·(|z_t| - E[|z_t|])
```

- `θ` (asymmetry): capture leverage effect — `θ < 0` nghĩa là shock âm → volatility tăng mạnh hơn shock dương
- `γ` (magnitude): phản ứng với biên độ shock
- `z_t = ε_t / σ_t`: standardized residual

**Ưu điểm so với GARCH:** variance luôn dương (vì dùng log), không cần ràng buộc `α, β ≥ 0`.

## HARX mean component

HARX (Heterogeneous Autoregression with eXogenous variables) dùng **lags 1, 5, 22** của log returns:

```python
model = arch_model(
    log_returns,
    mean="HARX",
    lags=[1, 5, 22],   # daily, weekly, monthly
    vol="EGARCH",
    p=1, o=1, q=1,
    dist="normal",
)
```

Lý thuyết HAR: thị trường có participants với horizon khác nhau — trader ngắn hạn (lag 1), hedge fund tuần (lag 5), fund tháng (lag 22). Mỗi nhóm tạo ra contribution riêng vào price movement.

## Logic dự đoán

### Bước 1 — Chuyển về log returns

```python
log_returns = np.diff(np.log(prices)) * 100   # percent scale
```

Percent scale giúp optimizer hội tụ ổn định hơn.

### Bước 2 — Fit model

```python
model_fit = model.fit(disp="off", options={"maxiter": 300})
```

### Bước 3 — Forecast

```python
forecast = model_fit.forecast(horizon=1)
mean_forecast_pct = float(forecast.mean.iloc[-1, 0])
predicted_price = current * (1 + mean_forecast_pct / 100)
```

Lấy mean từ HARX component, chuyển từ % return về giá tuyệt đối.

### Bước 4 — Confidence từ variance

```python
forecast_variance = forecast.variance.iloc[-1, 0]
sigma = sqrt(forecast_variance)
confidence = clamp(1.0 / (1.0 + sigma / 100 * 3), 0.3, 0.9)
```

Nhân với 3 thay vì 5 (như ARIMA-GARCH) vì EGARCH sigma đã ở percent scale.

### Bước 5 — Clamp + fallback EMA

## Leverage effect trong thực tế

Thị trường chứng khoán và crypto thường có leverage effect rõ:
- Khi giá giảm mạnh → VIX (volatility index) tăng vọt
- Khi giá tăng tương đương → VIX tăng ít hơn

EGARCH với `θ < 0` capture được asymmetry này, GARCH(1,1) thì không.

## So sánh EGARCH vs ARIMA-GARCH

| Tiêu chí | ARIMA-GARCH | EGARCH |
|---------|-------------|--------|
| Mean model | ARIMA(2,1,2) | HARX(1,5,22) |
| Variance model | GARCH(1,1) | EGARCH(1,1,1) |
| Leverage effect | Không | Có |
| Non-negativity constraint | Cần (α,β ≥ 0) | Không cần (dùng log) |
| Min data | 50 | 60 |
| Tốc độ fit | Nhanh hơn | Chậm hơn (~2–3×) |

## Điểm mạnh / yếu

**Mạnh:**
- Capture leverage effect — quan trọng cho crypto và stock downturns
- HARX nhận diện được nhịp daily/weekly/monthly của thị trường
- Không cần ràng buộc params phi âm

**Yếu:**
- Chậm nhất trong nhóm thống kê (~1–3s mỗi predict)
- Hội tụ không ổn định với data ngắn hoặc nhiều outliers
- `maxiter=300` có thể không đủ — fallback EMA xảy ra thường hơn ARIMA-GARCH

## Dependency

- `arch` — EGARCH (`arch.univariate.arch_model`) — bắt buộc; fallback EMA nếu thiếu
- `numpy`
