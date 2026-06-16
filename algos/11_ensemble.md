# Ensemble

**Key:** `ensemble`  
**Class:** `EnsemblePredictor`  
**File:** [prediction/src/algorithms/ensemble.py](../prediction/src/algorithms/ensemble.py)  
**Stateless:** Không train, gọi lại các base algorithms

## Tóm tắt

**Equal-weight ensemble** của 10 base algorithms. Mỗi algorithm predict độc lập; kết quả cuối là **trung bình số học** của tất cả predictions thành công. Thuật toán nào fail sẽ bị bỏ qua — ensemble vẫn chạy tiếp với tập còn lại.

## Các base algorithms (10)

```python
# Thứ tự trong registry.py
bases = [
    MovingAveragePredictor(),   # moving_average
    EMAMACDPredictor(),         # ema
    LSTMPredictor(),            # lstm_nn
    ARIMAGARCHPredictor(),      # arima_garch
    LightGBMPredictor(),        # lightgbm
    SARIMAPredictor(),          # sarima
    EGARCHPredictor(),          # egarch
    GRUPredictor(),             # gru_nn
    RandomForestPredictor(),    # random_forest
    XGBoostPredictor(),         # xgboost
]
```

Ensemble nhận các instances này và gọi `predict()` trên từng instance.

## Logic

```python
def predict(self, prices, volumes):
    successful = []
    for algo in self._bases:
        try:
            result = algo.predict(prices, volumes)
            successful.append(result)
        except Exception:
            pass   # bỏ qua, tiếp tục với algo tiếp theo

    if not successful:
        raise ValueError("All base algorithms failed")

    avg_price = sum(r.predicted_price for r in successful) / len(successful)
    avg_conf  = sum(r.confidence for r in successful) / len(successful)
    return PredictionResult(predicted_price=avg_price, confidence=avg_conf, ...)
```

## Equal weight — lý do chọn

**Weighted averaging** (theo confidence hoặc historical accuracy) phức tạp hơn và có thể làm tệ đi vì:
1. Confidence của mỗi thuật toán được tính theo cách khác nhau → không comparable
2. Historical accuracy thay đổi theo market regime — weight từ quá khứ có thể không đúng hiện tại
3. Overfitting to historical performance dễ xảy ra với lượng data ít

Equal weight đơn giản và robust hơn khi không có ground truth để calibrate weights.

## Fault tolerance

| Tình huống | Hành vi |
|-----------|---------|
| 1 algo raise exception | Bỏ qua, tiếp tục |
| n algo fail (n < 10) | Ensemble với (10-n) algo còn lại |
| Tất cả fail | Raise `ValueError` — prediction service log lỗi |

Các trường hợp algo fail thường gặp:
- LSTM/GRU: PyTorch không có (train fail → EMA fallback không raise)
- ARIMA/EGARCH/SARIMA: data quá nhiễu, optimizer không hội tụ → EMA fallback (không raise, được tính vào ensemble)
- LightGBM/XGBoost/RF: không đủ feature rows → raise ValueError → bị bỏ qua

## Lợi thế của ensemble

**Variance reduction:** Mỗi thuật toán có sai số ngẫu nhiên khác nhau; khi trung bình lại, các sai số ngẫu nhiên triệt tiêu nhau, sai số có hệ thống giảm.

**Coverage:** Khi một nhóm thuật toán sai (ví dụ: các statistical models fail trong trending market), nhóm khác (technical, ML) vẫn contribute.

**Bias-variance tradeoff:** Individual models mạnh về bias thấp (LightGBM) hoặc variance thấp (RandomForest); ensemble hưởng lợi từ cả hai.

## Điểm mạnh / yếu

**Mạnh:**
- Thường có accuracy cao nhất trong tập 11 algorithms
- Tự động điều chỉnh khi một số algo fail
- Không cần training riêng

**Yếu:**
- Tốc độ: phải chạy tất cả 10 algorithms → chậm nhất
- Không thông minh hơn base algorithms — chỉ average, không học
- Nếu hầu hết base algorithms cùng sai (systemic error), ensemble cũng sai
- Confidence là average confidence của base → có thể misleading

## Vị trí trong registry

Ensemble phải được đăng ký **sau** tất cả base algorithms trong `registry.py` vì nó nhận các base instances:

```python
def build_algorithms(market_key: str) -> list[PredictionAlgorithm]:
    # Phase 1: tạo base algorithms
    ma = MovingAveragePredictor(); ...

    # Phase 2: tạo ensemble từ base instances
    ens = EnsemblePredictor([ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb])

    return [ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb, ens]
```

Ensemble và các base algorithms dùng chung `_market_key` — registry set key trên từng instance trước khi gọi.
