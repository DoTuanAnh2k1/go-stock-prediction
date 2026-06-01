# Plan: Thêm 5 Thuật Toán Dự Đoán Mới

**Thuật toán:** SARIMA, EGARCH, GRU, Random Forest, XGBoost  
**Phạm vi:** Python Prediction Service · Go API Backend · React Frontend

---

## Tổng quan

| Thuật toán | Key | Library | Library status |
|---|---|---|---|
| SARIMA | `sarima` | `statsmodels` | Đã có trong `[ml]` extras |
| EGARCH | `egarch` | `arch` | Đã có trong `[ml]` extras |
| GRU Neural Network | `gru_nn` | `torch` | Đã có trong `[ml]` extras |
| Random Forest | `random_forest` | `scikit-learn` | Đã có trong base deps |
| XGBoost | `xgboost` | `xgboost` | **Cần thêm** vào `[ml]` |

Nguyên tắc:
- Không thay đổi gRPC proto, không thay đổi DB schema
- Mỗi thuật toán implement đúng abstract class `PredictionAlgorithm`
- Ensemble tự động include tất cả 5 thuật toán mới (equal-weight)
- Frontend không cần refactor — chỉ thêm entries vào constant maps

---

## Thứ tự thực hiện

```
Bước 1: Dependencies (pyproject.toml + Dockerfile)
Bước 2: random_forest.py           ← scikit-learn đã có, test ngay được
Bước 3: xgboost_model.py           ← copy feature engineering từ RF
Bước 4: sarima.py                  ← statsmodels, pattern gần ARIMA
Bước 5: egarch.py                  ← arch, phức tạp nhất nhóm stats
Bước 6: gru.py                     ← torch, copy LSTM thay 1 class
Bước 7: registry.py (Python)       ← sau khi tất cả algo pass test
Bước 8: algorithms.go (Go)         ← metadata registry
Bước 9: api/index.ts (Frontend)    ← ALGO_FALLBACK + ALGO_COLORS
```

---

## Bước 1: Dependencies

### `prediction/pyproject.toml`

Thêm `xgboost>=2.0.0` vào `[ml]` extras:

```toml
ml = [
    "torch>=2.5.0",
    "statsmodels>=0.14.0",
    "arch>=7.0.0",
    "lightgbm>=4.5.0",
    "xgboost>=2.0.0",    # thêm dòng này
]
```

### `prediction/Dockerfile`

Tìm dòng cài dependencies, đổi từ:

```dockerfile
RUN pip install --no-cache-dir -e ".[dev]"
```

thành:

```dockerfile
RUN pip install --no-cache-dir -e ".[dev,ml]"
```

> **Lý do:** Container production cần cài đủ torch/statsmodels/arch/lightgbm/xgboost. Hiện tại `.[dev]` bỏ qua toàn bộ ML extras.

---

## Bước 2: Random Forest

### Tạo `prediction/src/algorithms/random_forest.py`

**Hyperparameters:**
- `n_estimators=200` — cân bằng tốc độ/accuracy
- `max_depth=8` — tránh overfit với dataset nhỏ (train per-inference)
- `min_samples_leaf=5` — regularization
- `max_features="sqrt"` — default tốt cho regression
- `n_jobs=-1` — tận dụng tất cả CPU cores
- `random_state=42`

**Feature engineering (14 features, y hệt LightGBM):**
- Lag returns 1–10: `returns[t-1], returns[t-2], ..., returns[t-10]`
- RSI(14): relative strength index
- MA5 ratio: `price / MA(5) - 1`
- MA20 ratio: `price / MA(20) - 1`
- Volume ratio: `vol / MA_vol(20)`

**Target:** Next-day log return (regression)

**Các constant:**
- `MIN_DATA_POINTS = 80`
- Clamp predicted return: `±7%`
- Fallback confidence: `0.35`

**Input ordering:** `prices` vào là ASC (oldest first) — không cần reverse. `_build_features` build forward theo thứ tự thời gian.

**Confidence formula:** `max(0.3, min(0.9, 0.5 + abs(pred_return) * 5))`

**Pattern tham khảo:** `prediction/src/algorithms/lightgbm_model.py`

### Tạo `prediction/tests/unit/test_random_forest.py`

Không cần `pytest.importorskip` (scikit-learn trong base deps). Test cases:
- `test_predict_returns_valid_price` — giá ra phải > 0
- `test_predict_insufficient_data` — ít hơn MIN_DATA_POINTS → fallback không throw
- `test_predict_confidence_range` — confidence trong [0.0, 1.0]
- `test_get_name` — trả đúng `"random_forest"`

---

## Bước 3: XGBoost

### Tạo `prediction/src/algorithms/xgboost_model.py`

