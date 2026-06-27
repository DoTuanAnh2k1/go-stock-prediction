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

---

## Cập nhật 2026-06-24

> Audit toàn bộ từ DB lúc `2026-06-24`. KPI là giá trị thực tế hiện tại, KHÔNG phải thời điểm 2026-06-19.
> Tiêu chí Tier 1 / Tier 2 giữ nguyên như trên.

### 🔴 Tier 2 leo thang Tier 1 — ĐÃ DEACTIVATE 2026-06-24 (17 bot)

17 bot từ danh sách Tier 2 cũ đã tích lũy đủ lệnh và lỗ đủ nặng để lọt vào Tier 1. **Đã tắt lúc 2026-06-24**.

| Bot ID | Market | Thuật toán | Return | PF | Win-rate | Trades | Ghi chú |
|---|---|---|---|---|---|---|---|
| `crypto_ema_v3` | CRYPTO | ema | -12.91% | 0.06 | 4% | 23 | Leo từ -2.57% lên -12.91% trong 5 ngày |
| `crypto_ema_v6` | CRYPTO | ema | -10.76% | 0.00 | 0% | 16 | Leo từ -2.57% |
| `crypto_lstm_nn_v6` | CRYPTO | lstm_nn | -10.19% | 0.07 | 6% | 18 | Leo từ -3.06% |
| `crypto_lstm_nn_v3` | CRYPTO | lstm_nn | -10.01% | 0.07 | 6% | 17 | Leo từ -2.45% |
| `crypto_arima_garch_v6` | CRYPTO | arima_garch | -8.66% | 0.05 | 15% | 52 | 8/44 thắng, leo từ -1.07% |
| `crypto_arima_garch_v3` | CRYPTO | arima_garch | -8.66% | 0.05 | 15% | 52 | 8/44 thắng, leo từ -1.07% |
| `crypto_ensemble_v3` | CRYPTO | ensemble | -7.58% | 0.17 | 35% | 26 | Leo từ -2.35% |
| `crypto_ema` | CRYPTO | ema | -7.37% | 0.00 | 0% | 9 | Leo từ -2.73% |
| `crypto_ema_v4` | CRYPTO | ema | -7.37% | 0.00 | 0% | 9 | Leo từ -2.73% |
| `crypto_lstm_nn` | CRYPTO | lstm_nn | -7.23% | 0.00 | 0% | 9 | Leo từ -2.46% |
| `crypto_ema_v2` | CRYPTO | ema | -6.42% | 0.00 | 0% | 8 | Leo từ -1.65% |
| `crypto_lightgbm_v5` | CRYPTO | lightgbm | -4.33% | 0.17 | 67% | 27 | 18W/9L nhưng lỗ lớn/thắng nhỏ |
| `crypto_arima_garch_v4` | CRYPTO | arima_garch | -1.82% | 0.44 | 29% | 55 | 16/39, PF 0.44 < 0.8 |
| `crypto_arima_garch` | CRYPTO | arima_garch | -1.82% | 0.44 | 29% | 55 | 16/39, PF 0.44 < 0.8 |
| `crypto_arima_garch_v7` | CRYPTO | arima_garch | -1.79% | 0.43 | 28% | 54 | 15/39, PF 0.43 < 0.8 |
| `crypto_ensemble` | CRYPTO | ensemble | -1.11% | 0.57 | 67% | 9 | PF 0.57 < 0.8 |
| `crypto_ensemble_v6` | CRYPTO | ensemble | -1.10% | 0.57 | 60% | 10 | PF 0.57 < 0.8 |

### 🔴 Tier 1 mới — ĐÃ DEACTIVATE 2026-06-24 (92 bot chưa có trong file)

#### CRYPTO (19 bot mới)

