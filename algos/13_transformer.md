# Transformer (PatchTST) — Deep learning + báo cáo tài chính (giải thích từ số 0)

**Key:** `transformer_nn`
**Class:** `TransformerPredictor`
**File:** [prediction-svc/src/algorithms/transformer_model.py](../prediction-svc/src/algorithms/transformer_model.py)
**Stateful:** Có — `train_batch_labeled()` huấn luyện và **lưu checkpoint** (`${RL_MODEL_DIR}/transformer_{market}.pt`)
**Dữ liệu phụ:** bảng `stock_fundamentals` — báo cáo tài chính (P/E, EPS, tăng trưởng doanh thu, biên lợi nhuận, vốn hóa…) crawl hàng tuần từ Yahoo Finance qua `yfinance` (cron `crawler_fundamentals`, thứ Bảy 6AM)
**Không nằm trong Ensemble** (rollout theo giai đoạn, giống `rl_dqn` v1)

> **Cập nhật v2 (2026-07):** (1) **Dual head** — thêm head phân loại P(up) train chung với head regression (loss = Huber + 0.3·BCE); hướng dự đoán lấy từ P(up), độ lớn từ |r̂|; checkpoint cũ single-head vẫn load. (2) **Horizon-matched intraday** — train (`_collect_intraday_series_for_market`) VÀ predict (runner `_transformer_intraday_prices`) đều dùng hourly bars thay vì daily-live, khớp target +1h; fallback daily khi thiếu intraday. (3) **Selective prediction** — khi |P(up)−0.5| < 0.02 (coin-flip), kết quả mang `skip_write=True` và runner KHÔNG ghi row → direction accuracy chỉ đo trên các dự đoán có conviction. (4) Cron `train_transformer` Thứ 2/4/6 2:30AM refresh giữa tuần.

> Tài liệu này viết cho người chưa biết gì về Transformer. Chỉ cần hiểu vector, ma trận, và khái niệm "trung bình có trọng số".

---

## Phần A. Trực giác: tại sao lại là Transformer?