**Hyperparameters:**
- `n_estimators=200`
- `learning_rate=0.05`
- `max_depth=5` — shallower hơn default (6) để tránh overfit
- `subsample=0.8`
- `colsample_bytree=0.8`
- `min_child_weight=5` — tương đương `min_samples_leaf`
- `random_state=42`
- `verbosity=0` — tắt stdout noise
- `early_stopping_rounds=10` — như LightGBM, cần `eval_set` trong `fit()`

**Feature engineering:** Identical với Random Forest (14 features). Copy `_build_features` inline — không import chéo để tránh circular dependency risk.

**Key:** `"xgboost"` (method `get_key()` trả `"xgboost"`, không phải `"xgboost_model"`)

**Các constant, clamp, confidence formula:** Giống Random Forest.

**Pattern tham khảo:** `prediction/src/algorithms/lightgbm_model.py` (cùng pattern `early_stopping_rounds` với `eval_set`)

### Tạo `prediction/tests/unit/test_xgboost_model.py`

```python
pytest.importorskip("xgboost")
```

Test cases tương tự Random Forest.

---

## Bước 4: SARIMA

### Tạo `prediction/src/algorithms/sarima.py`

**Hyperparameters:**
- ARIMA order: `(1, 1, 1)` — giảm bậc so với ARIMA(2,1,2) hiện tại để seasonal component hội tụ tốt hơn
- Seasonal order: `(1, 0, 1, 5)` — period=5 (tuần giao dịch VN, 5 ngày làm việc)
- `enforce_stationarity=False, enforce_invertibility=False` — tránh convergence error trên dữ liệu nhiễu

**Fit options:**
```python
model.fit(disp=False, method='lbfgs', maxiter=200)
```

**Confidence:** Dùng forecast standard error:
```python
stderr = forecast_summary.summary_frame()["mean_se"].iloc[0]
confidence = max(0.3, min(0.9, 1.0 / (1.0 + stderr / current_price * 10)))
```

**Fallback confidence:** `0.37`

**MIN_DATA_POINTS:** `60` — SARIMA cần nhiều hơn ARIMA để ước lượng seasonal component.

**Clamp:** `±7%`

**Fallback:** EMA (copy pattern `_ema_fallback` từ `arima_garch.py`)

**Pattern tham khảo:** `prediction/src/algorithms/arima_garch.py`

### Tạo `prediction/tests/unit/test_sarima.py`

```python
pytest.importorskip("statsmodels")
```

Test cases:
- `test_predict_returns_valid_price`
- `test_predict_insufficient_data` — ít hơn 60 điểm → fallback EMA, không throw
- `test_predict_confidence_range`
- `test_get_name` — trả `"sarima"`

---

## Bước 5: EGARCH

### Tạo `prediction/src/algorithms/egarch.py`

**Hyperparameters:**
- Mean model: `HARX(lags=[1, 5, 22])` — HAR (Heterogeneous Autoregression), bắt chước long-memory volatility của stock VN qua 3 horizon: ngày, tuần, tháng
- Vol model: `EGARCH(p=1, o=1, q=1)` — asymmetric GARCH, `o=1` bắt leverage effect (bad news → volatility tăng mạnh hơn good news cùng magnitude)
- Distribution: `"normal"`

**Lưu ý scale:** `arch` package yêu cầu log returns nhân 100 (percent scale) để tránh numerical issue — giống `arima_garch.py` dòng hiện tại:
```python
log_returns = np.diff(np.log(arr)) * 100
```

**Fit options:**
```python
model.fit(disp="off", options={"maxiter": 300})
```

**Dự báo mean:** Từ `HAR` component — mean prediction + current price.

**Confidence:** Dựa vào conditional volatility dự báo:
```python
forecast_variance = model_fit.forecast(horizon=1).variance.iloc[-1, 0]
sigma = np.sqrt(forecast_variance)  # sigma in % units
confidence = max(0.3, min(0.9, 1.0 / (1.0 + sigma / 100 * 3)))
```

**Fallback:** EMA nếu fit không converge (bắt `Exception`)

**Fallback confidence:** `0.36`

**MIN_DATA_POINTS:** `60`

**Clamp:** `±7%`

**Pattern tham khảo:** `prediction/src/algorithms/arima_garch.py` (phần GARCH)

### Tạo `prediction/tests/unit/test_egarch.py`

```python
pytest.importorskip("statsmodels")
pytest.importorskip("arch")
```

Test cases tương tự SARIMA, thêm:
- `test_predict_convergence_failure_uses_fallback` — mock fit raise exception → fallback không throw

---

## Bước 6: GRU

### Tạo `prediction/src/algorithms/gru.py`

**Khác biệt so với LSTM (`lstm.py`) — chỉ 3 điểm:**

