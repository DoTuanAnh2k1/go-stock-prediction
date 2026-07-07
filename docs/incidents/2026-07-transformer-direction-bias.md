# 2026-07-07 — transformer_nn dự đoán "giảm" gần như mọi lúc (direction accuracy dưới 50%)

## Triệu chứng

Sau ~1 tháng chạy, người dùng phản ánh "chưa thấy kết quả". Điều tra direction
accuracy từng thuật toán cho thấy `transformer_nn` (thuật toán #13, PatchTST-lite,
thêm ở commit `7579de6`) nằm **dưới mức tung đồng xu** trên 3/4 thị trường:

| Market | acc | pred_up (tỷ lệ đoán tăng) | act_up (thực tế tăng) | n |
|--------|-----|------|------|---|
| SP500  | 0.344 | **0.047** | 0.674 | 215 |
| CRYPTO | 0.361 | **0.196** | 0.722 | 158 |
| NASDAQ | 0.459 | **0.034** | 0.568 | 266 |
| GOLD   | 0.632 | 0.211 | 0.368 | 38 |

Nó đoán "giảm" 95%+ mọi lúc. Chỉ "trông đẹp" trên GOLD vì GOLD **rơi** trong khoảng
mẫu ngắn của nó. Toàn bộ 12 bot lỗ nặng nhất trên leaderboard đều là `transformer_nn`.

## Root cause

Direction head được train bằng nhãn **sai ngưỡng**. Trong `train_batch_labeled`
(và quick-train path), return được chuẩn hóa trước khi tạo nhãn:

```python
z = (rets - self._r_mean) / self._r_std     # return chuẩn hóa (trừ mean)
y = z[i + SEQ_LEN]                           # target regression
ytr_dir = (ytr_t > 0).float()               # BUG: (y_std > 0) == (ret > r_mean)
```

`y_std > 0` tương đương `ret > r_mean` (trên return **trung bình**), KHÔNG phải
`ret > 0`. Nhưng hệ thống chấm điểm hỏi `ret > 0` (`direction_correct := actual > current`).
Với thị trường có drift dương (r_mean > 0), model bị dạy đòi hỏi cao hơn để gọi
"tăng" → lệch có hệ thống về "giảm" → dưới chance khi thị trường đi lên.

## Fix

`transformer_model.py` — thêm helper `_direction_targets(y_std, r_mean, r_std)` trả
`y_std > -r_mean/r_std` (ngưỡng trong không gian chuẩn hóa tương ứng `ret > 0`, vì
`ret > 0 ⟺ z > -r_mean/r_std`). Áp cho cả pooled training (`ytr_dir`/`yva_dir` +
logging `dir_acc`) lẫn quick-train (`y_dir`). Prediction path không đổi
(`direction = 1 if p_up>=0.5 else -1` vẫn đúng sau khi head được train đúng ngưỡng).

Test: `tests/unit/test_transformer.py::test_direction_targets_*` (TDD, red→green).
Retrain qua `POST /api/trigger/train {"algorithm":"transformer_nn"}` → checkpoint mới
`val_dir_acc` 0.53–0.54 (CRYPTO/SP500). Prediction live ngay sau fix hết kẹt "giảm".

## Bài học lớn hơn (không chỉ transformer)

Con số "direction accuracy" của phần lớn model **không đo kỹ năng** — nó đo xem bias
cố định của model có tình cờ khớp xu hướng của giai đoạn mẫu hay không. Bằng chứng:
`random_forest` GOLD đoán tăng chỉ 20% → sai 76% khi GOLD tăng (acc 23.6%), nhưng
về ~50% trên NASDAQ đi ngang. Các model regression-trên-return (RF/LGBM/XGB/transformer)
học hồi quy-về-trung-bình, nên trong uptrend chúng dự đoán ngược trend.

Edge THẬT (khiêm tốn) duy nhất: `ema`/`moving_average` (momentum) trên GOLD & CRYPTO.
Trên NASDAQ/SP500 mẫu lớn, gần như mọi thuật toán 47–53% = không edge — đúng bản chất
thị trường thanh khoản cao ở khung dự đoán giờ-kế-tiếp.

## Regenerate out-of-sample (2026-07-07, sau fix)

Theo yêu cầu: xóa toàn bộ prediction `transformer_nn` (backup ở bảng `bak_tf_*`),
train lại model đã sửa trên **intraday ≤ 28/6**, walk-forward predict +1h theo từng
bar intraday **29/6 → nay** (mirror regime production, không leakage), chấm trực tiếp.

| Market | dir_acc (fixed, OOS) | n scored | trước fix (buggy) |
|--------|------|------|------|
| GOLD      | **0.541** | 109 | 0.632 (n=38, chỉ do GOLD rơi) |
| CRYPTO    | **0.540** | 176 | 0.361 |
| NASDAQ100 | **0.489** | 726 | 0.459 |
| SP500     | **0.456** | 487 | 0.344 |

Hết kẹt "giảm" 95% (pred_up giờ biến thiên theo market, không còn constant), hết dưới-chance
bệnh lý. Kết luận không đổi: transformer đã sửa chỉ ~coin-flip, nhích trên chance ở GOLD/CRYPTO
(~54%), quanh/dưới 50% ở NASDAQ/SP500 — không phải máy in tiền, nhưng không còn gây hại.

Checkpoint production sau đó được retrain lại trên TOÀN BỘ data (backtest đã tạm ghi đè bằng
bản ≤28/6). Bảng backup `bak_tf_gold/nasdaq/crypto/sp500` giữ lại để phòng hờ — có thể `DROP` khi yên tâm.