| Bot ID | Thuật toán | Return | PF | Win-rate | Trades | Lý do |
|---|---|---|---|---|---|---|
| `crypto_gru_nn_v3` | gru_nn | -9.10% | 0.36 | 66% | 123 | Trend-following thua nặng dù win-rate 66% — thua lớn mỗi lệnh sai |
| `crypto_btc_xgboost__ps_v3` | xgboost__ps BTC | -5.75% | 0.04 | 19% | 26 | XGBoost per-symbol BTC, 5/21 |
| `crypto_btc_xgboost__ps_v9` | xgboost__ps BTC | -5.23% | 0.08 | 14% | 28 | XGBoost per-symbol BTC, 4/24 |
| `crypto_sol_gru_nn__ps_v9` | gru_nn__ps SOL | -4.68% | 0.00 | 0% | 8 | SOL GRU per-symbol, 0/8 |
| `crypto_sol_lstm_nn__ps_v9` | lstm_nn__ps SOL | -4.66% | 0.00 | 0% | 8 | SOL LSTM per-symbol, 0/8 |
| `crypto_sol_ensemble__ps_v9` | ensemble__ps SOL | -4.59% | 0.00 | 0% | 8 | SOL Ensemble per-symbol, 0/8 |
| `crypto_sol_arima_garch__ps_v9` | arima_garch__ps SOL | -3.27% | 0.16 | 45% | 29 | SOL ARIMA-GARCH, 13/16 |
| `crypto_sol_arima_garch__ps_v6` | arima_garch__ps SOL | -3.27% | 0.16 | 45% | 29 | SOL ARIMA-GARCH, 13/16 |
| `crypto_sol_arima_garch__ps_v3` | arima_garch__ps SOL | -3.27% | 0.16 | 45% | 29 | SOL ARIMA-GARCH, 13/16 |
| `crypto_btc_xgboost__ps_v6` | xgboost__ps BTC | -3.11% | 0.00 | 0% | 13 | BTC XGBoost, 0/13 |
| `crypto_eth_xgboost__ps_v9` | xgboost__ps ETH | -3.03% | 0.08 | 24% | 17 | ETH XGBoost, 4/13 |
| `crypto_eth_xgboost__ps_v3` | xgboost__ps ETH | -3.03% | 0.08 | 24% | 17 | ETH XGBoost, 4/13 |
| `crypto_btc_xgboost__ps` | xgboost__ps BTC | -2.98% | 0.00 | 0% | 17 | BTC XGBoost, 0/17 |
| `crypto_btc_xgboost__ps_v7` | xgboost__ps BTC | -2.98% | 0.00 | 0% | 17 | BTC XGBoost, 0/17 |
| `crypto_eth_lightgbm__ps_v6` | lightgbm__ps ETH | -2.84% | 0.10 | 50% | 14 | ETH LightGBM, 7/7 — lỗ lớn/thắng nhỏ |
| `crypto_eth_lightgbm__ps_v9` | lightgbm__ps ETH | -2.79% | 0.09 | 57% | 14 | ETH LightGBM, 8/6 |
| `crypto_eth_lightgbm__ps_v3` | lightgbm__ps ETH | -2.79% | 0.09 | 57% | 14 | ETH LightGBM, 8/6 |
| `crypto_gru_nn_v2` | gru_nn | -1.20% | 0.66 | 46% | 67 | Pooled GRU lỗ tích lũy, PF 0.66 |
| `crypto_gru_nn_v10` | gru_nn | -0.41% | 0.79 | 49% | 63 | Pooled GRU, PF 0.79 gần ngưỡng |

#### GOLD (20 bot mới)

