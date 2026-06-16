# Random Forest

**Key:** `random_forest`  
**Class:** `RandomForestPredictor`  
**File:** [prediction/src/algorithms/random_forest.py](../prediction/src/algorithms/random_forest.py)  
**Stateful:** Hỗ trợ `train()` / `train_batch()` để cache model

## Tóm tắt

**Random Forest** là ensemble của nhiều decision trees độc lập. Mỗi tree train trên bootstrap sample và chỉ xem xét random subset of features tại mỗi split — giúp giảm correlation giữa các trees và tăng tính đa dạng. Trong codebase, Random Forest là thuật toán đơn giản nhất trong nhóm tree-based — **không dùng Optuna**, hyperparameters cố định.

## Dữ liệu tối thiểu

80 price points (`MIN_DATA_POINTS`).

## Hyperparameters (cố định)

```python
RandomForestRegressor(
    n_estimators=200,    # số trees
    max_depth=8,         # độ sâu tối đa mỗi tree
    min_samples_leaf=5,  # node phải có ít nhất 5 samples
    max_features="sqrt", # √(n_features) features tại mỗi split
    n_jobs=-1,           # dùng tất cả CPU cores
    random_state=42,
)
```

Không có Optuna — hyperparameters cố định cho mọi market và mọi lần train.

## Features

Giống LightGBM và XGBoost — dùng `build_enhanced_features()` từ [features.py](../prediction/src/algorithms/features.py). ~30 features. Xem [features.md](features.md).

## Cơ chế Random Forest

**Bagging (Bootstrap Aggregating):**
1. Tạo 200 bootstrap samples từ training data (sample có replacement)
2. Train 1 decision tree trên mỗi bootstrap sample
3. Prediction = mean của 200 trees

**Feature randomness:**
- Tại mỗi split, chỉ xem xét `sqrt(30) ≈ 5–6` features (thay vì toàn bộ 30)
- Buộc các trees phải học từ các features khác nhau → ít correlation hơn
- Kết quả: variance thấp, ít overfit hơn single tree

## So sánh với Boosting (LightGBM/XGBoost)

| | Random Forest | LightGBM/XGBoost |
|--|--------------|-----------------|
| Cơ chế | Bagging (parallel trees) | Boosting (sequential trees) |
| Trees học | Độc lập nhau | Mỗi tree sửa lỗi tree trước |
| Bias | Cao hơn | Thấp hơn |
| Variance | Thấp hơn | Cao hơn (cần regularization) |
| Overfit risk | Thấp | Cao hơn (cần early stopping) |
| Hyperparameter | Ít nhạy | Rất nhạy |
| Tốc độ train | Nhanh (parallel) | Chậm hơn (sequential) |
| Tốc độ predict | Chậm (200 trees) | Nhanh hơn |

## Train pipeline

```python
# Không có Optuna — cố định params
model = RandomForestRegressor(
    n_estimators=200,
    max_depth=8,
    min_samples_leaf=5,
    max_features="sqrt",
    n_jobs=-1,
    random_state=42,
)
model.fit(X_train, y_train)
```

Khác với LightGBM/XGBoost: **không có train/val split** và **không có early stopping**. Dùng toàn bộ `features[:-1]` để train.

## Luồng predict

```
predict()
    ├─ cached model → _inference()
    ├─ _train_and_predict()
    └─ _ema_fallback() → confidence=0.35
```

## train_batch()

Giống LightGBM/XGBoost — ghép features/targets từ nhiều symbols, train 1 model chung.

## Tính confidence

```python
confidence = clamp(0.5 + abs(pred_return) * 5, 0.3, 0.9)
```

Giống LightGBM và XGBoost — confidence tỉ lệ với magnitude của predicted return.

## max_depth=8 và min_samples_leaf=5

**max_depth=8:** Mỗi tree có tối đa 2⁸ = 256 leaf nodes. Đủ complex để capture nonlinear patterns, không quá sâu để overfit.

**min_samples_leaf=5:** Node không split nếu dưới 5 samples — tránh trees quá granular, đặc biệt quan trọng khi data nhỏ (~80–200 points).

## Điểm mạnh / yếu

**Mạnh:**
- Rất ổn định — không cần Optuna, ít hyperparameter nhạy
- Tự nhiên xử lý nonlinearity và feature interaction
- OOB (out-of-bag) estimation — mỗi tree tự có validation set miễn phí
- Robust với noisy features — `max_features="sqrt"` tự lọc
- Không cần early stopping

**Yếu:**
- Chậm hơn LightGBM trong cả train lẫn predict (200 trees × full depth)
- Confidence không có ý nghĩa thống kê — giống gradient boosting
- Không adaptive — hyperparameters cố định bất kể market hay data size
- Cần `n_jobs=-1` để tận dụng parallel; với 1 CPU thì rất chậm

## Dependency

- `scikit-learn` — `RandomForestRegressor` — bắt buộc; fallback EMA nếu thiếu
- `numpy`; `pandas`, `pandas_ta` — dùng bởi `build_enhanced_features()`
