# ARIMA-GARCH

**Key:** `arima_garch`  
**Class:** `ARIMAGARCHPredictor`  
**File:** [prediction/src/algorithms/arima_garch.py](../prediction/src/algorithms/arima_garch.py)  
**Stateless:** Không cache model, fit mới mỗi lần predict

## Tóm tắt

Kết hợp hai mô hình thống kê kinh điển:
- **ARIMA(2,1,2)** — dự đoán giá trị kỳ vọng (mean forecast)
- **GARCH(1,1)** — ước lượng biến động (volatility) để tính confidence

ARIMA xử lý cấu trúc tuyến tính trong chuỗi giá; GARCH xử lý **heteroskedasticity** — hiện tượng biến động giá thay đổi theo thời gian (volatility clustering).

## Dữ liệu tối thiểu

50 price points (đủ để ARIMA fit ổn định).  
GARCH cần thêm ít nhất 30 log returns — nếu không đủ thì dùng volatility default 2%.

## ARIMA(2,1,2) — hoạt động

**ARIMA(p, d, q):**
- `d=1`: differencing bậc 1 → biến chuỗi giá thành chuỗi returns (stationarity)
- `p=2`: AR(2) — dùng 2 lagged residuals để predict mean
- `q=2`: MA(2) — dùng 2 lagged error terms

Phương trình (sau differencing):
```
Δy_t = c + φ₁·Δy_{t-1} + φ₂·Δy_{t-2} + θ₁·ε_{t-1} + θ₂·ε_{t-2} + ε_t
```

Dự báo 1 bước tới: `forecast = arima_fit.forecast(steps=1)`.

## GARCH(1,1) — hoạt động

Fit trên chuỗi **log returns** (× 100 để scale về đơn vị %):
```python
log_returns = np.diff(np.log(prices)) * 100
```

**Phương trình variance:**
```
σ²_t = ω + α·ε²_{t-1} + β·σ²_{t-1}
```

- `ω` (omega): baseline variance
- `α`: ARCH effect — mức độ shock giá hôm qua ảnh hưởng volatility hôm nay
- `β`: GARCH effect — persistence của volatility

Variance forecast 1 bước → lấy căn → `volatility` (đơn vị %).

## Logic dự đoán

### Bước 1 — ARIMA fit & forecast

```python
arima_model = ARIMA(series, order=(2, 1, 2))
arima_fit = arima_model.fit(method_kwargs={"warn_convergence": False})
forecast = float(arima_fit.forecast(steps=1).iloc[0])
```

`warn_convergence=False` để suppress warning khi dữ liệu nhiễu.

### Bước 2 — Clamp

```python
max_change = current * get_max_change_pct(self._market_key)
forecast = clamp(forecast, current - max_change, current + max_change)
```

### Bước 3 — GARCH volatility cho confidence

```python
volatility = sqrt(variance_forecast) / 100   # về fraction
confidence = clamp(1.0 / (1.0 + volatility * 5), 0.3, 0.9)
```

Volatility cao → confidence thấp. Nếu GARCH fail: `volatility = 0.02` (2%), `confidence ≈ 0.91` → clamp về 0.9.

### Fallback

Nếu ARIMA hoặc cả pipeline fail: EMA(26) fallback với `confidence=0.38`.

## Volatility clustering

GARCH capture được hiện tượng trong thực tế: thị trường có xu hướng "biến động theo cụm" — sau giai đoạn yên tĩnh thường tiếp tục yên tĩnh, sau giai đoạn biến động lớn thường tiếp tục biến động. ARIMA thuần không nắm được điều này.

## Điểm mạnh / yếu

**Mạnh:**
- Nền tảng thống kê vững chắc, giải thích được
- GARCH confidence phản ánh trạng thái thị trường thực tế
- Không cần training trước — fit online tại thời điểm predict

**Yếu:**
- Chậm — mỗi predict phải fit ARIMA + GARCH (~100ms–1s tùy data)
- ARIMA giả định chuỗi tuyến tính, thực tế giá chứng khoán có pattern phi tuyến
- Không tận dụng technical features (volume, RSI, v.v.)
- `order=(2,1,2)` cố định — có thể không optimal cho mọi market

## Dependency

- `statsmodels` — ARIMA (bắt buộc)
- `arch` — GARCH(1,1) (tùy chọn; nếu thiếu thì dùng volatility default 2%)
- `numpy`, `pandas`