| Bot ID | Thuật toán | Return | PF | Win-rate | Trades | Lý do |
|---|---|---|---|---|---|---|
| `gold_egarch_v7` | egarch | -1.68% | 0.11 | 10% | 21 | EGARCH Gold pooled, 2/19 |
| `gold_egarch_v6` | egarch | -1.68% | 0.11 | 10% | 21 | EGARCH Gold pooled, 2/19 |
| `gold_egarch_v4` | egarch | -1.68% | 0.11 | 10% | 21 | EGARCH Gold pooled, 2/19 |
| `gold_sarima_v3` | sarima | -1.56% | 0.00 | 0% | 17 | SARIMA Gold, 0/17 |
| `gold_sarima_v9` | sarima | -1.56% | 0.00 | 0% | 17 | SARIMA Gold, 0/17 |
| `gold_xau_spot_egarch__ps_v4` | egarch__ps XAU_spot | -1.50% | 0.00 | 0% | 19 | EGARCH per-symbol XAU_spot, 0/19 |
| `gold_xau_spot_egarch__ps` | egarch__ps XAU_spot | -1.50% | 0.00 | 0% | 19 | EGARCH per-symbol XAU_spot, 0/19 |
| `gold_xau_spot_egarch__ps_v7` | egarch__ps XAU_spot | -1.50% | 0.00 | 0% | 19 | EGARCH per-symbol XAU_spot, 0/19 |
| `gold_xau_spot_egarch__ps_v3` | egarch__ps XAU_spot | -1.50% | 0.00 | 0% | 19 | EGARCH per-symbol XAU_spot, 0/19 |
| `gold_xau_spot_egarch__ps_v6` | egarch__ps XAU_spot | -1.50% | 0.00 | 0% | 19 | EGARCH per-symbol XAU_spot, 0/19 |
| `gold_egarch` | egarch | -0.99% | 0.48 | 32% | 28 | EGARCH Gold pooled, 9/19 |
| `gold_egarch_v9` | egarch | -0.98% | 0.62 | 41% | 39 | EGARCH Gold pooled, 16/23 |
| `gold_xau_spot_egarch__ps_v9` | egarch__ps XAU_spot | -0.97% | 0.52 | 26% | 27 | EGARCH per-symbol XAU_spot, 7/20 |
| `gold_xau_spot_gru_nn__ps_v9` | gru_nn__ps XAU_spot | -0.95% | 0.02 | 9% | 22 | GRU per-symbol XAU_spot, 2/20 |
| `gold_btmc_sjc_sarima__ps_v3` | sarima__ps BTMC_sjc | -0.86% | 0.00 | 0% | 15 | SARIMA per-symbol BTMC_sjc, 0/14 |
| `gold_btmc_sjc_sarima__ps_v9` | sarima__ps BTMC_sjc | -0.86% | 0.00 | 0% | 15 | SARIMA per-symbol BTMC_sjc, 0/14 |
| `gold_xau_spot_gru_nn__ps_v3` | gru_nn__ps XAU_spot | -0.39% | 0.03 | 5% | 21 | GRU per-symbol XAU_spot, 1/20 |
| `gold_xau_spot_gru_nn__ps` | gru_nn__ps XAU_spot | -0.22% | 0.00 | 0% | 18 | GRU per-symbol XAU_spot, 0/18 |
| `gold_xau_spot_gru_nn__ps_v7` | gru_nn__ps XAU_spot | -0.22% | 0.00 | 0% | 18 | GRU per-symbol XAU_spot, 0/18 |
| `gold_xau_spot_gru_nn__ps_v6` | gru_nn__ps XAU_spot | -0.22% | 0.00 | 0% | 18 | GRU per-symbol XAU_spot, 0/18 |

#### NASDAQ (41 bot mới)

