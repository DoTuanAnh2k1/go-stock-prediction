# LSTM Neural Network

**Key:** `lstm_nn`  
**Class:** `LSTMPredictor`  
**File:** [prediction/src/algorithms/lstm.py](../prediction/src/algorithms/lstm.py)  
**Stateful:** Hỗ trợ `train()` / `train_batch()` để cache model

## Tóm tắt

**Long Short-Term Memory** (LSTM) là mạng neural hồi tiếp (RNN) với cơ chế gates giúp học các dependency dài hạn trong chuỗi thời gian. Mỗi bước giá trở thành một token trong sequence, model học pattern từ 60 ngày để dự đoán ngày tiếp theo.

## Kiến trúc model

```
Input:  (batch, seq_len=60, features=1)  — chuỗi giá normalized
        ↓
LSTM(input=1, hidden=64, layers=2, dropout=0.2, batch_first=True)
        ↓
Linear(64 → 1)
        ↓
Output: predicted_price (normalized)
```

| Hyperparameter | Giá trị |
|---------------|---------|
| `SEQUENCE_LENGTH` | 60 |
| `HIDDEN_SIZE` | 64 |
| `NUM_LAYERS` | 2 |
| `DROPOUT` | 0.2 |
| `EPOCHS` | 50 |
| `LR` | 0.001 (Adam) |
| `MIN_DATA_POINTS` | 70 (= 60 + 10) |

## Normalization

**MinMax normalization** trên toàn bộ price array trước khi feed vào model:

```python
arr_norm = (arr - p_min) / (p_max - p_min)
```

Sau khi model predict, denormalize về giá thực:

```python
predicted_price = pred_norm * (p_max - p_min) + p_min
```

Mỗi lần predict đều dùng p_min/p_max của batch hiện tại — không fix từ training.

## Luồng predict

```
predict() được gọi
    │
    ├─ model đã train (is_trained=True)?
    │       └─ Yes → _inference() — chỉ forward pass, không retrain
    │                   └─ Fail → fallthrough
    │
    ├─ _train_and_predict() — train fresh + predict ngay
    │       └─ Fail → fallthrough
    │
    └─ _ema_fallback() — EMA đơn giản, confidence=0.40
```

## train() vs train_batch()

**`train(prices, volumes)`** — train trên 1 symbol, cache model vào `self._model`. Dùng trong scheduled weekly training.

**`train_batch(series)`** — train trên nhiều symbol cùng lúc (dùng cho NASDAQ với 15 symbols, SP500 với 16 symbols). Tất cả sequences ghép lại thành 1 dataset lớn → 1 model chung. Lợi thế: model học được pattern chung từ nhiều tài sản cùng market.

## Xây dựng sequences

Từ N giá normalized, tạo ra `(N - seq_len)` cặp (X, y):
```
X[i] = arr_norm[i : i + seq_len]    # shape: (seq_len, 1)
y[i] = arr_norm[i + seq_len]        # giá tiếp theo
```

Dự đoán thực tế dùng `arr_norm[-seq_len:]` làm input cuối.

## Tính confidence

Khi dùng cached model: **confidence = 0.70** (fixed).

Khi train fresh:
```python
final_loss = MSE trên mẫu cuối
confidence = clamp(1.0 / (1.0 + sqrt(final_loss) * 10), 0.3, 0.9)
```

Loss nhỏ → confidence cao.

## Điểm mạnh / yếu

**Mạnh:**
- Có thể học dependency dài hạn (60 ngày lookback)
- Gates (input, forget, output) giúp lọc nhiễu tốt hơn RNN vanilla
- `train_batch()` tận dụng được nhiều data từ nhiều symbols

**Yếu:**
- Chậm hơn các thuật toán stateless khi không có cached model
- Với data ít (< 200 điểm) thường overfit
- Không dùng technical features — chỉ học từ raw price sequence
- 2 layers với hidden=64 là kiến trúc nhỏ, không đủ capacity cho pattern phức tạp

## So sánh với GRU

LSTM và GRU gần như giống hệt nhau trong codebase này (cùng hyperparameters, cùng logic). Sự khác biệt nằm ở kiến trúc gates:

| | LSTM | GRU |
|--|------|-----|
| Gates | 3 gates (input, forget, output) | 2 gates (reset, update) |
| Cell state | Có `c_t` riêng | Không có |
| Parameters | Nhiều hơn | Ít hơn (~75%) |
| Tốc độ train | Chậm hơn | Nhanh hơn |

## Dependency

- `torch` (PyTorch) — bắt buộc; fallback về EMA nếu không có
- `numpy`
