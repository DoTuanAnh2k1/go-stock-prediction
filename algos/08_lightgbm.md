# LightGBM

**Key:** `lightgbm`  
**Class:** `LightGBMPredictor`  
**File:** [prediction/src/algorithms/lightgbm_model.py](../prediction/src/algorithms/lightgbm_model.py)  
**Stateful:** Hỗ trợ `train()` / `train_batch()` để cache model

## Tóm tắt

**LightGBM** (Light Gradient Boosting Machine) là thuật toán tree-based dùng **gradient boosting** với leaf-wise tree growth. Dự đoán **log return ngày tiếp theo** từ ~30 technical features. Hỗ trợ **Optuna hyperparameter tuning** khi có đủ dữ liệu.

## Dữ liệu tối thiểu

| Threshold | Giá trị | Ý nghĩa |
|-----------|---------|---------|
| `MIN_DATA_POINTS` | 80 | Tối thiểu để build features |
| `HYPEROPT_MIN_POINTS` | 200 | Tối thiểu để chạy Optuna |

## Default hyperparameters

```python
{
    "objective": "regression",
    "metric": "mae",
    "learning_rate": 0.05,
    "num_leaves": 15,
    "min_data_in_leaf": 5,
    "n_estimators": 100,
    "verbose": -1,
}
```

## Features (~30)

Xem chi tiết tại [features.md](features.md). Tóm tắt:

| Group | Features | Số lượng |
|-------|---------|---------|
| A — Lag returns | lag_1 … lag_10 | 10 |
| B — MA ratios | price/MA5, /MA10, /MA20, /MA50 | 4 |
| C — Multi-timeframe returns | 5d, 10d, 20d | 3 |
| D — RSI | RSI(14) | 1 |
| E — StochRSI | %K, %D | 2 |
| F — Bollinger %B | bb_pct_b | 1 |
| G — MACD | line_norm, hist_norm | 2 |
| H — Volatility | std_5d, std_10d, std_20d | 3 |
| I — ROC | ROC(10) | 1 |
| J — Momentum | mom_5, mom_10 | 2 |
| K — Volume ratio | vol_ratio | 1 |
| **Total** | | **30** |

## Target variable

```python
target = log_return[i] = log(price[i+1] / price[i])
```

Model học predict next-day log return. Sau đó chuyển về giá:
```python
predicted_price = current * (1 + pred_return)
```

## Luồng predict

```
predict()
    ├─ model cached → _inference() — build feature row cuối, model.predict()
    ├─ _train_and_predict() — train fresh + predict
    └─ _ema_fallback() → confidence=0.35
```

## Train pipeline

```python
# 1. Build features
X, y = build_enhanced_features(arr, vol_arr)

# 2. Train/val split (80/20)
X_tr, X_val = X[:split], X[split:]

# 3. Optuna tuning (nếu data >= 200)
params = _tune_lightgbm_params(X_tr, y_tr, X_val, y_val, n_data_points)

# 4. Fit với early stopping (10 rounds)
model = lgb.LGBMRegressor(**params)
model.fit(X_tr, y_tr, eval_set=[(X_val, y_val)],
          callbacks=[lgb.early_stopping(10), lgb.log_evaluation(-1)])
```

## Optuna hyperparameter search

Chỉ chạy khi `n_data_points >= 200` và `optuna` đã cài:

| Parameter | Search space |
|-----------|-------------|
| `learning_rate` | log-uniform [0.01, 0.2] |
| `num_leaves` | int [8, 64] |
| `min_data_in_leaf` | int [3, 30] |
| `n_estimators` | int [50, 300] |
| `subsample` | uniform [0.6, 1.0] |
| `colsample_bytree` | uniform [0.6, 1.0] |

**30 trials, timeout 120 giây**, minimize MAE trên validation set.

## train_batch()

Ghép features/targets từ nhiều symbols → 1 model chung. Dùng tổng `n_data_points` của tất cả series để quyết định có chạy Optuna không.

## Tính confidence

```python
confidence = clamp(0.5 + abs(pred_return) * 5, 0.3, 0.9)
```

Return lớn hơn → model "tin tưởng" hơn vào signal. Không phản ánh model uncertainty trực tiếp.

## LightGBM vs XGBoost

| | LightGBM | XGBoost |
|--|----------|---------|
| Tree growth | Leaf-wise | Level-wise |
| Tốc độ | Nhanh hơn (~2–3×) | Chậm hơn |
| Memory | Ít hơn | Nhiều hơn |
| Small data | Dễ overfit hơn | Ổn định hơn |
| Histogram bin | Tối ưu tự động | Cố định |

## Điểm mạnh / yếu

**Mạnh:**
- Rất nhanh khi có cached model (chỉ build features + 1 predict)
- Optuna tối ưu hyperparameters tự động
- 30 features kỹ thuật phong phú — nắm bắt nhiều signal
- Early stopping tránh overfit

**Yếu:**
- Không capture temporal order như LSTM/GRU — coi mỗi ngày là independent sample
- Confidence tính đơn giản, không phản ánh model uncertainty
- Cần `min_data_in_leaf` để tránh overfit với data ít
- Không có seasonal awareness (không như SARIMA)

## Dependency

- `lightgbm` — bắt buộc; fallback EMA nếu thiếu
- `optuna`, `sklearn` — tùy chọn (Optuna tuning)
- `numpy`; `pandas`, `pandas_ta` — dùng bởi `build_enhanced_features()`