| Bot ID | Thuật toán | Return | PF | Win-rate | Trades | Lý do |
|---|---|---|---|---|---|---|
| `nasdaq_qcom_random_forest__ps` | random_forest__ps QCOM | -14.96% | 0.00 | 0% | 17 | RF per-symbol QCOM, 0/17 — thua hệ thống |
| `nasdaq_qcom_random_forest__ps_v7` | random_forest__ps QCOM | -14.96% | 0.00 | 0% | 17 | RF per-symbol QCOM, 0/17 |
| `nasdaq_qcom_random_forest__ps_v6` | random_forest__ps QCOM | -14.81% | 0.00 | 0% | 17 | RF per-symbol QCOM, 0/17 |
| `nasdaq_qcom_random_forest__ps_v9` | random_forest__ps QCOM | -14.77% | 0.00 | 0% | 18 | RF per-symbol QCOM, 0/18 |
| `nasdaq_qcom_random_forest__ps_v3` | random_forest__ps QCOM | -14.35% | 0.00 | 0% | 17 | RF per-symbol QCOM, 0/17 |
| `nasdaq_moving_average_v3` | moving_average | -14.14% | 0.30 | 15% | 33 | MA pooled, 5/28 — trend-following NASDAQ kém |
| `nasdaq_moving_average_v7` | moving_average | -12.32% | 0.00 | 0% | 11 | MA pooled, 0/11 |
| `nasdaq_moving_average_v6` | moving_average | -11.72% | 0.34 | 20% | 25 | MA pooled, 5/20 |
| `nasdaq_moving_average_v8` | moving_average | -11.48% | 0.14 | 10% | 10 | MA pooled, 1/9 |
| `nasdaq_moving_average_v2` | moving_average | -10.09% | 0.23 | 18% | 11 | MA pooled, 2/9 |
| `nasdaq_moving_average` | moving_average | -9.95% | 0.31 | 19% | 16 | MA pooled, 3/13 |
| `nasdaq_qcom_xgboost__ps_v9` | xgboost__ps QCOM | -8.51% | 0.00 | 0% | 9 | XGBoost per-symbol QCOM, 0/9 |
| `nasdaq_arima_garch_v8` | arima_garch | -8.09% | 0.52 | 39% | 67 | ARIMA-GARCH pooled, 26/41 |
| `nasdaq_arima_garch_v3` | arima_garch | -7.75% | 0.69 | 53% | 101 | ARIMA-GARCH pooled, 54/47 — lỗ lớn/thắng nhỏ |
| `nasdaq_qcom_moving_average__ps_v9` | moving_average__ps QCOM | -7.19% | 0.23 | 33% | 12 | MA per-symbol QCOM, 4/8 |
| `nasdaq_arima_garch_v9` | arima_garch | -5.77% | 0.80 | 54% | 123 | ARIMA-GARCH pooled, 67/56 — PF sát ngưỡng |
| `nasdaq_qcom_arima_garch__ps_v9` | arima_garch__ps QCOM | -5.38% | 0.29 | 40% | 10 | ARIMA-GARCH per-symbol QCOM, 4/6 |
| `nasdaq_random_forest_v6` | random_forest | -3.42% | 0.40 | 24% | 17 | RF pooled NASDAQ, 4/13 |
| `nasdaq_gru_nn_v4` | gru_nn | -3.15% | 0.57 | 36% | 47 | GRU pooled, 17/21 — tích lũy lỗ |
| `nasdaq_random_forest` | random_forest | -3.03% | 0.53 | 32% | 19 | RF pooled, 6/13 |
| `nasdaq_random_forest_v9` | random_forest | -2.85% | 0.77 | 37% | 52 | RF pooled, 19/33 |
| `nasdaq_gru_nn_v3` | gru_nn | -2.59% | 0.66 | 48% | 54 | GRU pooled, 26/28 |
| `nasdaq_ema_v3` | ema | -2.28% | 0.72 | 29% | 17 | EMA pooled, 5/12 |
| `nasdaq_amd_gru_nn__ps_v3` | gru_nn__ps AMD | -1.66% | 0.40 | 50% | 14 | GRU per-symbol AMD, 7/7 — lỗ lớn/thắng nhỏ |
| `nasdaq_amd_gru_nn__ps_v6` | gru_nn__ps AMD | -1.66% | 0.40 | 50% | 14 | GRU per-symbol AMD, 7/7 |
| `nasdaq_amd_gru_nn__ps_v9` | gru_nn__ps AMD | -1.66% | 0.40 | 50% | 14 | GRU per-symbol AMD, 7/7 |
| `nasdaq_egarch_v8` | egarch | -1.31% | 0.61 | 15% | 13 | EGARCH pooled, 2/11 |
| `nasdaq_avgo_gru_nn__ps_v4` | gru_nn__ps AVGO | -1.31% | 0.15 | 50% | 12 | GRU per-symbol AVGO, 6/6 |
| `nasdaq_lightgbm_v8` | lightgbm | -1.08% | 0.65 | 61% | 23 | LightGBM pooled, 14/9 — win-rate cao nhưng PF 0.65 |
| `nasdaq_intc_egarch__ps_v9` | egarch__ps INTC | -0.88% | 0.39 | 50% | 8 | EGARCH per-symbol INTC, 4/4 |
| `nasdaq_meta_egarch__ps_v9` | egarch__ps META | -0.57% | 0.53 | 50% | 20 | EGARCH per-symbol META, 10/10 |
| `nasdaq_meta_egarch__ps_v3` | egarch__ps META | -0.33% | 0.67 | 63% | 16 | EGARCH per-symbol META, 10/6 — borderline |
| `nasdaq_moving_average_v4` | moving_average | -0.31% | 0.59 | 30% | 10 | MA pooled, PF 0.59 |
| `nasdaq_moving_average_v5` | moving_average | -0.31% | 0.59 | 30% | 10 | MA pooled |
| `nasdaq_rl_dqn_v9` | rl_dqn | -0.26% | 0.72 | 39% | 18 | RL DQN NASDAQ pooled |
| `nasdaq_moving_average_v10` | moving_average | -0.22% | 0.68 | 29% | 17 | MA pooled |
| `nasdaq_ema_v9` | ema | -0.20% | 0.73 | 32% | 22 | EMA pooled |
| `nasdaq_ema_v7` | ema | -0.18% | 0.59 | 33% | 9 | EMA pooled |
| `nasdaq_ema` | ema | -0.17% | 0.75 | 40% | 10 | EMA pooled |
| `nasdaq_ema_v6` | ema | -0.14% | 0.76 | 44% | 9 | EMA pooled |
| `nasdaq_ema_v8` | ema | -0.12% | 0.74 | 33% | 9 | EMA pooled |