LSTM/GRU (thuật toán #3, #4) đọc chuỗi giá **tuần tự** — từng bước một, thông tin cũ phải "chuyền tay" qua từng bước để đến hiện tại, nên tín hiệu xa dễ bị mờ dần.

**Transformer** nhìn **toàn bộ chuỗi cùng lúc**. Cơ chế trung tâm là **attention**: với mỗi đoạn của chuỗi, mô hình tự hỏi *"những đoạn nào khác trong quá khứ liên quan nhất đến đoạn này?"* và lấy trung bình có trọng số theo mức liên quan đó. Trọng số này **học được** từ dữ liệu, không cố định như moving average.

**PatchTST** (2023) là biến thể cho chuỗi thời gian: thay vì đưa từng điểm giá làm một "từ", ta gom **PATCH_LEN=8 bước liền nhau thành một patch** — giống như đọc cả cụm từ thay vì từng chữ cái. Lợi ích: token mang ngữ nghĩa cục bộ (một "nhịp" của thị trường), chuỗi token ngắn hơn 8 lần nên attention rẻ và ổn định hơn.

### Điểm khác biệt lớn nhất: biết thêm "sức khỏe doanh nghiệp"

12 thuật toán trước chỉ nhìn **giá và volume**. `transformer_nn` được cung cấp thêm **báo cáo tài chính** của từng mã: P/E, EPS, tăng trưởng doanh thu/lợi nhuận, biên lợi nhuận, nợ/vốn, cổ tức, beta, vốn hóa. Trong huấn luyện gộp (pooled — mọi mã của market học chung một mô hình), vector này hoạt động như một "chứng minh thư" giúp mô hình phân biệt: cổ phiếu tăng trưởng P/E cao (NVDA) và cổ phiếu phòng thủ trả cổ tức (KO) có động lực giá **khác nhau** dù hình dạng chuỗi giá tương tự.

**Lưu ý trung thực:** hệ thống dự đoán **giờ kế tiếp** (`target = now + 1h`), còn báo cáo tài chính thay đổi theo quý. Fundamentals ở đây **không phải tín hiệu thời điểm** — nó là **ngữ cảnh tĩnh** (static context) giúp mô hình pooled tách biệt hành vi từng mã, tương tự "symbol embedding" nhưng có ý nghĩa kinh tế thật.

---

## Phần B. Dữ liệu vào và chuẩn hóa

1. **Chuỗi giá** → log-returns: $r_t = \ln(p_t / p_{t-1})$ — bất biến theo thang giá, nên BTC (60k USD) và cổ phiếu (50 USD) so sánh được.
2. **Chuẩn hóa returns**: $z_t = (r_t - \mu_r) / \sigma_r$ với $\mu_r, \sigma_r$ tính trên toàn bộ tập train và **lưu trong checkpoint** (dùng lại y hệt lúc dự đoán).
3. **Fundamentals** (11 trường theo `repository.FUNDAMENTAL_FIELDS`): vốn hóa lấy $\log_{10}$; chuẩn hóa bằng nan-mean/nan-std của tập train; **thiếu dữ liệu → 0 sau chuẩn hóa** (= giá trị trung bình). GOLD/CRYPTO không có báo cáo tài chính → toàn vector 0 → mô hình tự động chạy chế độ "chỉ nhìn giá".

Cửa sổ vào: **SEQ_LEN = 96 log-returns** (~96 phiên daily-live gần nhất). Nhãn: return chuẩn hóa của bước kế tiếp.

---

## Phần C. Kiến trúc mạng

```
returns z[t-95..t]  (96)                     fundamentals (11)
        │                                            │
  chia 12 patch × 8                            Linear → ReLU (16)
        │                                            │
  Linear 8→64  + positional embedding                │
        │                                            │
  TransformerEncoder × 2                             │
  (4 heads, FF 128, pre-norm, dropout 0.1)           │
        │                                            │
  mean-pool 12 token → (64)                          │
        └────────────── concat (80) ─────────────────┘
                            │
              Linear 80→64 → ReLU → Dropout → Linear 64→1
                            │
                      ŷ  (return chuẩn hóa dự đoán)
```

Tổng tham số ~110k — cỡ nhỏ, chạy tốt trên CPU.

---

## Phần D. Huấn luyện (`train_batch_labeled`)

Chạy **pooled per-market** (train tuần, Chủ nhật — cùng lịch `train_*` với các algo khác; hoặc `POST /api/trigger/train {"algorithm":"transformer_nn"}`).

1. Mỗi series (mỗi mã) → chuỗi windows (96 → 1). **10% windows MỚI NHẤT của từng series bị giữ lại làm validation** — split theo thời gian, không trộn lẫn, chống "học vẹt tương lai".
2. Mỗi window gắn vector fundamentals của mã đó (label `nasdaq/AAPL` → tra bảng theo `AAPL`).
3. AdamW (lr 1e-3, weight decay 1e-4), **Huber loss** (bớt nhạy outlier so với MSE), batch 256, tối đa 80 epoch.
4. **Early stopping**: dừng khi val loss không cải thiện 8 epoch liên tiếp; **giữ weights của epoch tốt nhất**. Log kèm `val_dir_acc` — tỷ lệ đoán đúng HƯỚNG trên tập validation.
5. Lưu checkpoint gồm: weights + config kiến trúc + $\mu_r, \sigma_r$ + mean/std fundamentals.

**Cảnh báo leakage có chủ đích chấp nhận:** bảng fundamentals chỉ giữ **snapshot mới nhất** (không phải point-in-time theo quý). Train hôm nay dùng P/E hôm nay cho cả windows quá khứ. Với các tỷ số biến động chậm, sai lệch này nhỏ; đổi lại schema và crawler đơn giản hơn nhiều.

---

## Phần E. Dự đoán (`predict`)

1. Cần ≥ **106 điểm giá** (SEQ_LEN + 10). Lazy-load checkpoint lần gọi đầu.
2. Lấy 96 returns cuối, chuẩn hóa bằng $\mu_r, \sigma_r$ **của checkpoint**; tra fundamentals theo `_context_symbol` (runner set cho từng mã NASDAQ/SP500) hoặc `_symbol_key` (bot per-symbol).
3. Forward → $\hat{y}$ → un-standardize: $\hat{r} = \hat{y}\sigma_r + \mu_r$, kẹp về $\pm 4\sigma_r$.
4. `predicted = current × exp(r̂)` rồi **market-aware clamp** như mọi thuật toán.
5. **Confidence** = $0.45 + 0.40\tanh(2|\hat{r}|/\sigma_{20})$ — dự đoán càng lớn so với biến động 20 bước gần nhất thì càng tự tin; kẹp [0.35, 0.85].
6. **Cold start** (chưa có checkpoint): train nhanh 15 epoch trên chính series đó (không fundamentals), confidence cố định 0.45, KHÔNG lưu checkpoint. Lỗi thật → raise (fail-loud policy).

---

## Phần F. Pipeline dữ liệu fundamentals

```
yfinance Ticker(sym).info          (thứ Bảy 6AM, cron crawler_fundamentals)
        │  trailingPE, forwardPE, priceToBook, trailingEps, revenueGrowth,
        │  earningsGrowth, profitMargins, debtToEquity, dividendYield, beta, marketCap
        ▼
stock_fundamentals  (1 row / (market_key, symbol), UPSERT ON CONFLICT)
        │  repo.get_fundamentals_map(market)  — cache in-process TTL 6h
        ▼
TransformerPredictor  (train: join theo label; predict: join theo _context_symbol)
```

- Crawler: [prediction-svc/src/crawlers/fundamentals.py](../prediction-svc/src/crawlers/fundamentals.py) — chỉ NASDAQ100 + SP500; ETF (QQQ/SPY) thiếu nhiều trường → NULL, hợp lệ.
- Trigger tay: hiện chạy qua scheduler; có thể gọi trực tiếp `FundamentalsCrawler().crawl()` trong container.

## Vị trí trong hệ thống

- Đăng ký: `registry.py` (pooled + per-symbol `transformer_nn__ps` khi `PER_SYMBOL_ENABLED`), metadata Go: `algorithms.go`.
- Bot simulation: nhánh native `is_conviction` trong `bot.py` — BUY khi P(up) ≥ max(0.5+buy_threshold/100, 0.52), SELL khi giữ và P(up) ≤ min(0.5−sell_threshold/100, 0.48); dùng `predict_direction_proba()` trên intraday history. Lý do: prediction hourly ±0.1–0.3% không bao giờ chạm threshold %-giá thang ngày của bot thường.
- Không trong Ensemble — sẽ cân nhắc thêm vào sau khi có track record direction accuracy.
