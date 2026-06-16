# EMA / MACD

**Key:** `ema`  
**Class:** `EMAMACDPredictor`  
**File:** [prediction/src/algorithms/ema_macd.py](../prediction/src/algorithms/ema_macd.py)  
**Stateless:** Không cần `train()`

## Tóm tắt

Kết hợp **EMA(12)** trend slope với **MACD(12,26,9)** momentum boost và **Bollinger %B** mean-reversion overlay. Là phiên bản "sắc nét" hơn so với VWMA — EMA phản ứng nhanh hơn SMA với dữ liệu mới nhất.

## Tham số cố định

| Tham số | Giá trị | Ý nghĩa |
|---------|---------|---------|
| `SHORT_PERIOD` | 12 | EMA nhanh |
| `LONG_PERIOD` | 26 | EMA chậm |
| `SIGNAL_PERIOD` | 9 | EMA của MACD line → signal line |
| Bollinger period | 20 | Window tính Bollinger Bands |
| Bollinger std | 2.0 | Số std dev cho upper/lower band |

## Dữ liệu tối thiểu

26 price points (bằng `LONG_PERIOD`).

## EMA — công thức

```
k = 2 / (period + 1)
EMA[0] = mean(prices[:period])
EMA[i] = price[i] * k + EMA[i-1] * (1 - k)
```

Với EMA(12): k ≈ 0.154 → giảm 15.4% mỗi period.  
Với EMA(26): k ≈ 0.074 → chậm hơn, phản ứng ít hơn.

## MACD — cấu trúc

```
MACD line     = EMA(12) - EMA(26)
Signal line   = EMA(9) của MACD line
Histogram     = MACD line - Signal line
```

- Histogram dương → EMA nhanh đang tách khỏi EMA chậm theo hướng lên → bullish momentum
- Histogram âm → bearish momentum

## Logic dự đoán

### Bước 1 — EMA slope

Lấy 4 điểm cuối của chuỗi EMA(12), fit least-squares bậc 1 → `ema_slope`.

### Bước 2 — MACD momentum boost

```python
macd_momentum = (macd_line - signal_line) / current * current * 0.5
predicted = current + ema_slope + macd_momentum
```

Khi MACD > signal (histogram dương), momentum boost đẩy prediction lên trên mức chỉ dự đoán từ slope thuần. Ngược lại kéo xuống.

### Bước 3 — Bollinger %B overlay (mean-reversion)

Bollinger %B = `(price - lower_band) / (upper_band - lower_band)`:
- %B = 1.0 → price nằm đúng trên upper band
- %B = 0.0 → price nằm đúng trên lower band
- %B = 0.5 → price ở giữa band

| %B | Điều chỉnh |
|----|-----------|
| > 0.9 (gần upper band) | Bearish nudge: `predicted -= (predicted - current) * factor * 0.2` |
| < 0.1 (gần lower band) | Bullish nudge: `predicted += (current - predicted) * factor * 0.2` |

`factor` tỉ lệ với mức độ extreme trong khoảng [0,1].  
Cường độ nhẹ (0.2) — chỉ điều chỉnh nhẹ, không override xu hướng chính.

### Bước 4 — Market-aware clamp

```python
max_change = current * get_max_change_pct(self._market_key)
predicted = clamp(predicted, current - max_change, current + max_change)
```

## Tính confidence

Dựa trên tỷ lệ histogram / MACD line:

```python
strength = min(1.0, |histogram| / |macd_line| * 2 + 0.5)
confidence = max(0.30, min(0.85, strength * 0.75))
```

MACD histogram lớn tương đối so với MACD line → momentum rõ → confidence cao.

## So sánh với Moving Average

| | VWMA | EMA/MACD |
|--|------|----------|
| Reacts to recent data | Chậm hơn (SMA) | Nhanh hơn (EMA) |
| Momentum signal | RSI | MACD histogram |
| Overbought/oversold | StochRSI | Bollinger %B |
| Cần volume | Tùy chọn (có thì tốt hơn) | Không cần |

## Điểm mạnh / yếu

**Mạnh:**
- MACD là indicator kinh điển, được kiểm chứng rộng rãi
- Bollinger %B phù hợp cho tài sản dao động trong range
- Không cần volume — tốt cho markets thiếu data volume chất lượng

**Yếu:**
- EMA lag — vẫn có độ trễ dù nhạy hơn SMA
- MACD momentum có thể quá mạnh trong trending market → prediction overshoot
- Không học từ lịch sử như ML models

## Dependency

- `numpy` (bắt buộc)
- `pandas`, `pandas_ta` (tùy chọn — dùng cho Bollinger %B chính xác; fallback numpy nếu vắng)