#### SP500 (12 bot mới)

| Bot ID | Thuật toán | Return | PF | Win-rate | Trades | Lý do |
|---|---|---|---|---|---|---|
| `sp500_qqq_sarima__ps_v9` | sarima__ps QQQ | -2.05% | 0.00 | 0% | 10 | SARIMA per-symbol QQQ, 0/10 |
| `sp500_sarima_v6` | sarima | -1.54% | 0.16 | 20% | 15 | SARIMA pooled SP500, 3/12 |
| `sp500_qqq_sarima__ps_v6` | sarima__ps QQQ | -1.43% | 0.00 | 0% | 8 | SARIMA per-symbol QQQ, 0/8 |
| `sp500_qqq_sarima__ps_v3` | sarima__ps QQQ | -1.43% | 0.00 | 0% | 8 | SARIMA per-symbol QQQ, 0/8 |
| `sp500_egarch_v9` | egarch | -1.35% | 0.77 | 44% | 116 | EGARCH pooled SP500, 51/63 — lỗ tích lũy nhiều lệnh |
| `sp500_qqq_egarch__ps_v9` | egarch__ps QQQ | -1.24% | 0.01 | 30% | 10 | EGARCH per-symbol QQQ, 3/7 |
| `sp500_moving_average_v9` | moving_average | -1.07% | 0.47 | 25% | 8 | MA pooled SP500, 2/6 |
| `sp500_sarima_v3` | sarima | -1.01% | 0.45 | 48% | 23 | SARIMA pooled, 11/12 |
| `sp500_sarima_v9` | sarima | -0.66% | 0.77 | 48% | 29 | SARIMA pooled, 14/15 |
| `sp500_sarima_v7` | sarima | -0.59% | 0.34 | 23% | 13 | SARIMA pooled, 3/10 |
| `sp500_sarima_v4` | sarima | -0.59% | 0.34 | 23% | 13 | SARIMA pooled, 3/10 |
| `sp500_sarima` | sarima | -0.59% | 0.34 | 23% | 13 | SARIMA pooled, 3/10 |

### 🟠 Tier 2 mới — ĐỀ XUẤT (chưa có trong file, lỗ nhẹ / ít lệnh)

> Tổng 276 bot (CRYPTO: 55, GOLD: 25, NASDAQ: 178, SP500: 18). Chỉ liệt kê nhóm lỗ nặng nhất (return < −2%). Phần còn lại gom theo nhóm symbol.

#### Nhóm lỗ nặng nhất (return < −2%)

