# `tactics/` — Chiến thuật giao dịch của bot

Thư mục này tài liệu hoá các **chiến thuật quyết định giao dịch (trading tactics)** mà bot simulation dùng để ra lệnh **mua / bán / giữ** và **chọn cỡ vị thế**.

## `tactics/` khác `algos/` ở đâu?

Hai thư mục giải quyết hai bài toán **khác tầng nhau**:

| | `algos/` | `tactics/` |
|---|---|---|
| **Bài toán** | *Giá giờ sau là bao nhiêu?* | *Biết các dự đoán đó rồi, nên làm gì?* |
| **Đầu ra** | Một con số `predicted_price` (+ confidence) | Một hành động `{BUY, SELL, HOLD}` + cỡ lệnh |
| **Đầu vào** | Chuỗi giá lịch sử | **Dự đoán của nhiều `algos/`** + lịch sử độ chính xác của chúng + trạng thái vị thế |
| **Tầng** | Dự đoán (prediction) | Quyết định (decision / simulation) |
| **Ví dụ** | `moving_average`, `lstm_nn`, `egarch`, … (01–12) | `meta-stacking`, `meta-rl` |

Nói ngắn gọn: **`algos/` đoán giá; `tactics/` đọc các dự đoán đó rồi quyết định giao dịch.** Một chiến thuật trong `tactics/` là **lớp nằm trên** (meta-layer) toàn bộ `algos/` — nó không tự đoán giá, mà tổng hợp ý kiến của các thuật toán dự đoán + biết thuật toán nào gần đây đáng tin để ra quyết định.

> **Lưu ý đặt tên:** file trong `tactics/` đặt tên **mô tả**, **không đánh số** (khác `algos/` đánh số 01–12). Lý do: chúng không phải "thuật toán dự đoán thứ N", mà là các chiến thuật độc lập về khái niệm.

## Vì sao cần `tactics/`?

Bot mặc định quyết định bằng luật ngưỡng tĩnh trên **đúng 1 thuật toán**: "thuật toán dự đoán xanh thì mua, đỏ thì bán, vượt một ngưỡng cố định". Cách này có ba điểm yếu:

1. **Tin mù 1 thuật toán** — không biết thuật toán đó *gần đây* đoán đúng hay sai (dù dữ liệu đó đã có sẵn).
2. **Không hội ý** — không có cơ chế đọc *đồng thời* tất cả dự đoán rồi cân nhắc.
3. **Ngưỡng tĩnh, vào lệnh full** — không theo độ biến động thị trường, không size theo độ tin.

Các chiến thuật trong thư mục này là những cách *thông minh hơn* để biến "một rổ dự đoán" thành "một quyết định giao dịch".

## Tài liệu hiện có

| File | Chiến thuật | Trạng thái |
|---|---|---|
| [meta-stacking.md](meta-stacking.md) | **Meta-Stacking** — mô hình supervised (LightGBM) đọc tất cả dự đoán + độ chính xác lịch sử → xác suất giá tăng đã hiệu chỉnh → quyết định + size theo độ tin | Đang triển khai |
| [meta-rl.md](meta-rl.md) | **Meta-RL** — biến thể Reinforcement Learning của Meta-Stacking: nhồi vector dự đoán vào trạng thái của một agent DQN | Thiết kế tương lai, **chưa triển khai** |
