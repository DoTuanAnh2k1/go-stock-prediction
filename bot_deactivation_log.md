# Nhật ký loại bot trading (deactivate)

> Tạo ngày **2026-06-19** từ `GET /api/simulation/leaderboard` (440 bot).
> Mục đích: ghi lại bot nào bị tắt (`is_active=false`) và **vì sao**, để sau này quyết định bật lại có cơ sở.
>
> **Bật lại 1 bot:** `POST /api/simulation/bots/{id}/toggle` (cần JWT). `toggle` chỉ đảo trạng thái `is_active` — chạy lần nữa là active trở lại.

## Tiêu chí loại
- **Tier 1 (đã deactivate 88 bot):** `total_return_pct < 0` **VÀ** `profit_factor < 0.8` **VÀ** `total_trades ≥ 8` → đang giao dịch thật và thua có hệ thống.
- **Tier 2 (CHƯA deactivate — đề xuất, 44 bot):** thuộc market CRYPTO, return < −0.5%, PF ≤ 1 — lỗ nhưng ít lệnh nên chưa đạt ngưỡng Tier 1; gom để dọn cả market CRYPTO yếu.

Các con số (return / PF / win-rate / trades) là KPI tại thời điểm 2026-06-19; bot "ngủ" (trades≈0) **không** bị loại vì không gây lỗ.

---

## 🔴 Tier 1 — ĐÃ DEACTIVATE (88 bot)