| Bot ID | Market | Thuật toán | Return | Trades | Ghi chú |
|---|---|---|---|---|---|
| `crypto_sol_gru_nn__ps_v6` | CRYPTO | gru_nn__ps SOL | -4.89% | 7 | 0/7 — chưa đủ 8 lệnh cho Tier 1 |
| `crypto_sol_gru_nn__ps_v3` | CRYPTO | gru_nn__ps SOL | -4.89% | 7 | 0/7 |
| `crypto_sol_ensemble__ps_v3` | CRYPTO | ensemble__ps SOL | -4.88% | 7 | 0/7 |
| `crypto_sol_ensemble__ps_v6` | CRYPTO | ensemble__ps SOL | -4.88% | 7 | 0/7 |
| `crypto_sol_lstm_nn__ps_v6` | CRYPTO | lstm_nn__ps SOL | -4.87% | 7 | 0/7 |
| `crypto_sol_lstm_nn__ps_v3` | CRYPTO | lstm_nn__ps SOL | -4.87% | 7 | 0/7 |
| `crypto_sol_ema__ps_v2` | CRYPTO | ema__ps SOL | -4.06% | 5 | 0/5 |
| `crypto_sol_gru_nn__ps` | CRYPTO | gru_nn__ps SOL | -4.06% | 5 | 0/5 |
| `crypto_sol_lstm_nn__ps` | CRYPTO | lstm_nn__ps SOL | -4.04% | 5 | 0/5 |
| `crypto_sol_ensemble__ps` | CRYPTO | ensemble__ps SOL | -4.00% | 5 | 0/5 |
| `crypto_eth_ema__ps_v9` | CRYPTO | ema__ps ETH | -3.92% | 6 | 0/6 |
| `nasdaq_qcom_moving_average__ps_v7` | NASDAQ | moving_average__ps QCOM | -9.70% | 7 | 0/7 — QCOM MA toàn thua |
| `nasdaq_qcom_moving_average__ps_v8` | NASDAQ | moving_average__ps QCOM | -9.70% | 7 | 0/7 |
| `nasdaq_qcom_moving_average__ps` | NASDAQ | moving_average__ps QCOM | -9.70% | 7 | 0/7 |
| `nasdaq_qcom_moving_average__ps_v2` | NASDAQ | moving_average__ps QCOM | -9.70% | 7 | 0/7 |
| `nasdaq_qcom_moving_average__ps_v3` | NASDAQ | moving_average__ps QCOM | -9.70% | 7 | 0/7 |
| `nasdaq_qcom_moving_average__ps_v6` | NASDAQ | moving_average__ps QCOM | -9.70% | 7 | 0/7 |
| `nasdaq_qcom_arima_garch__ps_v4` | NASDAQ | arima_garch__ps QCOM | -6.54% | 4 | 0/4 |
| `nasdaq_qcom_arima_garch__ps` | NASDAQ | arima_garch__ps QCOM | -6.54% | 4 | 0/4 |
| `nasdaq_qcom_arima_garch__ps_v6` | NASDAQ | arima_garch__ps QCOM | -6.54% | 4 | 0/4 |
| `nasdaq_qcom_arima_garch__ps_v7` | NASDAQ | arima_garch__ps QCOM | -6.54% | 4 | 0/4 |
| `nasdaq_qcom_arima_garch__ps_v3` | NASDAQ | arima_garch__ps QCOM | -6.54% | 4 | 0/4 |
| `nasdaq_qcom_arima_garch__ps_v8` | NASDAQ | arima_garch__ps QCOM | -5.24% | 4 | 0/4 |
| `nasdaq_qcom_arima_garch__ps_v2` | NASDAQ | arima_garch__ps QCOM | -5.19% | 4 | 0/4 |
| `nasdaq_random_forest_v2` | NASDAQ | random_forest | -3.77% | 4 | 0/4 |
| `nasdaq_random_forest_v8` | NASDAQ | random_forest | -3.09% | 7 | 1/6 |
| `nasdaq_tsla_lightgbm__ps_v3` | NASDAQ | lightgbm__ps TSLA | -3.08% | 4 | 0/4 |
| `nasdaq_tsla_lightgbm__ps_v9` | NASDAQ | lightgbm__ps TSLA | -3.08% | 4 | 0/4 |
| `sp500_qqq_sarima__ps_v9` | SP500 | sarima__ps QQQ | -2.05% | 7 | 0/7 — QQQ SARIMA toàn thua |

