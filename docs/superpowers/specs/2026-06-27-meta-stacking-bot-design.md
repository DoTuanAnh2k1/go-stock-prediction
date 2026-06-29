# Meta-Stacking Bot — Thiết kế (spec)

**Ngày:** 2026-06-27
**Trạng thái:** Đã chốt thiết kế, chờ review trước khi lập kế hoạch triển khai.
**Loại:** Chiến thuật giao dịch cho bot (KHÔNG phải thuật toán dự đoán). Sống ở tầng simulation.

---

## 1. Bối cảnh & vấn đề

Bot simulation hiện tại quyết định theo luật ngưỡng tĩnh trên dự đoán của **đúng 1 thuật toán** (`simulation/signal.py → _derive_signals`):

```
signal_strength = (predicted - current) / current * 100
BUY  nếu signal_strength >  buy_threshold  AND confidence > min_confidence
SELL nếu signal_strength < -sell_threshold AND confidence > min_confidence
HOLD còn lại
```

Hạn chế: (a) mỗi bot tin mù 1 algo, không biết algo đó gần đây đúng/sai bao nhiêu dù đã có sẵn cột `direction_correct`; (b) không bot nào đọc đồng thời tất cả dự đoán rồi mới quyết; (c) ngưỡng tĩnh không theo regime; (d) vào lệnh full bất kể độ tin.

## 2. Bước 0 — Direction accuracy thật (đo 2026-06-27)

Đo từ `direction_correct` (chỉ hàng đã reconcile, ≥20 mẫu/algo):

| Market | Tốt nhất | Tệ nhất | Nhận xét |
|---|---|---|---|
| GOLD | egarch **62.5%** | lightgbm/xgboost/random_forest **~25%** | Phân tán cực mạnh |
| CRYPTO | rl_dqn 54.5% | ema 37.3% | egarch/moving_average ~54% |
| NASDAQ | ema 55.4% | gru 45.4% | Cụm quanh 50% |
| SP500 | lightgbm 53.6% | egarch 45.3% | Cụm quanh 50% |

**Kết luận định hướng thiết kế:**
- Đa số algo dao động ~45–55% (gần ngẫu nhiên) → kỳ vọng meta thắng nhưng **biên nhỏ**; phí giao dịch theo giờ phải được tính vào.
- Độ chính xác **phân tán mạnh giữa các algo** → đây là nguyên liệu cho stacking: lớp quyết định biết "tin egarch, bỏ qua/đảo lightgbm-GOLD" sẽ ăn đứt việc cho mỗi algo 1 bot ngang nhau.
- GOLD lightgbm/xgboost/rf ~25% = sai có hệ thống → meta có thể **đảo dấu** để khai thác (cần soi xem có phải bug tín hiệu đảo).

## 3. Mục tiêu & phạm vi

Thêm **chiến thuật bot "meta-stacking"** đọc *tất cả* dự đoán + độ tin cậy lịch sử từng algo → ra **P(giá tăng giờ kế tiếp)** đã hiệu chỉnh → quyết định + **size theo độ tin**. Tất cả bot dùng **vốn khởi đầu $1000**, chạy chung leaderboard để so trực tiếp.

### 3.1 Các bot sinh ra (mirror pattern pooled/`__ps` của rl_dqn)

| Biến thể | Key (sim_bots.algorithm) | Số bot | Checkpoint |
|---|---|---|---|
| Pooled | `meta_stack` | 4 (1/market) | `${RL_MODEL_DIR}/meta_{market}.pkl` |
| Per-symbol (gate `PER_SYMBOL_ENABLED=true`) | `meta_stack__ps` | = số mã tradable/market | `${RL_MODEL_DIR}/meta_{market}_{symbol}.pkl` |

So sánh trên leaderboard: 11 algo pooled vs `meta_stack` pooled; per-symbol vs per-symbol. Đây chính là "số liệu so sánh" mong muốn.

