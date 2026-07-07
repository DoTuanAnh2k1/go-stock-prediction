# Sự cố CRWD: split 4:1 chưa xử lý + tick giá rác (07/2026)

**Ngày:** 2026-07-03 → 07-04
**Trạng thái:** ĐÃ SỬA & verify xong
**Ảnh hưởng:** Bot `nasdaq_rl_dqn` hiển thị lỗ −13% (thực chất chỉ −1,6%); `nasdaq_crwd_meta_stack__ps` treo vị thế lỗ ảo.

---

## 1. Triệu chứng ban đầu

Khi check bot đang active, `nasdaq_rl_dqn` đứng cuối bảng với **return −13,07%** — tệ bất thường so với các bot NASDAQ khác (quanh −2% → +5%).

## 2. Điều tra — bóc từng lệnh

Session live 28044 của rl_dqn: 8 lệnh, tổng PnL −128,25. Một lệnh duy nhất gây ra gần như toàn bộ:

| Symbol | close_reason | PnL | % |
|--------|-------------|-----|---|
| **CRWD** | rl_signal | **−112,12** | **−74,74%** |
| các lệnh khác | — | −16,13 | (bình thường, ~±5%) |

→ **1 lệnh CRWD = 87% khoản lỗ.** Không có lệnh nào thực sự "sai" về mặt chiến thuật.

## 3. Nguyên nhân gốc — thực ra là HAI vấn đề tách biệt

Ban đầu tưởng là "tick rác", nhưng điều tra sâu (timeline giá CRWD) lật ngược:

### (a) Stock split 4:1 chưa điều chỉnh — cái lớn
CRWD chia tách cổ phiếu **4:1** ngày ~2026-07-01. Giá thật rơi từ ~762 xuống ~194:
```
07-01 02:00   762.45   ← pre-split
07-01 20:00   194.64   ← post-split (762 / 194 ≈ 3.93 → tỷ lệ 4:1)
```
- Yahoo trả **daily đã split-adjust sẵn** (~193) NHƯNG **intraday thô, chưa adjust** (~762) → hai feed lệch nhau.
- Bot mua CRWD @770 (giá pre-split, đọc từ bảng daily-live lúc đó còn giữ 770), **giữ vị thế qua split**. Simulation KHÔNG nhân số lượng ×4 → khi giá về 194 thật, hiển thị **lỗ ảo −74%** (thực ra hòa vốn vì số cổ phiếu đã ×4).

### (b) 1 bar giá rác thật — cái nhỏ
Một bar intraday `772.74 @ 07-02 03:00` (pre-split) lọt giữa vùng đã post-split (~193). Đây là glitch Yahoo — nhảy lên rồi bật lại ngay.

> **Bài học:** guard kiểu "chặn nhảy giá > X%" sẽ chặn NHẦM cả split thật (762→194 cũng là −74%). Phải phân biệt: **tick rác = nhảy rồi bật lại**, **split = nhảy rồi giữ nguyên**.

## 4. Đã sửa gì

### 4.1 Dọn dữ liệu (thủ công, đã làm)
- Xoá bar rác `772.74`.
- Gỡ vị thế CRWD mở hỏng của `meta_stack__ps` (xoá BUY chưa khớp → cash tự hoàn).
- Unwind round-trip hỏng của `rl_dqn` (xoá BUY+SELL lỗ ảo) → recompute KPI → **return thật = −1,61%** (không phải −13%).

### 4.2 Phòng thủ chống tick rác — `crawlers/sanity.py`
- **`check_update`** (gate daily-live, bảng simulation đọc giá): giá lệch ngoài băng bị GIỮ pending, **chỉ chấp nhận khi crawl kế tiếp báo lại cùng mức** (persistence confirmation). → Tick rác bị chặn, split thật cho qua sau 1 nhịp.
- **`batch_outlier_mask`** (gate intraday): so sánh HAI PHÍA với neighbor → chỉ loại spike cô lập, KHÔNG chặn nhầm điểm biên split.
- Ngưỡng market-aware: GOLD 0.30 / NASDAQ·SP500 0.40 / CRYPTO 0.80 (override env `CRAWL_MAX_TICK_DEVIATION_<MARKET>`).

### 4.3 Xử lý split đầy đủ
- **Bảng `stock_splits`**: source of truth idempotent.
- **Phát hiện**: crawler `_fetch` thêm `&events=split`, parse `events.splits` của Yahoo (authoritative: tỷ lệ + ex-date) → `record_split`.
- **`orchestrator/splits.py`**: chỉnh **CHỈ intraday** (Yahoo daily đã adjust — chỉnh nữa sẽ double-adjust hỏng). Boundary **phát hiện từ data** (điểm consecutive close rớt ~ratio), KHÔNG dùng ex-date, vì intraday lưu lẫn scale. Chia bar trước boundary cho ratio → chuỗi liên tục. Idempotent.
- **Simulation split-aware** (`engine.py` `_restore_portfolio_state`): vị thế mở entry trước split → `quantity × ratio`, `entry_price / ratio`. KHÔNG mutate trade lịch sử. **Đây là fix gốc cho lỗ ảo.**

## 5. Verify

- Backfill CRWD 4:1: boundary phát hiện đúng = `07-01 20:00`, chỉnh 3479 bar. Intraday sau đó **liên tục** (~190 xuyên split), **daily không bị đụng**.
- `_fetch('CRWD')` live parse đúng split 4:1 từ Yahoo.
- Tests: 20 split + 43 sanity + **701 unit pass, 0 fail**.
- Rebuild exit 0, container khỏe, split trong DB `applied=true`.

## 6. Giới hạn đã biết

- **Closed cross-split trades** (đã SELL trước khi sửa) không được re-realize — chỉ vị thế MỞ được điều chỉnh ở restore.
- Entry đúng ngày ex-date có thể mis-classify (so sánh theo DATE) — hiếm.
- `check_update` với **backfill lịch sử** (mỗi ngày upsert 1 lần): 1 ngày split thật trong batch có thể bị bỏ 1 lần, tự lành ở crawl live kế tiếp.
- Split handling chỉ áp NASDAQ/SP500 (gold/crypto không có split).

## 7. File liên quan

- `prediction-svc/src/crawlers/sanity.py` — guard tick rác
- `prediction-svc/src/orchestrator/splits.py` — apply split history adjustment
- `prediction-svc/src/simulation/engine.py` — `_restore_portfolio_state` split-aware
- `prediction-svc/src/crawlers/nasdaq.py` / `sp500.py` — detect split events
- `prediction-svc/src/database/repository.py` — `record_split`, `list_unapplied_splits`, `list_splits_for_symbol`, `mark_split_applied` + sanity gate trong `upsert_*_price`
- Bảng: `stock_splits` (`database.sql`)
- Tests: `tests/unit/test_crawl_sanity.py`, `tests/unit/test_splits.py`