> Còn ~240 bot Tier 2 mới khác lỗ nhẹ (-0.5% đến -2%), chủ yếu thuộc các nhóm:
> - NASDAQ: AMD (tất cả algo), INTC (sarima, arima_garch, ema, xgboost, lgbm, rf), NVDA (gru, sarima, xgboost, lgbm, ma), NFLX (rf, xgboost, ensemble, lgbm), AVGO (lgbm), AMZN (lstm, ensemble, sarima, gru, rf, egarch)
> - CRYPTO: ETH (ema, ensemble, lstm, rf, gru, arima_garch per-symbol), BTC (ema, lstm, gru, ensemble per-symbol), SOL (ema, gru, lstm, ensemble variants nhỏ)
> - GOLD: egarch pooled (v2,v5,v8,v10), sarima pooled (v3,v4,v6,v7), ensemble pooled (v2–v8), gru pooled (v4), lightgbm (v2), xgboost (v2), egarch__ps XAU_spot (v2,v5,v8,v10), lstm_nn__ps XAU_spot (v9), ensemble__ps/rf__ps XAU_spot

### ⚪ Tier 0 — Chưa trade lần nào (0 trades)

> Tổng **3.480 bot** chưa thực hiện bất kỳ lệnh nào. Không gây lỗ nhưng chiếm slot active. Phân theo market:

| Market | Số bot Tier 0 | Nhóm algo chưa giao dịch phổ biến |
|---|---|---|
| **CRYPTO** | 274 | arima_garch__ps BTC/ETH/SOL, egarch__ps (cả 3), sarima__ps (cả 3), lightgbm__ps BTC/SOL, moving_average__ps (cả 3), random_forest__ps BTC/ETH/SOL (trừ top performers), xgboost__ps SOL |
| **GOLD** | 346 | arima_garch pooled (v2–v10), arima_garch__ps BTMC_nhan_tron/BTMC_sjc (tất cả), phần lớn algo__ps XAU_spot/BTMC_sjc chưa có signal |
| **NASDAQ** | 1.325 | Hầu hết per-symbol bots của AMZN, MSFT, CSCO, AAPL, AVGO, NVDA (các algo chưa kịp phát signal), nhiều pooled variants (rl_dqn variants, lstm_nn variants, arima_garch variants) |
| **SP500** | 1.535 | Hầu hết per-symbol bots (KO, UNH, QQQ, SPY, và nhiều mã khác), phần lớn pooled variants chưa có signal |

> Các bot Tier 0 **không cần deactivate** — chỉ đang chờ điều kiện thị trường phù hợp. Theo dõi lại sau 2–4 tuần nếu vẫn 0 trades thì mới xem xét.

---

## ⚪ Tier 0 leo thang — ĐÃ DEACTIVATE 2026-06-24 (65 bot)

> Tiêu chí: `total_trades = 0` **VÀ** đã chạy **> 7 ngày** trong market GOLD hoặc CRYPTO.
> Lý do: threshold quá cao so với biên độ tín hiệu thực tế — sau 7+ ngày không một lệnh nào, không có triển vọng tự phục hồi.

### CRYPTO (26 bot — chạy từ 2026-06-16, 8 ngày)