| Bot ID | Market | Thuật toán | Return | PF | Win-rate | Trades | Lý do |
|---|---|---|---|---|---|---|---|
| `crypto_xgboost_v9` | CRYPTO | xgboost | -14.89% | 0.00 | 2% | 49 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -14.89%, PF 0.00, win-rate 2%, 49 lệnh |
| `crypto_lightgbm` | CRYPTO | lightgbm | -11.73% | 0.00 | 0% | 57 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -11.73%, PF 0.00, win-rate 0%, 57 lệnh |
| `crypto_random_forest_v3` | CRYPTO | random_forest | -10.80% | 0.00 | 0% | 40 | Thua hệ thống. Return -10.80%, PF 0.00, win-rate 0%, 40 lệnh |
| `crypto_random_forest_v9` | CRYPTO | random_forest | -9.67% | 0.00 | 0% | 41 | Thua hệ thống. Return -9.67%, PF 0.00, win-rate 0%, 41 lệnh |
| `crypto_lightgbm_v9` | CRYPTO | lightgbm | -9.49% | 0.00 | 0% | 45 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -9.49%, PF 0.00, win-rate 0%, 45 lệnh |
| `crypto_lightgbm_v3` | CRYPTO | lightgbm | -9.49% | 0.00 | 0% | 36 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -9.49%, PF 0.00, win-rate 0%, 36 lệnh |
| `crypto_lightgbm_v6` | CRYPTO | lightgbm | -9.44% | 0.00 | 0% | 35 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -9.44%, PF 0.00, win-rate 0%, 35 lệnh |
| `crypto_lightgbm_v7` | CRYPTO | lightgbm | -9.44% | 0.00 | 0% | 35 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -9.44%, PF 0.00, win-rate 0%, 35 lệnh |
| `crypto_ema_v9` | CRYPTO | ema | -9.00% | 0.00 | 0% | 24 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -9.00%, PF 0.00, win-rate 0%, 24 lệnh |
| `crypto_moving_average_v9` | CRYPTO | moving_average | -8.82% | 0.00 | 0% | 24 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -8.82%, PF 0.00, win-rate 0%, 24 lệnh |
| `crypto_ensemble_v9` | CRYPTO | ensemble | -8.79% | 0.00 | 0% | 24 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -8.79%, PF 0.00, win-rate 0%, 24 lệnh |
| `crypto_lstm_nn_v9` | CRYPTO | lstm_nn | -8.76% | 0.00 | 0% | 24 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -8.76%, PF 0.00, win-rate 0%, 24 lệnh |
| `crypto_xgboost_v5` | CRYPTO | xgboost | -8.15% | 0.00 | 0% | 19 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -8.15%, PF 0.00, win-rate 0%, 19 lệnh |
| `nasdaq_lightgbm_v7` | NASDAQ | lightgbm | -8.01% | 0.19 | 29% | 68 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -8.01%, PF 0.19, win-rate 29%, 68 lệnh |
| `nasdaq_lightgbm_v9` | NASDAQ | lightgbm | -7.83% | 0.56 | 33% | 109 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.83%, PF 0.56, win-rate 33%, 109 lệnh |
| `crypto_xgboost_v3` | CRYPTO | xgboost | -7.73% | 0.01 | 4% | 27 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.73%, PF 0.01, win-rate 4%, 27 lệnh |
| `crypto_xgboost_v6` | CRYPTO | xgboost | -7.73% | 0.01 | 4% | 27 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.73%, PF 0.01, win-rate 4%, 27 lệnh |
| `crypto_xgboost_v7` | CRYPTO | xgboost | -7.73% | 0.01 | 4% | 27 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.73%, PF 0.01, win-rate 4%, 27 lệnh |
| `nasdaq_lightgbm_v6` | NASDAQ | lightgbm | -7.55% | 0.45 | 32% | 81 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.55%, PF 0.45, win-rate 32%, 81 lệnh |
| `crypto_xgboost` | CRYPTO | xgboost | -7.55% | 0.06 | 20% | 35 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.55%, PF 0.06, win-rate 20%, 35 lệnh |
| `nasdaq_lightgbm` | NASDAQ | lightgbm | -7.01% | 0.34 | 31% | 72 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -7.01%, PF 0.34, win-rate 31%, 72 lệnh |
| `crypto_gru_nn_v9` | CRYPTO | gru_nn | -6.75% | 0.27 | 68% | 80 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -6.75%, PF 0.27, win-rate 68%, 80 lệnh |
| `nasdaq_lightgbm_v3` | NASDAQ | lightgbm | -6.65% | 0.52 | 38% | 88 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -6.65%, PF 0.52, win-rate 38%, 88 lệnh |
| `nasdaq_lightgbm_v5` | NASDAQ | lightgbm | -6.21% | 0.05 | 15% | 27 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -6.21%, PF 0.05, win-rate 15%, 27 lệnh |
| `nasdaq_xgboost_v7` | NASDAQ | xgboost | -5.18% | 0.14 | 13% | 52 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -5.18%, PF 0.14, win-rate 13%, 52 lệnh |
| `sp500_xgboost_v9` | SP500 | xgboost | -5.01% | 0.27 | 16% | 50 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -5.01%, PF 0.27, win-rate 16%, 50 lệnh |
| `crypto_arima_garch_v9` | CRYPTO | arima_garch | -4.86% | 0.00 | 0% | 13 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -4.86%, PF 0.00, win-rate 0%, 13 lệnh |
| `gold_xgboost_v9` | GOLD | xgboost | -4.67% | 0.00 | 15% | 27 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -4.67%, PF 0.00, win-rate 15%, 27 lệnh |
| `nasdaq_xgboost_v3` | NASDAQ | xgboost | -4.60% | 0.57 | 24% | 85 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -4.60%, PF 0.57, win-rate 24%, 85 lệnh |
| `sp500_xgboost_v3` | SP500 | xgboost | -4.55% | 0.22 | 18% | 45 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -4.55%, PF 0.22, win-rate 18%, 45 lệnh |
| `gold_lightgbm_v9` | GOLD | lightgbm | -4.44% | 0.00 | 0% | 13 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -4.44%, PF 0.00, win-rate 0%, 13 lệnh |
| `gold_lstm_nn_v9` | GOLD | lstm_nn | -4.38% | 0.15 | 15% | 13 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -4.38%, PF 0.15, win-rate 15%, 13 lệnh |
| `gold_ensemble_v9` | GOLD | ensemble | -4.38% | 0.00 | 0% | 11 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -4.38%, PF 0.00, win-rate 0%, 11 lệnh |
| `gold_gru_nn_v9` | GOLD | gru_nn | -4.38% | 0.15 | 15% | 13 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -4.38%, PF 0.15, win-rate 15%, 13 lệnh |
| `gold_random_forest_v9` | GOLD | random_forest | -4.38% | 0.00 | 0% | 11 | Thua hệ thống. Return -4.38%, PF 0.00, win-rate 0%, 11 lệnh |
| `gold_ema_v9` | GOLD | ema | -4.25% | 0.00 | 0% | 11 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -4.25%, PF 0.00, win-rate 0%, 11 lệnh |
| `crypto_xgboost_v8` | CRYPTO | xgboost | -4.22% | 0.00 | 0% | 14 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -4.22%, PF 0.00, win-rate 0%, 14 lệnh |
| `sp500_lightgbm_v9` | SP500 | lightgbm | -4.04% | 0.29 | 15% | 54 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -4.04%, PF 0.29, win-rate 15%, 54 lệnh |
| `sp500_lightgbm_v3` | SP500 | lightgbm | -3.90% | 0.27 | 19% | 43 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.90%, PF 0.27, win-rate 19%, 43 lệnh |
| `crypto_lightgbm_v8` | CRYPTO | lightgbm | -3.88% | 0.00 | 0% | 10 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.88%, PF 0.00, win-rate 0%, 10 lệnh |
| `sp500_lightgbm_v6` | SP500 | lightgbm | -3.80% | 0.30 | 17% | 46 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.80%, PF 0.30, win-rate 17%, 46 lệnh |
| `nasdaq_xgboost_v9` | NASDAQ | xgboost | -3.76% | 0.71 | 30% | 111 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.76%, PF 0.71, win-rate 30%, 111 lệnh |
| `sp500_lightgbm` | SP500 | lightgbm | -3.74% | 0.17 | 14% | 42 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.74%, PF 0.17, win-rate 14%, 42 lệnh |
| `gold_moving_average_v9` | GOLD | moving_average | -3.68% | 0.00 | 0% | 63 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -3.68%, PF 0.00, win-rate 0%, 63 lệnh |
| `sp500_xgboost_v7` | SP500 | xgboost | -3.61% | 0.13 | 14% | 50 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.61%, PF 0.13, win-rate 14%, 50 lệnh |
| `sp500_xgboost` | SP500 | xgboost | -3.61% | 0.13 | 14% | 50 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.61%, PF 0.13, win-rate 14%, 50 lệnh |
| `nasdaq_xgboost_v6` | NASDAQ | xgboost | -3.57% | 0.54 | 15% | 54 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.57%, PF 0.54, win-rate 15%, 54 lệnh |
| `sp500_lightgbm_v7` | SP500 | lightgbm | -3.57% | 0.17 | 14% | 42 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.57%, PF 0.17, win-rate 14%, 42 lệnh |
| `gold_moving_average_v3` | GOLD | moving_average | -3.52% | 0.00 | 0% | 50 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -3.52%, PF 0.00, win-rate 0%, 50 lệnh |
| `sp500_xgboost_v6` | SP500 | xgboost | -3.15% | 0.28 | 17% | 53 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -3.15%, PF 0.28, win-rate 17%, 53 lệnh |
| `nasdaq_sarima_v6` | NASDAQ | sarima | -3.04% | 0.31 | 25% | 28 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -3.04%, PF 0.31, win-rate 25%, 28 lệnh |
| `crypto_random_forest_v6` | CRYPTO | random_forest | -2.93% | 0.00 | 0% | 10 | Thua hệ thống. Return -2.93%, PF 0.00, win-rate 0%, 10 lệnh |
| `crypto_random_forest_v7` | CRYPTO | random_forest | -2.86% | 0.00 | 0% | 9 | Thua hệ thống. Return -2.86%, PF 0.00, win-rate 0%, 9 lệnh |
| `nasdaq_sarima` | NASDAQ | sarima | -2.80% | 0.41 | 32% | 31 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -2.80%, PF 0.41, win-rate 32%, 31 lệnh |
| `crypto_random_forest` | CRYPTO | random_forest | -2.70% | 0.00 | 0% | 9 | Thua hệ thống. Return -2.70%, PF 0.00, win-rate 0%, 9 lệnh |
| `nasdaq_xgboost` | NASDAQ | xgboost | -2.40% | 0.65 | 19% | 59 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -2.40%, PF 0.65, win-rate 19%, 59 lệnh |
| `nasdaq_gru_nn_v9` | NASDAQ | gru_nn | -2.35% | 0.62 | 45% | 49 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -2.35%, PF 0.62, win-rate 45%, 49 lệnh |
| `nasdaq_sarima_v9` | NASDAQ | sarima | -2.02% | 0.68 | 31% | 49 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -2.02%, PF 0.68, win-rate 31%, 49 lệnh |
| `sp500_arima_garch_v9` | SP500 | arima_garch | -1.73% | 0.59 | 25% | 36 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -1.73%, PF 0.59, win-rate 25%, 36 lệnh |
| `nasdaq_sarima_v4` | NASDAQ | sarima | -1.71% | 0.53 | 32% | 28 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -1.71%, PF 0.53, win-rate 32%, 28 lệnh |
| `crypto_gru_nn` | CRYPTO | gru_nn | -1.67% | 0.41 | 76% | 45 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.67%, PF 0.41, win-rate 76%, 45 lệnh |
| `sp500_lstm_nn_v4` | SP500 | lstm_nn | -1.66% | 0.59 | 20% | 30 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.66%, PF 0.59, win-rate 20%, 30 lệnh |
| `nasdaq_sarima_v3` | NASDAQ | sarima | -1.61% | 0.69 | 34% | 41 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -1.61%, PF 0.69, win-rate 34%, 41 lệnh |
| `sp500_gru_nn_v4` | SP500 | gru_nn | -1.54% | 0.28 | 30% | 27 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.54%, PF 0.28, win-rate 30%, 27 lệnh |
| `gold_moving_average_v7` | GOLD | moving_average | -1.50% | 0.00 | 0% | 28 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.50%, PF 0.00, win-rate 0%, 28 lệnh |
| `sp500_ensemble_v8` | SP500 | ensemble | -1.44% | 0.22 | 85% | 13 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.44%, PF 0.22, win-rate 85%, 13 lệnh |
| `crypto_gru_nn_v6` | CRYPTO | gru_nn | -1.29% | 0.50 | 82% | 39 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.29%, PF 0.50, win-rate 82%, 39 lệnh |
| `nasdaq_sarima_v8` | NASDAQ | sarima | -1.27% | 0.30 | 32% | 19 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -1.27%, PF 0.30, win-rate 32%, 19 lệnh |
| `sp500_ema_v9` | SP500 | ema | -1.26% | 0.69 | 38% | 8 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.26%, PF 0.69, win-rate 38%, 8 lệnh |
| `sp500_arima_garch_v3` | SP500 | arima_garch | -1.05% | 0.62 | 24% | 33 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -1.05%, PF 0.62, win-rate 24%, 33 lệnh |
| `nasdaq_xgboost_v5` | NASDAQ | xgboost | -1.05% | 0.20 | 38% | 24 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -1.05%, PF 0.20, win-rate 38%, 24 lệnh |
| `sp500_ensemble_v3` | SP500 | ensemble | -1.04% | 0.72 | 28% | 130 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -1.04%, PF 0.72, win-rate 28%, 130 lệnh |
| `sp500_egarch_v3` | SP500 | egarch | -1.01% | 0.74 | 31% | 84 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -1.01%, PF 0.74, win-rate 31%, 84 lệnh |
| `nasdaq_moving_average_v9` | NASDAQ | moving_average | -0.98% | 0.75 | 20% | 25 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -0.98%, PF 0.75, win-rate 20%, 25 lệnh |
| `sp500_random_forest_v9` | SP500 | random_forest | -0.98% | 0.60 | 25% | 12 | Thua hệ thống. Return -0.98%, PF 0.60, win-rate 25%, 12 lệnh |
| `nasdaq_ensemble_v4` | NASDAQ | ensemble | -0.92% | 0.61 | 33% | 9 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -0.92%, PF 0.61, win-rate 33%, 9 lệnh |
| `nasdaq_gru_nn_v6` | NASDAQ | gru_nn | -0.86% | 0.78 | 68% | 25 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -0.86%, PF 0.78, win-rate 68%, 25 lệnh |
| `crypto_gru_nn_v4` | CRYPTO | gru_nn | -0.86% | 0.58 | 43% | 35 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -0.86%, PF 0.58, win-rate 43%, 35 lệnh |
| `gold_moving_average` | GOLD | moving_average | -0.77% | 0.00 | 0% | 28 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -0.77%, PF 0.00, win-rate 0%, 28 lệnh |
| `gold_moving_average_v6` | GOLD | moving_average | -0.77% | 0.00 | 0% | 28 | Trend-following: đoán tăng gần như mọi lúc, sai toàn bộ ở ngày đảo chiều. Return -0.77%, PF 0.00, win-rate 0%, 28 lệnh |
| `gold_xgboost_v3` | GOLD | xgboost | -0.68% | 0.02 | 27% | 15 | Tree-based: ngoại suy xu hướng cứng nhắc, thua nặng khi thị trường đảo chiều. Return -0.68%, PF 0.02, win-rate 27%, 15 lệnh |
| `nasdaq_egarch_v7` | NASDAQ | egarch | -0.61% | 0.48 | 33% | 24 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -0.61%, PF 0.48, win-rate 33%, 24 lệnh |
| `nasdaq_sarima_v2` | NASDAQ | sarima | -0.59% | 0.41 | 29% | 14 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -0.59%, PF 0.41, win-rate 29%, 14 lệnh |
| `nasdaq_arima_garch_v2` | NASDAQ | arima_garch | -0.57% | 0.43 | 48% | 31 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -0.57%, PF 0.43, win-rate 48%, 31 lệnh |
| `crypto_egarch_v9` | CRYPTO | egarch | -0.43% | 0.11 | 8% | 13 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -0.43%, PF 0.11, win-rate 8%, 13 lệnh |
| `gold_egarch_v3` | GOLD | egarch | -0.27% | 0.52 | 18% | 11 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -0.27%, PF 0.52, win-rate 18%, 11 lệnh |
| `crypto_egarch_v3` | CRYPTO | egarch | -0.22% | 0.19 | 9% | 11 | Variant cấu hình kém (SL/TP lệch) dù họ thuật toán nhìn chung tốt. Return -0.22%, PF 0.19, win-rate 9%, 11 lệnh |
| `nasdaq_sarima_v7` | NASDAQ | sarima | -0.01% | 0.66 | 33% | 27 | SARIMA seasonal kém hợp ngày giao dịch ngắn, PF thấp. Return -0.01%, PF 0.66, win-rate 33%, 27 lệnh |