1. Đổi class name: `GRUPredictor` (thay vì `LSTMPredictor`)
2. Đổi `nn.LSTM` → `nn.GRU` trong model definition
3. Bỏ unpack cell state: LSTM trả `(output, (h_n, c_n))`, GRU chỉ trả `(output, h_n)` — sửa forward pass

**Architecture:**
```
Input(1) → GRU(hidden=64, layers=2, batch_first=True, dropout=0.2) → Linear(64→1)
```

Lấy hidden state timestep cuối: `output[:, -1, :]` (giống LSTM).

**Hyperparameters (giống LSTM):**
- `SEQUENCE_LENGTH = 60`
- `HIDDEN_SIZE = 64`
- `NUM_LAYERS = 2`
- `DROPOUT = 0.2`
- `EPOCHS = 50`
- `LR = 0.001`
- `MIN_DATA_POINTS = 70` (= SEQUENCE_LENGTH + 10)

**Key:** `"gru_nn"` — naming consistent với `"lstm_nn"`

**Clamp:** `±7%`

**Pattern tham khảo:** `prediction/src/algorithms/lstm.py` — copy toàn bộ, chỉ sửa 3 điểm trên

### Tạo `prediction/tests/unit/test_gru.py`

```python
pytest.importorskip("torch")
```

Test cases tương tự `test_lstm.py`.

---

## Bước 7: Cập nhật Python Registry

### Sửa `prediction/src/algorithms/registry.py`

**Thêm 5 import ở đầu file** (sau các import hiện có):

```python
from src.algorithms.sarima import SARIMAPredictor
from src.algorithms.egarch import EGARCHPredictor
from src.algorithms.gru import GRUPredictor
from src.algorithms.random_forest import RandomForestPredictor
from src.algorithms.xgboost_model import XGBoostPredictor
```

**Trong `build_algorithms()`, thêm 5 instance** (trước dòng tạo Ensemble):

```python
sarima = SARIMAPredictor()
egarch = EGARCHPredictor()
gru    = GRUPredictor()
rf     = RandomForestPredictor()
xgb    = XGBoostPredictor()
```

**Update Ensemble constructor** để include tất cả 10 base models:

```python
ensemble = EnsemblePredictor([ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb])
```

**Thêm 5 key vào return dict:**

```python
sarima.get_key():  sarima,
egarch.get_key():  egarch,
gru.get_key():     gru,
rf.get_key():      rf,
xgb.get_key():     xgb,
```

> **Lưu ý:** `EnsemblePredictor.predict()` đã có `try/except` per-algorithm — nếu một algo fail, nó bị skip gracefully. Không cần sửa logic Ensemble.

---

## Bước 8: Cập nhật Go Metadata Registry

### Sửa `pkg/service/predict/registry/algorithms.go`

Thêm 5 `Register()` call vào `init()` **trước** dòng register `ensemble`:

```go
Register(AlgorithmDef{
    Key:         "sarima",
    DisplayName: "SARIMA",
    Config: map[string]interface{}{
        "order": "1,1,1", "seasonal_order": "1,0,1,5", "seasonal_period": 5,
    },
})
Register(AlgorithmDef{
    Key:         "egarch",
    DisplayName: "EGARCH",
    Config: map[string]interface{}{
        "p": 1, "o": 1, "q": 1, "mean_model": "HARX",
    },
})
Register(AlgorithmDef{
    Key:         "gru_nn",
    DisplayName: "GRU Neural Network",
    Config: map[string]interface{}{
        "epochs": 50, "learning_rate": 0.001, "hidden_layers": 2,
        "hidden_size": 64, "sequence_length": 60, "dropout": 0.2,
    },
})
Register(AlgorithmDef{
    Key:         "random_forest",
    DisplayName: "Random Forest",
    Config: map[string]interface{}{
        "n_estimators": 200, "max_depth": 8, "min_samples_leaf": 5,
    },
})
Register(AlgorithmDef{
    Key:         "xgboost",
    DisplayName: "XGBoost",
    Config: map[string]interface{}{
        "n_estimators": 200, "learning_rate": 0.05, "max_depth": 5,
        "subsample": 0.8, "colsample_bytree": 0.8,
    },
})
```

> **Tại sao cần file Go này?** API endpoint `GET /api/training/algorithms` đọc từ Go registry này để trả metadata (DisplayName, Config, last_trained, accuracy). Frontend gọi endpoint này khi load để override `ALGO_FALLBACK`.

---

## Bước 9: Cập nhật Frontend

### Sửa `frontend/src/api/index.ts`

**1. Thêm 5 entries vào `ALGO_FALLBACK`** (object bắt đầu dòng 49):

