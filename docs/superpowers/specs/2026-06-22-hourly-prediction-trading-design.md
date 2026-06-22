# Thiết kế: Dự đoán & giao dịch theo GIỜ cho NASDAQ, SP500, CRYPTO (GOLD-style)

- **Ngày:** 2026-06-22
- **Trạng thái:** Chờ duyệt
- **Phạm vi:** prediction-svc (Python), database.sql, api-svc (Go), web-svc (React)

## 1. Bối cảnh & vấn đề

Hiện NASDAQ, SP500, CRYPTO dự đoán **giá đóng cửa NGÀY GIAO DỊCH KẾ TIẾP** (`target_date` = next trading day / +1 ngày) trên chuỗi giá đóng cửa theo ngày. Hệ quả: direction accuracy chỉ chấm được sau khi qua ngày → người dùng thấy như "không hoạt động" trong ngày.

GOLD **đã** dự đoán **+1 GIỜ** và reconcile ngay trong giờ. Mục tiêu: đưa NASDAQ/SP500/CRYPTO về **đúng mô hình GOLD** (target +1h, reconcile bằng giá live mới nhất), và cho bot **trade + ghi đường vốn theo giờ**.

### Phát hiện nền tảng (đã xác minh trên DB & code)
- `gold_prices`, `nasdaq_prices`, `sp500_prices`, `crypto_prices` đều **1 dòng/ngày**, nhưng dòng "hôm nay" được **ghi đè giá mới nhất mỗi lần crawl** (crawl chạy theo giờ). Đây là "giá live".
- GOLD reconcile (`training.py:656`): lấy giá **gần target nhất theo thời gian** → chính là dòng live hôm nay → chấm được trong giờ.
- NASDAQ/SP500/CRYPTO reconcile (`training.py:690,699,...`): cố tình `_to_date()` bỏ phần giờ và chỉ lấy giá ở **ngày ≥ target** → buộc đợi qua ngày.
- Crypto crawl thực tế chạy **theo giờ** (`0 0 * * * *`), KHÔNG phải 2h như CLAUDE.md ghi (doc cũ).
- GOLD `predict()` dùng `target = now + timedelta(hours=1)` (`runner.py:125`).

## 2. Mục tiêu / Không làm

**Mục tiêu**
- NASDAQ/SP500/CRYPTO: `target = now + 1h`, reconcile bằng giá live mới nhất (giống GOLD), **giữ nguyên 12 thuật toán** và **giữ chuỗi input daily 270 điểm** (không đói dữ liệu).
- Bot trade theo tín hiệu giờ; **snapshot + trade lưu mốc GIỜ** (đường vốn mịn theo giờ).
- NASDAQ/SP500 chỉ predict/trade trong **giờ phiên Mỹ** (lúc có giá tươi). CRYPTO 24/7. GOLD giữ nguyên.

**Không làm**
- Không đổi GOLD (đã +1h).
- Không chuyển input sang bảng intraday (đã chọn GOLD-style, dùng chuỗi daily-live).
- Không tạo bảng prediction/sim mới (tái dùng bảng hiện có; `target_date` vốn đã TIMESTAMP).
- Không backfill lịch sử giờ.

## 3. Thiết kế

### Phase 1 — Dự đoán + reconcile theo giờ

**3.1 `runner.py`** — `_predict_nasdaq`, `_predict_sp500`, `_predict_crypto`:
- Đổi `target` từ `next_trading_day(...)` / `last_d + 1 day` → `now + timedelta(hours=1)` (mirror `_predict_gold`).
- Giữ nguyên đọc `get_*_prices_asc` (chuỗi daily-live), `current = giá cuối`.
- NASDAQ/SP500: bọc bằng guard giờ phiên (xem 3.3) — bỏ qua predict ngoài phiên. CRYPTO/GOLD không guard.

**3.2 `orchestrator/training.py`** — `reconcile_predictions()`, nhánh nasdaq/sp500/crypto:
- Bỏ logic `_to_date` + "first price on/after target_d".
- Thay bằng GOLD-style: chỉ reconcile khi `target_date <= now` (đã chín); `actual` = **giá live mới nhất** (dòng `get_*_prices_asc` mới nhất). Tính `direction_correct` như cũ (so `predicted-current` vs `actual-current`).
- Thêm điều kiện chín `target_date <= now` (cải tiến nhỏ so với GOLD để hourly có ý nghĩa, tránh chấm sớm).

**3.3 `utils/market_calendar.py`** — thêm `is_intraday_open(market_key, when=None) -> bool`:
- NASDAQ/SP500: True chỉ trong giờ phiên Mỹ quy đổi giờ VN (≈ 20:30–03:00 ICT), T2–T6, loại lễ NYSE (tái dùng logic lễ sẵn có).
- CRYPTO/GOLD: trả True (gold đã có `is_market_open` xử lý cuối tuần).
- Dùng bởi cả runner (3.1) và bot live-step (3.6).