| Bot ID | Thuật toán | Lý do |
|---|---|---|
| `crypto_arima_garch_v10` | arima_garch | 8 ngày, 0 trades — threshold quá cao |
| `crypto_egarch_v2` | egarch | 8 ngày, 0 trades |
| `crypto_egarch_v5` | egarch | 8 ngày, 0 trades |
| `crypto_egarch_v8` | egarch | 8 ngày, 0 trades |
| `crypto_egarch_v10` | egarch | 8 ngày, 0 trades |
| `crypto_ema_v5` | ema | 8 ngày, 0 trades |
| `crypto_ema_v10` | ema | 8 ngày, 0 trades |
| `crypto_ensemble_v4` | ensemble | 8 ngày, 0 trades |
| `crypto_lightgbm_v4` | lightgbm | 8 ngày, 0 trades |
| `crypto_lstm_nn_v4` | lstm_nn | 8 ngày, 0 trades |
| `crypto_moving_average_v2` | moving_average | 8 ngày, 0 trades |
| `crypto_moving_average_v4` | moving_average | 8 ngày, 0 trades |
| `crypto_moving_average_v5` | moving_average | 8 ngày, 0 trades |
| `crypto_moving_average_v8` | moving_average | 8 ngày, 0 trades |
| `crypto_moving_average_v10` | moving_average | 8 ngày, 0 trades |
| `crypto_random_forest_v4` | random_forest | 8 ngày, 0 trades |
| `crypto_random_forest_v10` | random_forest | 8 ngày, 0 trades |
| `crypto_sarima_v2` | sarima | 8 ngày, 0 trades — SARIMA intraday signal quá yếu |
| `crypto_sarima_v4` | sarima | 8 ngày, 0 trades |
| `crypto_sarima_v5` | sarima | 8 ngày, 0 trades |
| `crypto_sarima_v6` | sarima | 8 ngày, 0 trades |
| `crypto_sarima_v7` | sarima | 8 ngày, 0 trades |
| `crypto_sarima_v8` | sarima | 8 ngày, 0 trades |
| `crypto_sarima_v10` | sarima | 8 ngày, 0 trades |
| `crypto_xgboost_v4` | xgboost | 8 ngày, 0 trades |
| `crypto_xgboost_v10` | xgboost | 8 ngày, 0 trades |

### GOLD (39 bot — chạy từ 2026-06-15, 9 ngày)

| Bot ID | Thuật toán | Lý do |
|---|---|---|
| `gold_arima_garch` | arima_garch | 9 ngày, 0 trades — ARIMA-GARCH không ra signal trên GOLD |
| `gold_arima_garch_v2` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v3` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v4` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v5` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v6` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v7` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v8` | arima_garch | 9 ngày, 0 trades |
| `gold_arima_garch_v10` | arima_garch | 9 ngày, 0 trades |
| `gold_ema` | ema | 9 ngày, 0 trades |
| `gold_ema_v2` | ema | 9 ngày, 0 trades |
| `gold_ema_v4` | ema | 9 ngày, 0 trades |
| `gold_ema_v5` | ema | 9 ngày, 0 trades |
| `gold_ema_v6` | ema | 9 ngày, 0 trades |
| `gold_ema_v7` | ema | 9 ngày, 0 trades |
| `gold_ema_v8` | ema | 9 ngày, 0 trades |
| `gold_ema_v10` | ema | 9 ngày, 0 trades |
| `gold_ensemble_v4` | ensemble | 9 ngày, 0 trades |
| `gold_ensemble_v10` | ensemble | 9 ngày, 0 trades |
| `gold_gru_nn_v10` | gru_nn | 9 ngày, 0 trades |
| `gold_lightgbm_v4` | lightgbm | 9 ngày, 0 trades |
| `gold_lightgbm_v10` | lightgbm | 9 ngày, 0 trades |
| `gold_lstm_nn_v4` | lstm_nn | 9 ngày, 0 trades |
| `gold_moving_average_v2` | moving_average | 9 ngày, 0 trades |
| `gold_moving_average_v4` | moving_average | 9 ngày, 0 trades |
| `gold_moving_average_v5` | moving_average | 9 ngày, 0 trades |
| `gold_moving_average_v8` | moving_average | 9 ngày, 0 trades |
| `gold_moving_average_v10` | moving_average | 9 ngày, 0 trades |
| `gold_random_forest_v2` | random_forest | 9 ngày, 0 trades |
| `gold_random_forest_v4` | random_forest | 9 ngày, 0 trades |
| `gold_random_forest_v5` | random_forest | 9 ngày, 0 trades |
| `gold_random_forest_v8` | random_forest | 9 ngày, 0 trades |
| `gold_random_forest_v10` | random_forest | 9 ngày, 0 trades |
| `gold_sarima_v2` | sarima | 9 ngày, 0 trades |
| `gold_sarima_v5` | sarima | 9 ngày, 0 trades |
| `gold_sarima_v8` | sarima | 9 ngày, 0 trades |
| `gold_sarima_v10` | sarima | 9 ngày, 0 trades |
| `gold_xgboost_v4` | xgboost | 9 ngày, 0 trades |
| `gold_xgboost_v10` | xgboost | 9 ngày, 0 trades |
