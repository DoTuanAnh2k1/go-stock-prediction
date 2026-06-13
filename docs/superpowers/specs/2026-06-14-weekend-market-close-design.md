# Thiết kế: Chặn NASDAQ & SP500 vào cuối tuần và ngày lễ US

**Ngày:** 2026-06-14
**Trạng thái:** Đã duyệt thiết kế, chờ implementation plan

## Bối cảnh / Vấn đề

Sàn NASDAQ và S&P 500 không giao dịch vào Thứ 7, Chủ nhật và các ngày lễ thị
trường Mỹ (NYSE). Hiện tại pipeline vẫn chạy đầy đủ mọi ngày:

- `crawler_nasdaq` (`0 15 * * * *`) và `crawler_sp500` (`0 0,30 * * * *`) chạy
  `_run_pipeline()` → crawl → train (mỗi 10 lần) → predict → `_trigger_sim_step()`
  (bot trade ngay sau predict).
- `simulation_daily` (8PM) gọi `engine.run_live_step()` cho **tất cả** bot đang
  active, không phân biệt market.

Kết quả: cuối tuần/ngày lễ vẫn crawl giá đứng yên, sinh prediction rác và để bot
NASDAQ/SP500 "trade" trên dữ liệu không đổi.

GOLD và CRYPTO giao dịch cả cuối tuần nên **không** bị ảnh hưởng.

## Mục tiêu

Vào cuối tuần (Thứ 7/CN) và ngày lễ NYSE, NASDAQ + SP500:
- Không crawl
- Không predict
- Không cho bot trade (cả pipeline lẫn job bot 8PM)

GOLD + CRYPTO chạy bình thường.

## Quyết định thiết kế

- **Cách chặn:** kết hợp cron (Mon–Fri) + code guard (robust, không bị ảnh hưởng
  khi user sửa cron qua Settings).
- **Phạm vi:** cuối tuần **và** ngày lễ thị trường Mỹ.
- **Nguồn ngày lễ:** hàm tự viết `_nyse_holidays(year)` — không thêm dependency.
- **Cơ sở thời gian:** lịch NYSE theo **US/Eastern**. Đây là cổng chính xác cho
  "thị trường Mỹ có mở không". Cron `1-5` (giờ VN) chỉ là bộ lọc thô để service
  khỏi wake dậy không cần thiết.

## Kiến trúc

### 1. Module mới: `prediction/src/utils/market_calendar.py`

Nguồn sự thật duy nhất về việc một market có đang mở cửa hay không.

```python
def is_market_open(market_key: str, when: datetime | None = None) -> bool
```

- Chuẩn hóa `market_key` về upper. Chấp nhận cả `NASDAQ` (bot.market) lẫn
  `NASDAQ100` (orchestrator key).
- `GOLD`, `CRYPTO`, hoặc bất kỳ key nào không thuộc nhóm theo lịch NYSE → `True`.
- `NASDAQ`/`NASDAQ100`, `SP500` → quy đổi `when or now()` sang ngày US/Eastern
  (qua `zoneinfo.ZoneInfo("America/New_York")`, stdlib Python 3.12) rồi trả về
  `_is_us_trading_day(et_date)`.

Hàm phụ:

```python
_NYSE_MARKETS = {"NASDAQ", "NASDAQ100", "SP500"}

def _is_us_trading_day(d: date) -> bool:
    # False nếu Thứ 7 (5) / CN (6) hoặc d ∈ _nyse_holidays(d.year)

def _nyse_holidays(year: int) -> set[date]:
    # New Year (1/1), MLK (Thứ 2 thứ 3 của tháng 1),
    # Presidents (Thứ 2 thứ 3 tháng 2), Good Friday (Thứ 6 trước Easter — computus),
    # Memorial (Thứ 2 cuối tháng 5), Juneteenth (19/6), Independence (4/7),
    # Labor (Thứ 2 đầu tháng 9), Thanksgiving (Thứ 5 thứ 4 tháng 11),
    # Christmas (25/12).
    # Quy tắc observed: nếu lễ ngày-cố-định rơi vào CN → quan sát Thứ 2;
    # rơi vào Thứ 7 → quan sát Thứ 6.
```

