# Per-Symbol Models — Tài liệu triển khai

> Tính năng **bổ sung, song song** với mô hình pooled per-market hiện có. Bật/tắt bằng 1 flag,
> tách bạch hoàn toàn để **so sánh A/B theo thời gian** và **rollback** an toàn.

## 1. Mục tiêu & nguyên tắc

Trước đây mỗi market (GOLD/NASDAQ100/CRYPTO/SP500) chỉ có **một bộ 12 instance thuật toán dùng chung
cho tất cả các mã** trong market đó (pooled / global model). Tính năng này thêm một nhánh mới: **mỗi
(mã × thuật toán) có model riêng**, train chỉ trên dữ liệu của chính mã đó.

Nguyên tắc bất di bất dịch:

- **Cộng thêm, không sửa cũ.** Pooled per-market giữ nguyên 100%. Per-symbol chỉ hoạt động khi
  `PER_SYMBOL_ENABLED=true`. Khi tắt, hệ thống chạy y hệt như trước.
- **Tách bạch bằng hậu tố `__ps`.** Mọi prediction per-symbol lưu `algorithm_name` có hậu tố `__ps`
  (vd `lightgbm__ps`) trong **cùng** bảng prediction. Dashboard/reconcile/direction-accuracy cũ không
  bị lẫn — per-symbol hiện thành các "thuật toán" riêng.
- **Rollback** = đặt `PER_SYMBOL_ENABLED=false` (ngừng train/predict/seed per-symbol). Dữ liệu `__ps`
  và bot per-symbol cũ vẫn nằm đó nhưng không sinh thêm; có thể xóa bằng `algorithm_name LIKE '%\_\_ps'`
  và `sim_bots.symbol IS NOT NULL` nếu muốn dọn.

## 2. Cấu hình (flag)

`prediction-svc/src/config.py` + `.env` + `deploy/docker-compose.yaml`:

| Env | Default | Ý nghĩa |
|-----|---------|---------|
| `PER_SYMBOL_ENABLED` | `false` | Bật/tắt toàn bộ nhánh per-symbol |
| `PER_SYMBOL_MIN_POINTS` | `80` | Số điểm giá tối thiểu để 1 mã được train per-symbol (sàn chung) |
| `PER_SYMBOL_WORKERS` | `0` | Số luồng cho training + bot live-step (`0` = `os.cpu_count()`) |

## 3. Schema thay đổi

`sim_bots` thêm cột **`symbol VARCHAR(30) NULL`**:
- `NULL` → bot pooled per-market (hành vi cũ).
- Có giá trị → bot per-symbol, khóa vào đúng 1 mã; cột `algorithm` mang key có hậu tố `__ps`.

Cập nhật 3 nơi: `database.sql` (CREATE + `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` để idempotent với DB
đã tồn tại), GORM struct `api-svc/pkg/models/models_db/simulation.go` (`Symbol *string`), Python ORM
`prediction-svc/src/database/models.py` (`symbol`). Index `idx_sim_bots_market_symbol`.

## 4. Thành phần đã sửa

### 4.1 Registry — `prediction-svc/src/algorithms/registry.py`
- Cache 2 tầng mới `_symbol_algos[market][symbol] -> {algo_key: instance}` (song song với pooled
  `_market_algos`, không đụng).
- Hằng số & helper: `PS_SUFFIX = "__ps"`, `is_ps()`, `base_key()`.
- `PER_SYMBOL_ALGO_MIN_POINTS` — ngưỡng dữ liệu tối thiểu **theo từng thuật toán** (data-starved skip):
  ```
  moving_average:30, ema:35, arima_garch:60, sarima:80, egarch:80,
  lightgbm:80, xgboost:80, random_forest:80, lstm_nn:150, gru_nn:150,
  ensemble:150, rl_dqn:150   (mặc định 80)
  ```
- API mới: `build_algorithms_for_symbol`, `get_algos_for_symbol`, `set_algos_for_symbol`,
  `has_symbol_algos`, `get_trained_algos_for_symbol` (trả `None` khi cold-start → **không bao giờ
  train trong hot path predict**).
