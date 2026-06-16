# XGBoost

**Key:** `xgboost`  
**Class:** `XGBoostPredictor`  
**File:** [prediction/src/algorithms/xgboost_model.py](../prediction/src/algorithms/xgboost_model.py)  
**Stateful:** Hỗ trợ `train()` / `train_batch()` để cache model

## Tóm tắt

**XGBoost** (eXtreme Gradient Boosting) là thuật toán tree-based sử dụng gradient boosting với regularization mạnh. Cùng feature set ~30 features với LightGBM, cùng target (next-day log return), cùng Optuna hyperparameter tuning. Điểm khác chính: **level-wise tree growth** thay vì leaf-wise.

## Dữ liệu tối thiểu

| Threshold | Giá trị |
|-----------|---------|
| `MIN_DATA_POINTS` | 80 |
| `HYPEROPT_MIN_POINTS` | 200 |

## Default hyperparameters

```python
{
    "n_estimators": 200,
    "learning_rate": 0.05,
    "max_depth": 5,
    "subsample": 0.8,
    "colsample_bytree": 0.8,
    "min_child_weight": 5,
    "random_state": 42,
    "verbosity": 0,
}
```

## Features

Giống LightGBM — dùng chung `build_enhanced_features()` từ [features.py](../prediction/src/algorithms/features.py). Xem [features.md](features.md) để biết chi tiết.

## Optuna search space

Khác LightGBM ở chỗ search `max_depth` và `min_child_weight` thay vì `num_leaves` và `min_data_in_leaf`:

| Parameter | Search space |
|-----------|-------------|
| `n_estimators` | int [50, 300] |
| `learning_rate` | log-uniform [0.01, 0.2] |
| `max_depth` | int [3, 10] |
| `subsample` | uniform [0.6, 1.0] |
| `colsample_bytree` | uniform [0.6, 1.0] |
| `min_child_weight` | int [1, 10] |

**30 trials, timeout 120 giây**, minimize MAE.

## Level-wise vs Leaf-wise tree growth

**XGBoost (level-wise):**
```
Level 0:        [root]
Level 1:       [L] [R]
Level 2:    [LL][LR][RL][RR]
```
Mỗi iteration split **tất cả leaf** ở level hiện tại → tree cân đối, ít overfit hơn với data nhỏ.

**LightGBM (leaf-wise):**
```
Chọn leaf có gain cao nhất → split tiếp
```
Tree không cân đối, gain tối đa nhanh hơn, nhưng dễ overfit hơn với data nhỏ.

## Train pipeline

```python
# Train/val split 80/20
X_tr, X_val = X[:split], X[split:]

# Optuna (nếu đủ data)
params = _tune_xgboost_params(X_tr, y_tr, X_val, y_val, n_data_points)

# Fit với early stopping
model = XGBRegressor(**params, early_stopping_rounds=10)
model.fit(X_tr, y_tr, eval_set=[(X_val, y_val)], verbose=False)
```

Early stopping dùng `eval_set` để dừng khi val loss không cải thiện sau 10 rounds.

## Luồng predict

Giống LightGBM:
```
predict()
    ├─ cached model → _inference()
    ├─ _train_and_predict()
    └─ _ema_fallback() → confidence=0.35
```

## Regularization trong XGBoost

XGBoost có regularization L1/L2 built-in:
- `reg_alpha` (L1): thưa hóa model, bỏ features ít quan trọng
- `reg_lambda` (L2): penalize magnitude của leaf weights
- `min_child_weight`: node không split nếu tổng Hessian < threshold

Default params không set `reg_alpha/lambda` → dùng XGBoost defaults (L2=1, L1=0). Optuna không search `reg_alpha/lambda` trong codebase này.

## So sánh chi tiết LightGBM vs XGBoost trong codebase

| Khía cạnh | LightGBM | XGBoost |
|----------|----------|---------|
| Tree growth | Leaf-wise | Level-wise |
| Default n_estimators | 100 | 200 |
| Default max_depth | — (dùng num_leaves) | 5 |
| Optuna: tree depth | `num_leaves` [8,64] | `max_depth` [3,10] |
| Regularization search | Không (codebase) | Không (codebase) |
| Early stopping | 10 rounds | 10 rounds |
| Tốc độ train | Nhanh hơn | Chậm hơn |
| Ổn định nhỏ data | Kém hơn | Tốt hơn |

## Điểm mạnh / yếu

**Mạnh:**
- Level-wise growth ổn định hơn với data ít hoặc nhiễu
- Regularization tốt — ít overfit hơn LightGBM khi data ngắn
- Optuna tự động tìm hyperparameters tốt nhất
- Được kiểm chứng trong nhiều competition ML tài chính

**Yếu:**
- Chậm hơn LightGBM (~2–3×)
- Cùng hạn chế: không capture temporal order, confidence đơn giản
- Cần XGBoost package — nếu thiếu thì EMA fallback

## Dependency

- `xgboost` — bắt buộc; fallback EMA nếu thiếu
- `optuna`, `sklearn` — tùy chọn
- `numpy`; `pandas`, `pandas_ta` — dùng bởi `build_enhanced_features()`