Helper nội bộ cần: `_nth_weekday(year, month, weekday, n)`,
`_last_weekday(year, month, weekday)`, `_easter(year)` (thuật toán computus của
Gauss/Anonymous) cho Good Friday = Easter − 2 ngày.

### 2. Ba điểm guard (defense-in-depth)

| Vị trí | Hành vi khi market đóng |
|--------|------------------------|
| `jobs.py::_run_pipeline` (đầu hàm) | `log.info("pipeline.skip.market_closed", market=...)` + `return` — bỏ qua toàn bộ crawl → train → predict → sim |
| `runner.py::run_for_market` (đầu hàm) | `log` + `return 0` — chặn standalone predict, `run_all_markets`, và `_trigger_sim_step` |
| `engine.py::run_live_step` (job bot 8PM) | Lọc bỏ các `bot_configs` có `market` đang đóng trước khi `_run_bot_steps` |

Mỗi guard gọi cùng hàm `is_market_open`. `_run_pipeline` dùng `market_key`
(`"NASDAQ100"`, `"SP500"`); `run_live_step` dùng `db_bot.market` (`"NASDAQ"`,
`"SP500"`) — cả hai đều được `is_market_open` chuẩn hóa.

### 3. Cron `DEFAULT_SCHEDULES` (`prediction/src/scheduler/manager.py`)

| Job key | Cũ | Mới |
|---------|-----|-----|
| `crawler_nasdaq` | `0 15 * * * *` | `0 15 * * * 1-5` |
| `crawler_sp500` | `0 0,30 * * * *` | `0 0,30 * * * 1-5` |

Các training job (`train_nasdaq`, `train_sp500` — chạy Chủ nhật) **giữ nguyên**:
chỉ fit lại model offline từ data có sẵn, không trade, vô hại.

> `upsert_cron_schedule()` chạy true-upsert mỗi lần Python service khởi động nên
> giá trị mới sẽ ghi đè DB tự động.

## Luồng dữ liệu sau thay đổi

- **Ngày thường (US trading day):** không đổi — crawl → predict → bot như cũ.
- **Cuối tuần/ngày lễ US:**
  - Cron `1-5` không kích hoạt `crawler_nasdaq`/`crawler_sp500` vào Thứ 7/CN (giờ VN).
  - Nếu vẫn kích hoạt (vd. user sửa cron, hoặc vùng biên giờ ET), guard trong
    `_run_pipeline`/`run_for_market` chặn.
  - Job bot 8PM lọc bỏ bot NASDAQ/SP500.
  - GOLD + CRYPTO không bị chặn.

## Xử lý lỗi

- `is_market_open` không truy cập mạng, chỉ tính toán ngày — không ném lỗi trong
  điều kiện bình thường. Nếu có exception bất ngờ, fail-open không áp dụng: hàm
  được viết đơn giản, thuần số học, nên rủi ro thấp.
- Các guard chỉ thêm một nhánh `return` sớm, không thay đổi luồng lỗi hiện có.

## Testing

`prediction/tests/unit/test_market_calendar.py`:
- GOLD, CRYPTO → luôn `True` (kể cả Thứ 7/CN).
- NASDAQ100/SP500 → `False` vào một ngày Thứ 7 và một ngày CN xác định.
- NASDAQ100/SP500 → `True` vào một ngày thường xác định.
- Ngày lễ tiêu biểu (vd. Christmas 2025-12-25, Independence Day, New Year,
  Good Friday) → `False`.
- Chuẩn hóa key: `"NASDAQ"` và `"NASDAQ100"` cho cùng kết quả.

## Out of scope (YAGNI)

- Early-close (nửa phiên) của NYSE — pipeline chỉ quan tâm mở/đóng theo ngày.
- Lịch giao dịch riêng của vàng SJC (giờ làm việc cửa hàng) — ngoài yêu cầu.
- UI hiển thị trạng thái "thị trường đóng cửa" — có thể thêm sau nếu cần.