- `base.py`: thêm class attr `_symbol_key` cạnh `_market_key`.

### 4.2 Checkpoint — `prediction-svc/src/algorithms/rl_dqn.py`
- `_checkpoint_path` nhận thêm `symbol_key`: per-symbol → `rl_dqn_{MARKET}_{symbol}.pt`,
  pooled (symbol None) → giữ nguyên `rl_dqn_{MARKET}.pt` (không regression).
- LSTM/GRU/tree không lưu checkpoint đĩa (model in-memory), nên per-symbol chỉ cần instance cache riêng.

### 4.3 Training — `prediction-svc/src/orchestrator/training.py`
- `_collect_symbol_series_for_market(mk)` → list `(symbol_label, prices_asc, vols)` mỗi mã.
  **Nhãn symbol** (phải khớp runner + seeder): GOLD `f"{source}_{product_type}"` (XAU_spot, BTMC_sjc,
  BTMC_nhan_tron); NASDAQ/SP500 = symbol; CRYPTO = coin symbol (BTC/ETH/SOL).
- `train_per_symbol_for_market(mk)`: **đa luồng** (`ThreadPoolExecutor`, ≤8 worker) — mỗi mã 1 worker,
  build instance riêng, train algo nào đủ `PER_SYMBOL_ALGO_MIN_POINTS`, **skip algo đói data**, lưu qua
  `set_algos_for_symbol`. Log 1 dòng/mã (trained/skipped).
- **Hook**: cuối `train_for_market(mk)` (sau pooled) gọi `train_per_symbol_for_market` nếu flag bật →
  pipeline "train mỗi 10 lần crawl" tự refresh cả per-symbol.

### 4.4 Prediction — `prediction-svc/src/orchestrator/runner.py`
- Mỗi `_predict_*` sau vòng pooled chạy thêm `_predict_<market>_per_symbol` (đa luồng theo mã) nếu flag bật.
- Dùng `get_trained_algos_for_symbol(...)`; mã nào chưa có model cached → **skip** (cold-start, không train
  trong predict path). Ghi prediction với `algorithm_name=f"{key}{PS_SUFFIX}"`, cùng cột symbol như pooled
  → reconcile & direction-accuracy tự pick up.

### 4.5 Bot — `prediction-svc/src/simulation/bot.py`
- `BotConfig` thêm `symbol: str | None = None`.
- `step()`: khi `config.symbol` set → lọc signals/SL-TP về đúng mã đó. RL branch nhận diện bằng
  `base_key(algorithm) == "rl_dqn"` (để `rl_dqn__ps` vào nhánh RL) và load policy per-symbol qua
  `get_algos_for_symbol(market, symbol)` (đúng checkpoint `rl_dqn_{market}_{symbol}.pt`).

### 4.6 Engine — `prediction-svc/src/simulation/engine.py` (đa luồng + tối ưu live-step)
- Mọi nơi dựng `BotConfig` truyền thêm `symbol=db_bot.symbol`.
- **`StepDataCache`**: build **một lần** mỗi market mỗi live-step — fetch prediction **một query mỗi
  `algorithm_name` riêng biệt** (tất cả variant × tất cả mã của cùng 1 algo dùng chung 1 query) + 1 batch
  current_prices. Loại bỏ hàng nghìn query trùng lặp → live-step O(số algo) thay vì O(số bot).
- **Đa luồng**: `_run_bot_steps` chạy bot qua `ThreadPoolExecutor` (≤16 worker, mỗi worker session
  riêng — sessionmaker thread-safe). Backtest giữ tuần tự (`cache=None`) để ưu tiên đúng đắn.
- `signal.py`: `get_signals(..., cached_rows=...)` dùng rows pre-fetched; thêm `fetch_predictions(...)`
  cho cache.
- `connection.py`: nâng pool `pool_size=20, max_overflow=40, pool_pre_ping=True` cho thread pool.

