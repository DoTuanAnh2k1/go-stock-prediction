# RL DQN Agent — thuật toán dự đoán thứ 12 + trading bot native

Ngày: 2026-06-20 · Trạng thái: approved, đang implement

## Mục tiêu
Bổ sung một Reinforcement Learning agent (DQN viết tay bằng PyTorch) vào hệ thống,
đóng **cả hai** vai trò: (1) thuật toán dự đoán thứ 12 (`rl_dqn`) ghi `predicted_price`
như 11 thuật toán hiện có; (2) trading bot **native** thực thi trực tiếp action của policy
ở tầng simulation. Train đầy đủ trên data lịch sử, gộp vào training tuần. Cả 4 market.

## Quyết định đã chốt
- **Tích hợp**: cả hai (predictor + bot).
- **Thuật toán**: DQN viết tay bằng PyTorch (không thêm dependency nặng), theo pattern `lstm.py`/`gru.py`.
- **Reward**: dense từng bước — `reward_t = position_t × (price_{t+1}/price_t − 1) − cost × |Δposition|`, `cost ≈ 0.0005`.
- **Bot**: RL **native** (B) — policy tự quyết entry/exit, không dùng buy/sell threshold.
- **Train**: gộp vào `train_for_market()` qua `train_batch()` — job `train_*` Chủ nhật hàng tuần.
- **Phạm vi**: GOLD, NASDAQ100, CRYPTO, SP500.

## RL formulation
- **Observation**: `build_enhanced_features()` (~30 feature) tại bước `t` + position_state
  `(holding_flag, unrealized_pnl_pct, holding_days)`.
- **Action**: rời rạc `{0: hold, 1: buy/long, 2: sell/flat}`.
- **Network**: MLP nhỏ PyTorch (vd in_dim → 128 → 64 → 3), viết tay như `_build_model()` của LSTM.
- **Training**: DQN chuẩn — replay buffer, target network (soft/periodic update), ε-greedy decay,
  Huber loss, Adam. Mỗi market = 1 environment trên chuỗi giá lịch sử (gộp episode qua các instrument
  của market). Minimum data như các tree-model (`MIN_DATA_POINTS`).

## Vai trò predictor — map action → predicted_price
- `policy.act(observation)` greedy → action; magnitude từ volatility gần đây `σ` (rolling std), hằng số `k` nhỏ:
  - buy  → `predicted = current × (1 + k·σ)`
  - sell → `predicted = current × (1 − k·σ)`
  - hold → `predicted ≈ current`
- Clamp qua `get_max_change_pct(market)`. `confidence` = softmax-prob của action được chọn.
- **Không** thêm RL vào Ensemble ở v1.

## Vai trò bot — RL native (nhánh trong TradingBot.step)
```
if config.algorithm == "rl_dqn":
    1. SL/TP safety check (GIỮ — guard rủi ro cứng)
    2. mỗi symbol của market:
         obs = enhanced_features(giá ≤ sim_date) + position_state(từ self.portfolio)
         action = policy.act(obs)        # greedy, no exploration
    3. thực thi BUY/SELL/HOLD trực tiếp qua portfolio (KHÔNG dùng buy/sell threshold)
else:
    # luồng threshold hiện tại — nguyên vẹn
```
- `policy.act(observation)`: method chung cho cả predictor lẫn bot (một nguồn sự thật).
- **As-of price helper**: cần history `trading_date ≤ sim_date` để dựng observation lúc backtest
  (live step = data mới nhất, đã có). Thêm helper trong `signal.py` hoặc `repository.py`.
- **Seed bot**: chỉ **1 bot RL/market** (4 bot, không nhân 10 variant) — variant threshold vô nghĩa khi
  policy tự quyết; chỉ `stop_loss`/`take_profit` còn tác dụng.

## Lưu model (hạ tầng mới duy nhất)
- DQN quá tốn để train lúc inference → **persist checkpoint xuống đĩa**.
- File: `${RL_MODEL_DIR}/rl_dqn_{market}.pt`. Env mới `RL_MODEL_DIR=/models`.
- Docker volume mới `rl_models:/models` cho `prediction-svc`.
- `train_batch()` → `torch.save`; `build_algorithms()`/lazy-load checkpoint khi tạo instance RL.
- Thiếu PyTorch / thiếu checkpoint / lỗi inference → **fallback EMA** (đúng pattern mọi model), không vỡ pipeline.

## Files chạm
- **Mới (Python)**: `prediction-svc/src/algorithms/rl_dqn.py` (+ optional `algorithms/rl/`:
  network, replay buffer, env helper), `prediction-svc/tests/unit/test_rl_dqn.py`.
- **Sửa (Python)**: `algorithms/registry.py` (thêm vào `build_algorithms`),
  `simulation/bot.py` (nhánh RL native), `simulation/signal.py` hoặc `database/repository.py`
  (as-of price helper), `simulation/seeder.py` (seed 4 bot RL, không variant),
  `config.py` (`RL_MODEL_DIR`).
- **Sửa (Go)**: `api-svc/pkg/service/predict/registry/algorithms.go` (AlgorithmDef metadata `rl_dqn`).
- **Infra**: `deploy/docker-compose.yaml` (volume `rl_models:/models`), `.env` (`RL_MODEL_DIR=/models`).
- **Docs**: CLAUDE.md (12 thuật toán, env mới, volume mới, bot RL native).

## An toàn / fallback
- Tôn trọng `market_calendar` (NASDAQ/SP500 đóng cửa) và clamp như mọi thuật toán.
- Inference greedy (ε=0); training mới dùng ε-greedy.
- SL/TP giữ vai trò guard cho bot RL.

## Test
- Unit (`tests/unit/test_rl_dqn.py`, `pytest.importorskip("torch")`): env step/reward đúng dấu,
  action→price mapping (buy>current, sell<current, hold≈current), fallback khi thiếu checkpoint,
  `train_batch` tạo file checkpoint, shape mạng & `act()` trả action hợp lệ, clamp market-aware.
```