### 3.2 Ngoài phạm vi (YAGNI)

- KHÔNG thêm bảng prediction mới, KHÔNG migration cho prediction (meta-bot tự đọc `*_predictions` lúc chạy, giống `_step_rl`).
- KHÔNG đăng ký vào predict registry (`algorithms.go` / `build_algorithms()`) — đây là tactic, không phải thuật toán thứ 13.
- RL meta (phương án 3) **chỉ viết tài liệu toán**, chưa code.

## 4. "Bộ não" supervised (bản triển khai)

**Tách bạch vai trò:** classifier chỉ học **P(up)** độc lập vị thế; bot bọc ngoài lo quyết định + size + SL/TP. Tách để tránh leakage và dễ validate.

### 4.1 Feature vector tại thời điểm t, mã j (market m)

```
x_t = [ g_{t,1}..g_{t,A}        # % thay đổi mỗi algo dự đoán
      ‖ w_{t,1}..w_{t,A}        # direction accuracy rolling gần đây mỗi algo
      ‖ sigma_t, mom_t ]        # regime: volatility + momentum ngắn
```

- `g_{t,a} = (pred_{t,a} - p_t) / p_t` — lấy từ `*_predictions` (các algo có dự đoán cho (mã, timestamp) đó). A = số algo khả dụng.
- `w_{t,a}` = direction accuracy **rolling K lần gần nhất** (mặc định K=40) của algo a, **chỉ tính từ prediction đã reconcile có `target_date < t`** (chống leakage). Thiếu lịch sử → điền 0.5.
- `sigma_t` = std log-return cửa sổ 20; `mom_t` = momentum ngắn (vài return gần nhất). Giữ ít feature regime để chống overfit.

### 4.2 Nhãn

`y_t = 1 nếu p_actual_{t+1h} > p_t, else 0`. Lấy từ giá thật (reconcile đã có); fallback suy ngược từ `direction_correct` của bất kỳ algo nào (`sign(actual−p) = sign(pred_a−p)` nếu dc_a=1, ngược lại đảo dấu).

### 4.3 Model

LightGBM binary classifier + **calibration isotonic** → `P(up)` đáng tin. Lưu checkpoint `.pkl`. Thiếu LightGBM/checkpoint → **fallback reliability-weighted vote** (heuristic: P(up) = trung bình có trọng số của sign(g_a) theo w_a, algo <0.45 bị đảo dấu) — pipeline không bao giờ vỡ.

## 5. Chính sách bot `_step_meta`

```
SL/TP cứng (lưới an toàn, giữ nguyên như mọi bot)
p = P(up | x_t)                      # model đã calibrate (hoặc fallback)
nếu p > 0.5 + δ:        BUY, size = clamp((p−0.5)/0.5, 0..1) × max_position_pct
nếu p < 0.5 − δ và đang giữ vị thế:  SELL
ngược lại:             HOLD
```

- δ ≈ `buy_threshold/100` (dùng lại config sẵn) để tránh lật lệnh quanh 50/50.
- **Thay đổi cô lập, backward-compatible:** thêm tham số tùy chọn `position_pct` vào `Portfolio.buy()`. Bot thường không truyền → hành xử y như cũ; meta-bot truyền để size theo conviction.

## 6. Training pipeline + chống leakage

`train_meta_for_market(market, symbol=None)`:

1. Gom hàng prediction **căn theo (mã, timestamp)** → dựng `g_{t,a}`; tính `w_{t,a}` **as-of t** (chỉ dùng dc của target chín trước t); regime as-of t; nhãn từ giá t+1h.
2. **Walk-forward theo thời gian (KHÔNG shuffle):** train `[0, t_split)`, test `[t_split, end)`. Báo cáo **out-of-sample direction accuracy + calibration curve + Brier score**.
3. Calibrate trên slice held-out → lưu checkpoint.
4. Pooled = gộp mọi mã của market; per-symbol = từng mã, guard `PER_SYMBOL_MIN_POINTS`.