### 4.7 Seeder — `prediction-svc/src/simulation/seeder.py`
- Sau khi seed pooled (444 bot, giữ nguyên), nếu flag bật seed thêm bot per-symbol:
  **mỗi (mã × 11 algo × 10 variant) + 1 RL/mã**. `bot_id = f"{market}_{sym}_{algo}__ps{variant}"`,
  `algorithm = f"{algo}__ps"`, `symbol = <mã>`. Insert-if-not-exists.

## 5. Quy mô thực tế (đã seed)

| Market | Số mã | Bot per-symbol |
|--------|------:|---------------:|
| CRYPTO | 3 | 333 |
| GOLD | 3 | 333 |
| NASDAQ | 15 | 1665 |
| SP500 | 15 | 1665 |
| **Tổng per-symbol** | | **3996** |
| Pooled (cũ) | | 444 |
| **Tổng cộng** | | **4440** |

## 6. Thuật toán per-symbol: chạy vs skip

Per-symbol train **11/12 algo** cho mỗi mã với data crypto ~180 điểm; **`ensemble` bị skip ban đầu**
(cần ≥150 điểm + direction-accuracy của các base, sẽ tham gia sau khi có dữ liệu reconcile). Các market
ít data hơn sẽ skip thêm LSTM/GRU/RL (cần 150). Bot của algo bị skip vẫn được seed nhưng **idle** (không
có prediction → không trade) cho tới khi đủ data — "bổ sung sau" như yêu cầu.

## 7. Kết quả verify (2026-06-22)

- Startup sạch, seeder chèn **3996** bot per-symbol (tổng 4440).
- Train per-symbol CRYPTO: **25s** (3 mã × 11 algo, đa luồng), skip ensemble.
- Predict per-symbol: sinh prediction `__ps` cho 11 algo (đã kiểm tra trong DB).
- Live-step đa luồng:
  - CRYPTO 415 bot → **7.4s**, sinh **421 trade** per-symbol.
  - **Toàn bộ 4440 bot → 10.9s**, prediction-svc đỉnh **~238% CPU** (đa lõi), **~481MiB** RAM; DB nhẹ.
- api-svc không vỡ: leaderboard trả 4440 entry, monitoring HTTP 200.

## 8. Tài nguyên & công cụ đo

`scripts/svc-metrics.sh` + Makefile:
- `make stats` — snapshot mem/cpu/net/io/pids từng service + dòng TOTAL.
- `make stats-watch` — refresh mỗi 3s (log-friendly).
- `--json` cho scripting.

Baseline idle ~860MiB toàn stack; dưới tải live-step 4440 bot prediction-svc nhảy lên ~481MiB / ~2.4 lõi
trong ~11s rồi về idle.

## 9. Cách bootstrap / vận hành

1. Đảm bảo `PER_SYMBOL_ENABLED=true` trong `.env`, rebuild + up prediction-svc.
2. Seeder tự chèn bot per-symbol lúc startup.
3. Per-symbol model được train qua: (a) pipeline crawl mỗi 10 lần (`train_for_market` đã hook), hoặc
   (b) cron `train_*` Chủ nhật, hoặc (c) `POST /api/trigger/train` (admin). Trước khi có model, per-symbol
   prediction skip (cold-start) và bot per-symbol idle — **không lỗi**.
4. Sau khi có prediction `__ps`, bot per-symbol bắt đầu trade ở live-step (job `simulation_daily` 8PM,
   hoặc `POST /api/trigger/simulation-live-step`).

## 10. Out of scope (lần này)

- UI riêng cho per-symbol (dashboard cũ vẫn liệt kê `__ps` như thuật toán thường, leaderboard có cột symbol).
- Per-symbol cho `ensemble` (sẽ tự bật khi đủ direction-accuracy).
- NASDAQ/SP500 chưa được train per-symbol trong phiên verify (chỉ CRYPTO) — sẽ tự train theo cron/pipeline.