---

## 🟠 Tier 2 — ĐỀ XUẤT (chưa tắt, 44 bot)

| Bot ID | Market | Thuật toán | Return | PF | Win-rate | Trades | Lý do |
|---|---|---|---|---|---|---|---|
| `crypto_lstm_nn_v6` | CRYPTO | lstm_nn | -3.06% | 0.00 | 0% | 5 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -3.06%, PF 0.00, 5 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema` | CRYPTO | ema | -2.73% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.73%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema_v4` | CRYPTO | ema | -2.73% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.73%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v2` | CRYPTO | ensemble | -2.61% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.61%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema_v3` | CRYPTO | ema | -2.57% | 0.00 | 0% | 4 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.57%, PF 0.00, 4 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_moving_average_v3` | CRYPTO | moving_average | -2.57% | 0.00 | 0% | 4 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.57%, PF 0.00, 4 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema_v6` | CRYPTO | ema | -2.57% | 0.00 | 0% | 4 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.57%, PF 0.00, 4 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v6` | CRYPTO | ensemble | -2.57% | 0.00 | 0% | 4 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.57%, PF 0.00, 4 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn_v2` | CRYPTO | lstm_nn | -2.47% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.47%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn` | CRYPTO | lstm_nn | -2.46% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.46%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn_v3` | CRYPTO | lstm_nn | -2.45% | 0.00 | 0% | 4 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.45%, PF 0.00, 4 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_moving_average` | CRYPTO | moving_average | -2.43% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.43%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema_v8` | CRYPTO | ema | -2.40% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.40%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble` | CRYPTO | ensemble | -2.35% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.35%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v3` | CRYPTO | ensemble | -2.35% | 0.00 | 0% | 4 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.35%, PF 0.00, 4 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn_v10` | CRYPTO | lstm_nn | -2.29% | 0.00 | 0% | 0 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.29%, PF 0.00, 0 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema_v7` | CRYPTO | ema | -2.28% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.28%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v8` | CRYPTO | ensemble | -2.28% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.28%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn_v8` | CRYPTO | lstm_nn | -2.28% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.28%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn_v5` | CRYPTO | lstm_nn | -2.13% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.13%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v7` | CRYPTO | ensemble | -2.11% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.11%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_moving_average_v6` | CRYPTO | moving_average | -2.07% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.07%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lstm_nn_v7` | CRYPTO | lstm_nn | -2.07% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.07%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v5` | CRYPTO | ensemble | -2.02% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -2.02%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_random_forest_v8` | CRYPTO | random_forest | -1.98% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.98%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_moving_average_v7` | CRYPTO | moving_average | -1.96% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.96%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ema_v2` | CRYPTO | ema | -1.65% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.65%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_random_forest_v5` | CRYPTO | random_forest | -1.43% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.43%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v2` | CRYPTO | arima_garch | -1.36% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.36%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch` | CRYPTO | arima_garch | -1.20% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.20%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v7` | CRYPTO | arima_garch | -1.17% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.17%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_ensemble_v10` | CRYPTO | ensemble | -1.12% | 0.00 | 0% | 0 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.12%, PF 0.00, 0 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v4` | CRYPTO | arima_garch | -1.12% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.12%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lightgbm_v5` | CRYPTO | lightgbm | -1.11% | 0.00 | 0% | 3 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.11%, PF 0.00, 3 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v8` | CRYPTO | arima_garch | -1.09% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.09%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v3` | CRYPTO | arima_garch | -1.07% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.07%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v6` | CRYPTO | arima_garch | -1.07% | 0.00 | 0% | 2 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -1.07%, PF 0.00, 2 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_random_forest_v10` | CRYPTO | random_forest | -0.97% | 0.00 | 0% | 0 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.97%, PF 0.00, 0 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_xgboost_v2` | CRYPTO | xgboost | -0.94% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.94%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lightgbm_v2` | CRYPTO | lightgbm | -0.90% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.90%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_random_forest_v2` | CRYPTO | random_forest | -0.90% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.90%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_lightgbm_v10` | CRYPTO | lightgbm | -0.81% | 0.00 | 0% | 0 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.81%, PF 0.00, 0 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_xgboost_v10` | CRYPTO | xgboost | -0.72% | 0.00 | 0% | 0 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.72%, PF 0.00, 0 lệnh — lỗ/ít giao dịch, dọn cùng cụm |
| `crypto_arima_garch_v5` | CRYPTO | arima_garch | -0.54% | 0.00 | 0% | 1 | Thuộc CRYPTO (toàn market chỉ 2/110 bot lãi, tổng −275%). Return -0.54%, PF 0.00, 1 lệnh — lỗ/ít giao dịch, dọn cùng cụm |

---

## Ghi chú khi muốn bật lại
- **Đừng bật lại nguyên `xgboost` / `lightgbm`** trừ khi đã đổi feature/SL-TP — đây là nhóm lỗ nặng nhất ở MỌI market.
- CRYPTO: nếu muốn giữ lại để bắt nhịp đảo chiều, ưu tiên họ **ARIMA-GARCH / EGARCH** (dự đoán đúng hướng tốt nhất ở crypto), không phải trend-following.
- Sau khi bật lại, theo dõi lại bằng `GET /api/simulation/leaderboard` và `GET /api/monitoring/overview`.
