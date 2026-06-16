# GRU Neural Network

**Key:** `gru_nn`  
**Class:** `GRUPredictor`  
**File:** [prediction/src/algorithms/gru.py](../prediction/src/algorithms/gru.py)  
**Stateful:** Hỗ trợ `train()` / `train_batch()` để cache model

## Tóm tắt

**Gated Recurrent Unit** (GRU) là biến thể đơn giản hóa của LSTM — bỏ cell state riêng, dùng 2 gates thay vì 3. Trong codebase này GRU và LSTM có hyperparameter và logic giống hệt nhau; sự khác biệt nằm thuần túy ở kiến trúc gates bên trong PyTorch.

## Kiến trúc model

```
Input:  (batch, seq_len=60, features=1)
        ↓
GRU(input=1, hidden=64, layers=2, dropout=0.2, batch_first=True)
        ↓
Linear(64 → 1)
        ↓
Output: predicted_price (normalized)
```

GRU forward pass dùng `output[:, -1, :]` (last timestep) giống LSTM — chỉ lấy hidden state của bước cuối.

## Hyperparameters

Giống LSTM hoàn toàn:

| Hyperparameter | Giá trị |
|---------------|---------|
| `SEQUENCE_LENGTH` | 60 |
| `HIDDEN_SIZE` | 64 |
| `NUM_LAYERS` | 2 |
| `DROPOUT` | 0.2 |
| `EPOCHS` | 50 |
| `LR` | 0.001 (Adam) |
| `MIN_DATA_POINTS` | 70 |

## GRU gates — hoạt động

GRU có 2 gates:

**Reset gate** `r_t`:
```
r_t = σ(W_r·[h_{t-1}, x_t])
```
Quyết định bao nhiêu hidden state cũ được "quên" khi tính candidate hidden state.

**Update gate** `z_t`:
```
z_t = σ(W_z·[h_{t-1}, x_t])
```
Quyết định bao nhiêu thông tin cũ giữ lại vs thông tin mới từ candidate.

**Output:**
```
h̃_t = tanh(W·[r_t ⊙ h_{t-1}, x_t])    # candidate
h_t = (1 - z_t) ⊙ h_{t-1} + z_t ⊙ h̃_t
```

Khi `z_t ≈ 0`: giữ nguyên hidden state cũ (bỏ qua input hiện tại).  
Khi `z_t ≈ 1`: thay toàn bộ bằng candidate mới.

## Luồng predict

Giống hệt LSTM:
```
predict()
    ├─ cached model → _inference() → confidence=0.70
    ├─ _train_and_predict() → confidence từ MSE loss
    └─ _ema_fallback() → confidence=0.40
```

## train_batch()

Ghép sequences từ nhiều symbols thành 1 dataset, train 1 model GRU chung. Log: `gru.batch_trained`.

## So sánh chi tiết với LSTM

| Thuộc tính | LSTM | GRU |
|-----------|------|-----|
| Cell state (`c_t`) | Có | Không |
| Gates | input, forget, output | reset, update |
| Số tham số | ~4× hidden² | ~3× hidden² |
| Với hidden=64, layers=2 | ~66K params | ~50K params |
| Gradient vanishing | Ít hơn vanilla RNN | Ít hơn vanilla RNN |
| Tốc độ (forward) | Chậm hơn ~15% | Nhanh hơn |
| Hiệu quả thực tế | Tương đương GRU | Tương đương LSTM |

Với bài toán dự đoán giá ngắn hạn (1 ngày tới) từ 60 ngày lookback, GRU thường cho kết quả tương đương LSTM với thời gian training ngắn hơn.

## Điểm mạnh / yếu

**Mạnh:**
- Nhanh hơn LSTM trong training (ít parameters hơn)
- Vẫn capture được dependency dài hạn nhờ update gate
- Cùng `train_batch()` — tận dụng multi-symbol data

**Yếu:**
- Cùng hạn chế với LSTM: chỉ dùng raw price, không dùng technical features
- Kiến trúc nhỏ — có thể không đủ capacity với market phức tạp
- Cần PyTorch — không chạy được nếu không cài torch

## Dependency

- `torch` (PyTorch) — bắt buộc; fallback về EMA nếu không có
- `numpy`
