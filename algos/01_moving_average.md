# Moving Average (VWMA)

**Key:** `moving_average`  
**Class:** `MovingAveragePredictor`  
**File:** [prediction/src/algorithms/moving_average.py](../prediction/src/algorithms/moving_average.py)  
**Stateless:** Không cần `train()`, predict trực tiếp từ giá

## Tóm tắt

Dùng **Volume-Weighted Moving Average** (VWMA) để tính độ dốc xu hướng, sau đó kết hợp với **RSI** để scale động lượng và **StochRSI** để điều chỉnh mean-reversion khi giá ở vùng overbought/oversold.

## Tham số cố định

| Tham số | Giá trị | Ý nghĩa |
|---------|---------|---------|
| `SHORT_PERIOD` | 5 | 1 tuần giao dịch |
| `LONG_PERIOD` | 20 | 1 tháng giao dịch |
| Slope window | 5 | Số bước để fit least-squares slope |
| RSI period | 14 | Chu kỳ RSI chuẩn |
| StochRSI period | 14 | Chu kỳ StochRSI |

## Dữ liệu tối thiểu

20 price points (bằng `LONG_PERIOD`).

## Logic dự đoán

### Bước 1 — Tính VWMA

Nếu có volume: `VWMA = Σ(price × vol) / Σ(vol)` trên window.  
Nếu không có volume: fallback về SMA thông thường.  
Volume bằng 0 được thay bằng 1 để tránh chia zero.

### Bước 2 — Đo độ dốc xu hướng

Xây dựng chuỗi VWMA rolling, lấy 5 điểm cuối và fit **least-squares** bậc 1 → thu được `slope` (đơn vị: giá/period).

### Bước 3 — Scale theo RSI

RSI đo động lượng thị trường:

| Vùng RSI | Hành vi |
|----------|---------|
| RSI > 60 (bullish) | `momentum_mult = 0.8 + (RSI-60)/40 * 0.8`, tối đa 1.4 |
| RSI < 40 (bearish) | Tương tự nhưng flip slope về âm, `mult` tối đa 1.4 |
| 40–60 (neutral) | `momentum_mult = 0.6` (dampen) |

```python
predicted = current + slope * momentum_mult
```

### Bước 4 — Điều chỉnh StochRSI (mean-reversion)

Nếu `pandas_ta` có sẵn, tính StochRSI %K(14,14,3,3):

| Điều kiện | Điều chỉnh |
|-----------|-----------|
| %K > 80 (overbought) | Kéo predicted về phía current: `predicted -= (predicted - current) * factor * 0.3` |
| %K < 20 (oversold) | Đẩy predicted lên phía current: `predicted += (current - predicted) * factor * 0.3` |

`factor` tỷ lệ thuận với mức độ extreme (0 → 1 trong khoảng 80–100 hoặc 0–20).

### Bước 5 — Market-aware clamp

```python
max_change = current * get_max_change_pct(self._market_key)
predicted = clamp(predicted, current - max_change, current + max_change)
```

## Tính confidence

Dựa trên mức độ phân kỳ giữa VWMA ngắn và dài:

```python
ma_diff_pct = |short_ma - long_ma| / long_ma
confidence = 0.40 + min(ma_diff_pct * 10, 0.45)  # clamp vào [0.30, 0.85]
```

Phân kỳ càng lớn → confidence càng cao (xu hướng rõ nét hơn).

## Điểm mạnh / yếu

**Mạnh:**
- Rất nhanh, không cần training
- Tích hợp volume — phân biệt được những ngày giao dịch nhiều vs ít
- StochRSI overlay giúp tránh overpredict ở vùng cực trị

**Yếu:**
- Thuần kỹ thuật, không học từ dữ liệu lịch sử
- Slope của 5 ngày có thể nhiễu cao với tài sản biến động mạnh (CRYPTO)
- Không nắm được pattern phi tuyến

## Dependency

- `numpy` (bắt buộc)
- `pandas`, `pandas_ta` (tùy chọn — cần cho StochRSI; fallback về `None` nếu vắng)
