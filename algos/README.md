# Thuật toán dự đoán giá

Hệ thống chạy **11 thuật toán** song song cho mỗi market, kết quả được lưu vào DB và so sánh với giá thực tế thông qua `direction_correct`.

## Danh sách

| # | Key | Tên hiển thị | File | Min data | Có train() |
|---|-----|-------------|------|----------|------------|
| 1 | `moving_average` | Volume-Weighted Moving Average | [01_moving_average.md](01_moving_average.md) | 20 points | Không |
| 2 | `ema` | Exponential Moving Average | [02_ema_macd.md](02_ema_macd.md) | 26 points | Không |
| 3 | `lstm_nn` | LSTM Neural Network | [03_lstm.md](03_lstm.md) | 70 points | Có |
| 4 | `gru_nn` | GRU Neural Network | [04_gru.md](04_gru.md) | 70 points | Có |
| 5 | `arima_garch` | ARIMA-GARCH | [05_arima_garch.md](05_arima_garch.md) | 50 points | Không |
| 6 | `egarch` | EGARCH | [06_egarch.md](06_egarch.md) | 60 points | Không |
| 7 | `sarima` | SARIMA | [07_sarima.md](07_sarima.md) | 60 points | Không |
| 8 | `lightgbm` | LightGBM | [08_lightgbm.md](08_lightgbm.md) | 80 points | Có |
| 9 | `xgboost` | XGBoost | [09_xgboost.md](09_xgboost.md) | 80 points | Có |
| 10 | `random_forest` | Random Forest | [10_random_forest.md](10_random_forest.md) | 80 points | Có |
| 11 | `ensemble` | Ensemble | [11_ensemble.md](11_ensemble.md) | — | Không |

Tài liệu kỹ thuật chung về feature engineering dùng bởi LightGBM / XGBoost / Random Forest: [features.md](features.md)

## Kiến trúc chung

Tất cả thuật toán implement abstract class `PredictionAlgorithm` trong [prediction/src/algorithms/base.py](../prediction/src/algorithms/base.py):

```python
class PredictionAlgorithm(ABC):
    _market_key: str = ""          # được registry set trước khi gọi predict()

    def predict(prices, volumes) -> PredictionResult: ...
    def train(prices, volumes): ...          # no-op nếu stateless
    def train_batch(series): ...             # train trên nhiều symbol cùng lúc
    def is_trained() -> bool: ...
```

`PredictionResult` chứa: `predicted_price`, `confidence` (0–1), `current_price`, `algorithm_name`.

## Market-aware clamp

Mỗi thuật toán **bắt buộc** áp dụng giới hạn thay đổi giá theo market trước khi trả kết quả:

| Market | Giới hạn |
|--------|----------|
| GOLD | ±15% |
| SP500 | ±15% |
| NASDAQ100 | ±20% |
| CRYPTO | ±50% |

```python
max_change = current * get_max_change_pct(self._market_key)
predicted = max(current - max_change, min(current + max_change, predicted))
```

## Phân loại

**Kỹ thuật (Technical Analysis):** `moving_average`, `ema`

**Deep Learning (Sequence Modeling):** `lstm_nn`, `gru_nn`

**Time Series Thống kê:** `arima_garch`, `egarch`, `sarima`

**Tree-based ML:** `lightgbm`, `xgboost`, `random_forest`

**Meta:** `ensemble` (gộp 10 base algorithms)