```typescript
sarima:        { short: 'SARM', cls: 'sarima', name: 'SARIMA' },
egarch:        { short: 'EGA',  cls: 'egarch', name: 'EGARCH' },
gru_nn:        { short: 'GRU',  cls: 'gru',    name: 'GRU Neural Network' },
gru:           { short: 'GRU',  cls: 'gru',    name: 'GRU Neural Network' },
random_forest: { short: 'RF',   cls: 'rf',     name: 'Random Forest' },
xgboost:       { short: 'XGB',  cls: 'xgb',    name: 'XGBoost' },
```

**2. Thêm 5 colors vào `ALGO_COLORS`** (array bắt đầu dòng 75):

```typescript
'oklch(0.72 0.18 50)',   // amber  (SARIMA)
'oklch(0.70 0.15 340)',  // rose   (EGARCH)
'oklch(0.74 0.16 220)',  // sky    (GRU)
'oklch(0.71 0.14 160)',  // emerald (Random Forest)
'oklch(0.73 0.17 280)',  // indigo (XGBoost)
```

**3. Thêm 5 keys vào `ALGO_ORDER`** (array dòng 85) — quyết định màu stable khi chart render:

```typescript
const ALGO_ORDER = [
  'lstm_nn', 'arima_garch', 'moving_average', 'ema', 'ensemble',
  'sarima', 'egarch', 'gru_nn', 'random_forest', 'xgboost',  // thêm dòng này
];
```

> **Tại sao:** `ALGO_COLORS` hiện có 7 màu cho 5 algo + 2 dự phòng. Sau khi thêm 5 algo mới (tổng 11 algo), cần thêm ít nhất 5 màu mới. `ALGO_ORDER` đảm bảo màu gán stable (không đổi mỗi lần reload) cho accuracy trend line chart.

> **Không cần sửa gì khác trên frontend:** `buildAlgos()`, `buildTrend()`, `algoMeta()` đều dynamic — chúng đọc từ API và fallback về `ALGO_FALLBACK`. Khi backend trả 11 algo, frontend tự render 11 series/cards/bars mà không cần thay đổi component logic.

---

## Danh sách file thay đổi tổng hợp

| File | Loại | Nội dung |
|---|---|---|
| `prediction/pyproject.toml` | Sửa | Thêm `xgboost>=2.0.0` vào `[ml]` |
| `prediction/Dockerfile` | Sửa | `.[dev]` → `.[dev,ml]` |
| `prediction/src/algorithms/random_forest.py` | Tạo | RF n=200, depth=8, 14 features |
| `prediction/src/algorithms/xgboost_model.py` | Tạo | XGB n=200, lr=0.05, depth=5 |
| `prediction/src/algorithms/sarima.py` | Tạo | SARIMA(1,1,1)(1,0,1,5,5) + EMA fallback |
| `prediction/src/algorithms/egarch.py` | Tạo | EGARCH(1,1,1) HARX mean + EMA fallback |
| `prediction/src/algorithms/gru.py` | Tạo | GRU 2-layer, seq=60, hidden=64 |
| `prediction/tests/unit/test_random_forest.py` | Tạo | Unit tests, không cần skip guard |
| `prediction/tests/unit/test_xgboost_model.py` | Tạo | Unit tests với `importorskip("xgboost")` |
| `prediction/tests/unit/test_sarima.py` | Tạo | Unit tests với `importorskip("statsmodels")` |
| `prediction/tests/unit/test_egarch.py` | Tạo | Unit tests với `importorskip("arch")` |
| `prediction/tests/unit/test_gru.py` | Tạo | Unit tests với `importorskip("torch")` |
| `prediction/src/algorithms/registry.py` | Sửa | 5 import + 5 instance + Ensemble update + 5 dict key |
| `pkg/service/predict/registry/algorithms.go` | Sửa | 5 `Register()` call mới |
| `frontend/src/api/index.ts` | Sửa | `ALGO_FALLBACK` + `ALGO_COLORS` + `ALGO_ORDER` |

**Tổng:** 5 file tạo mới (algorithms) + 5 file tạo mới (tests) + 5 file sửa = **15 file**

---

## Checklist khi done

- [ ] `docker-compose build prediction` thành công (xgboost install không lỗi)
- [ ] `docker exec prediction_service python -m pytest tests/unit/ -v` — tất cả pass (kể cả 5 test mới)
- [ ] `GET /api/training/algorithms` trả 11 algorithms (6 cũ + 5 mới)
- [ ] `POST /api/trigger/predict` chạy xong — DB có predictions từ `sarima`, `egarch`, `gru_nn`, `random_forest`, `xgboost`
- [ ] Frontend Training page hiển thị 11 thuật toán (cards + accuracy bars)
- [ ] Frontend Predictions accuracy trend chart có 11 series không bị trùng màu
