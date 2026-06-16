# Feature Engineering (Shared)

**File:** [prediction/src/algorithms/features.py](../prediction/src/algorithms/features.py)  
**Dùng bởi:** LightGBM, XGBoost, Random Forest

## Tóm tắt

Module cung cấp 2 hàm dùng chung cho các tree-based models:
- `build_basic_features()` — 14 features (backward compatible)
- `build_enhanced_features()` — ~30 features (primary, dùng pandas-ta khi có)

Cả hai trả `(features_list, targets_list)` — mỗi phần tử tương ứng với 1 ngày.

## build_basic_features — 14 features

| # | Feature | Mô tả |
|---|---------|-------|
| 1–10 | lag_1 … lag_10 | Log returns lagged 1–10 ngày |
| 11 | RSI(14) | Relative Strength Index |
| 12 | price/MA5 | Giá hiện tại / SMA 5 ngày |
| 13 | price/MA20 | Giá hiện tại / SMA 20 ngày |
| 14 | vol_ratio | Volume hiện tại / avg volume(10) |

Thuần numpy, không cần pandas-ta.

## build_enhanced_features — 30 features

### Group A — Lag returns (10 features)

```python
lag_1 = log(price[t] / price[t-1])
lag_2 = log(price[t-1] / price[t-2])
...
lag_10 = log(price[t-9] / price[t-10])
```

Capture short-term autocorrelation và mean-reversion.

### Group B — Moving average ratios (4 features)

```python
price / MA5    # short-term deviation
price / MA10
price / MA20   # medium-term trend
price / MA50   # long-term trend
```

Giá trị > 1: price trên MA (bullish). Giá trị < 1: price dưới MA (bearish).

### Group C — Multi-timeframe log returns (3 features)

```python
ret_5d  = log(price[t] / price[t-5])    # weekly return
ret_10d = log(price[t] / price[t-10])   # bi-weekly return
ret_20d = log(price[t] / price[t-20])   # monthly return
```

Capture momentum ở nhiều timeframes.

### Group D — RSI(14) (1 feature)

Wilder RSI chuẩn, dùng average gains/losses:
```python
RSI = 100 - 100 / (1 + avg_gain / avg_loss)
```
Vùng 70+: overbought. Vùng 30-: oversold.

### Group E — Stochastic RSI (2 features)

```python
StochRSI_K = (RSI - min_RSI_window) / (max_RSI_window - min_RSI_window)
StochRSI_D = SMA(3) of K
```

RSI của RSI — nhạy hơn RSI chuẩn, phát hiện momentum exhaustion sớm hơn.

Khi `pandas_ta` có: dùng `ta.stochrsi(length=14, rsi_length=14, k=3, d=3)`.  
Khi không có: approximate bằng numpy rolling RSI.

### Group F — Bollinger %B (1 feature)

```python
%B = (price - lower_band) / (upper_band - lower_band)
   = (price - (SMA - 2σ)) / (4σ)
```

Giá trị [0, 1] — nằm trong band. < 0 hoặc > 1: vượt ngoài band.

Khi `pandas_ta` có: `ta.bbands(length=20, std=2.0)`.

### Group G — MACD normalized (2 features)

```python
macd_line_norm = (EMA12 - EMA26) / price
macd_hist_norm = (MACD_line - Signal9) / price
```

Normalize theo price để so sánh được giữa tài sản có mức giá khác nhau.

### Group H — Rolling volatility (3 features)

```python
vol_5d  = std(log_returns[-5:])
vol_10d = std(log_returns[-10:])
vol_20d = std(log_returns[-20:])
```

Annualized thành daily std deviation của log returns. Capture volatility regime.

### Group I — Rate of Change (1 feature)

```python
ROC(10) = (price[t] - price[t-10]) / price[t-10] * 100
```

Percent change so với 10 ngày trước — momentum indicator.

### Group J — Momentum (2 features)

```python
momentum_5  = (price[t] - price[t-5]) / price[t-5]
momentum_10 = (price[t] - price[t-10]) / price[t-10]
```

Tương tự ROC nhưng ở dạng fraction và không nhân 100.

### Group K — Volume ratio (1 feature)

```python
vol_ratio = volume[t] / avg_volume(10)
```

Volume spike (> 1) thường đi kèm với price movement có ý nghĩa hơn.

## Target variable

```python
target = log(price[t+1] / price[t])   # next-day log return
```

Predict return thay vì price tuyệt đối giúp model generalizable hơn (không phụ thuộc price level).

## Minimum data

`MIN_DATA_POINTS = 80`. Loop bắt đầu từ `i=20` (cho MA50 và multi-timeframe returns).

## pandas-ta vs numpy fallback

| Indicator | pandas-ta | numpy fallback |
|-----------|-----------|----------------|
| RSI | `ta.rsi()` | Window mean gains/losses |
| StochRSI | `ta.stochrsi()` | Rolling RSI series approximation |
| Bollinger %B | `ta.bbands()` | `(price - SMA ± 2std) / 4std` |
| MACD | `ta.macd()` | EMA scalar computation per timestep |
| ROC | `ta.roc()` | `(p[t]-p[t-10])/p[t-10]*100` |

Kết quả numpy fallback xấp xỉ pandas-ta — đủ chính xác cho ML, không phải exactmatch.

## Xử lý NaN

Tất cả NaN từ pandas-ta được `fillna` với giá trị neutral trước khi dùng:

| Feature | NaN → |
|---------|-------|
| RSI | 50.0 |
| StochRSI K/D | 50.0 |
| Bollinger %B | 0.5 |
| MACD | 0.0 |
| ROC | 0.0 |

## Volume handling

Nếu `vol_arr=None` hoặc length không match: `vol_ratio = 1.0` (neutral).  
Markets không có volume (GOLD intraday) hoạt động bình thường với fallback này.