Cron: thêm `train_meta` (Chủ nhật, sau các train khác) + trigger tay. (Tận dụng cơ chế cron DB-backed sẵn có.)

## 7. Tích hợp code (theo điểm mở rộng có sẵn)

- `prediction-svc/src/simulation/meta_stack.py` — `MetaStackModel` (build_features / train / predict_proba / save / load / fallback vote). **Tầng simulation, KHÔNG phải `algorithms/`.**
- `prediction-svc/src/simulation/bot.py` — thêm nhánh `is_meta` (base_key == "meta_stack") → `_step_meta`, song song `is_rl`. Tái dùng `_get_symbols_and_prices_as_of` / `_get_price_history_as_of`.
- `prediction-svc/src/orchestrator/training.py` (hoặc tương đương) — `train_meta_for_market`.
- Seeder simulation — thêm bot `meta_stack` (+ `meta_stack__ps` khi `PER_SYMBOL_ENABLED`).
- Leaderboard/monitoring **tự hiện** vì là rows `sim_bots` (không cần sửa API/web).

## 8. Tài liệu (deliverable)

Thư mục **mới `tactics/`** (chiến thuật giao dịch của bot — KHÁC `algos/` là thuật toán dự đoán). Tên mô tả, **không đánh số**:

- `tactics/README.md` — giải thích `tactics/` là gì, phân biệt với `algos/`.
- `tactics/meta-stacking.md` — toán bản supervised (viết từ số 0, tầm `algos/12_rldqn.md`): công thức `w_{t,a}`, stacking, calibration, sizing, leakage guards, fallback vote.
- `tactics/meta-rl.md` — **toán RL meta (phương án 3)**, đúng format `algos/12_rldqn.md` (MDP/Bellman/Double-DQN/ε-greedy/reward y như rl_dqn), **chỉ khác observation**:
  ```
  s_t = [ phi_t ‖ g_t ‖ w_t ‖ (rho_t, u_t, h_t) ]
  ```
  nhồi thêm vector dự đoán `g_t` và accuracy `w_t` của các algo vào trạng thái; reward = PnL − phí như cũ. **Doc-only, chưa code.**

## 9. Validation — "xem con số"

Train xong → `POST /api/trigger/simulation-backtest` → leaderboard hiện ngay **return% / Sharpe / win-rate / max drawdown** của `meta_stack` (và `__ps`) cạnh 11 bot per-algo, cùng mốc $1000. Đây là phép so trực tiếp.

## 10. Rủi ro & giả định

- **Biên lợi thế nhỏ:** nhiều algo ~50% → meta có thể chỉ nhỉnh hơn; phí giao dịch theo giờ có thể nuốt phần lợi → cần đo PnL ròng, không chỉ direction accuracy.
- **Per-symbol đói data:** ít điểm/mã → meta per-symbol có thể thua pooled. Chấp nhận — đó là số liệu so sánh cần thấy.
- **Leakage là rủi ro số 1:** mọi `w_{t,a}` và regime phải as-of t; split theo thời gian. Nếu làm sai, con số backtest đẹp giả tạo.
- **Phụ thuộc:** LightGBM (đã có trong `[ml]` extras); thiếu → fallback vote.

## 11. Tiêu chí hoàn thành

- 4 bot `meta_stack` (+ `__ps` khi bật) xuất hiện trên leaderboard, vốn $1000.
- `train_meta_for_market` chạy được, lưu/nạp checkpoint, báo cáo out-of-sample DA + Brier.
- Backtest cho ra con số so sánh meta vs per-algo.
- Hai file tài liệu `tactics/meta-stacking.md` + `tactics/meta-rl.md` + `tactics/README.md`.
- Test: unit cho build_features (leakage as-of), policy `_step_meta`, fallback vote.