**3.4 `scheduler/jobs.py`**: pipeline 3 market đã chạy theo giờ + crawl trước predict. Không đổi cron; chỉ đảm bảo predict bị skip khi `is_intraday_open=False` (NASDAQ/SP500).

### Phase 2 — Bot trade + đường vốn theo giờ

**3.5 Schema (`database.sql`, additive & idempotent — `ADD COLUMN IF NOT EXISTS`)**
- `sim_portfolio_snapshots`: thêm `snapshot_at TIMESTAMP`; tạo **unique index `uq_sim_snap_session_at (session_id, snapshot_at)`** (lệnh `CREATE UNIQUE INDEX` TÁCH RIÊNG, không chung transaction với DML — theo lưu ý Timescale trong CLAUDE.md). Giữ `snapshot_date` làm cột phân vùng hypertable. Backfill `snapshot_at = snapshot_date` cho dòng cũ.
- `sim_trades`: thêm `trade_at TIMESTAMP`; giữ `trade_date` phân vùng. Backfill `trade_at = trade_date`.

**3.6 `simulation/` (portfolio.py, bot.py, engine.py)**
- `Trade`/`Portfolio.snapshot()`: nhận `datetime` (giờ); vẫn xuất `trade_date`/`snapshot_date` (date) cho phân vùng + thêm `trade_at`/`snapshot_at` (datetime).
- `engine._upsert_snapshot`: ON CONFLICT theo `(session_id, snapshot_at)` (1 row/session/giờ).
- `engine.run_live_step_for_market`: dùng `now` (datetime, giờ) thay `today`; gate NASDAQ/SP500 bằng `is_intraday_open`.
- Backtest path giữ tương thích (snapshot_at = đầu ngày nếu chạy theo ngày).

**3.7 Go `api-svc`**
- `models_db/simulation.go`: thêm field `SnapshotAt`/`TradeAt` cho struct snapshot/trade.
- API leaderboard/monitoring/chart: expose mốc giờ; chart equity đọc theo `snapshot_at`.

**3.8 `web-svc`**
- Chart prediction-vs-actual + đường vốn bot: hiển thị theo giờ (trục thời gian dùng timestamp đầy đủ).

### Cross-cutting
- **Docs**: cập nhật CLAUDE.md (crypto crawl theo giờ; NASDAQ/SP500/CRYPTO horizon = +1h giống GOLD; bảng cron/horizon).
- GOLD bot cũng tự hưởng snapshot theo giờ nhờ đổi schema chung (nhất quán, không đổi hành vi predict GOLD).

## 4. Luồng dữ liệu (sau thay đổi)

```
crawl (theo giờ, trong phiên) → ghi đè dòng giá live
  → runner đọc chuỗi daily-live → 12 algo predict (+1h) → ghi *_predictions (target = giờ kế)
  → reconcile: prediction đã chín (target<=now) ↔ giá live mới nhất → direction_correct
  → sim live-step (giờ): trades + snapshot theo giờ
```

## 5. Edge cases
- **Ngoài giờ phiên (NASDAQ/SP500)**: không predict/trade (guard `is_intraday_open`) → không sinh prediction vô nghĩa.
- **Cận giờ đóng cửa**: prediction giờ cuối phiên có thể chấm với giá đứng yên (degenerate flat) — chấp nhận, giống GOLD cuối tuần.
- **Hypertable index**: tạo unique index theo lệnh riêng (Timescale rollback nếu chung block DML).

## 6. Kiểm thử
- Unit: `is_intraday_open` (trong/ngoài phiên, cuối tuần, lễ); reconcile maturity (`target<=now`) + chấm bằng giá live cho 3 market; `Portfolio.snapshot` + upsert → đúng 1 row/session/giờ.
- Integration: predict ghi `target_date = +1h`; reconcile set `direction_correct` ngay trong phiên (NASDAQ/SP500) và liên tục (CRYPTO); bot sinh nhiều trade/ngày + snapshot theo giờ.

## 7. Rollout
- Schema ALTER idempotent → an toàn với DB đang chạy.
- Prediction cũ (daily) còn trong bảng (vô hại); prediction mới là +1h.
- Sau deploy: trigger crawl + predict 3 market trong giờ phiên để seed.
- Theo workflow bắt buộc: test → docs → docker rebuild → verify (monitoring/overview, direction-accuracy).

## 8. Checklist file đụng tới
- `prediction-svc/src/orchestrator/runner.py` (target +1h ×3 + guard)
- `prediction-svc/src/orchestrator/training.py` (reconcile ×3)
- `prediction-svc/src/utils/market_calendar.py` (`is_intraday_open`)
- `prediction-svc/src/scheduler/jobs.py` (skip ngoài phiên)
- `prediction-svc/src/simulation/{portfolio.py,bot.py,engine.py}` (giờ)
- `database.sql` (snapshot_at, trade_at, index)
- `api-svc/pkg/models/models_db/simulation.go` + API sim/chart
- `web-svc` chart/equity theo giờ
- `prediction-svc/tests/...` + `CLAUDE.md`
