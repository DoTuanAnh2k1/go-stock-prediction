"""Market trading calendar — quyết định một market có đang mở cửa hay không.

NASDAQ và S&P 500 theo lịch NYSE: đóng vào Thứ 7, Chủ nhật và các ngày lễ thị
trường Mỹ. GOLD đóng cuối tuần (thị trường forex/commodity 24/5) nhưng không
theo ngày lễ NYSE. CRYPTO giao dịch 24/7 nên luôn True.

Cơ sở thời gian dùng US/Eastern (đúng nghĩa "thị trường Mỹ có đang giao dịch
không"). Không truy cập mạng, không phụ thuộc thư viện ngoài — ngày lễ NYSE được
tính động cho mọi năm.

is_market_open kiểm tra cả ngày lẫn giờ ET:
- NYSE markets (NASDAQ/SP500): trading day + 9:00 AM – 4:30 PM ET
  (9:00 AM: trước mở để crawl pre-open; 4:30 PM: 30 phút sau đóng cửa thực tế
  4:00 PM ET, đủ thời gian lấy giá đóng cửa chính thức — tránh crawl/predict
  lặp lại sau after-hours với data không đổi)
  Tương đương 8:00 PM – 3:30 AM ICT ngày hôm sau.
- GOLD: weekday only (no time restriction — commodity 24/5)
- CRYPTO: luôn True
"""
from __future__ import annotations

from datetime import date, datetime, time, timedelta
from zoneinfo import ZoneInfo

# NYSE/NASDAQ session window (ET): pre-open đến 30 phút sau đóng cửa thực tế
_NYSE_OPEN_ET = time(9, 0)
_NYSE_CLOSE_ET = time(16, 30)

# NASDAQ/SP500 đóng cuối tuần + ngày lễ NYSE.
_NYSE_MARKETS = {"NASDAQ", "NASDAQ100", "SP500"}

# GOLD đóng cuối tuần nhưng không theo ngày lễ NYSE (commodity toàn cầu).
_WEEKEND_ONLY_MARKETS = {"GOLD"}

_US_EASTERN = ZoneInfo("America/New_York")


def next_trading_day(from_date: date, market_key: str) -> date:
    """Returns the next trading day strictly after from_date for the given market."""
    key = (market_key or "").strip().upper()
    d = from_date + timedelta(days=1)
    if key in _NYSE_MARKETS:
        while not _is_us_trading_day(d):
            d += timedelta(days=1)
    elif key in _WEEKEND_ONLY_MARKETS:
        while d.weekday() >= 5:
            d += timedelta(days=1)
    return d


def is_market_open(market_key: str, when: datetime | None = None) -> bool:
    """True nếu market đang trong khung giờ giao dịch hợp lệ.

    CRYPTO (và mọi key không thuộc 2 nhóm trên) luôn True.
    GOLD trả False vào Thứ 7, Chủ nhật (không giới hạn giờ).
    NASDAQ/SP500 trả False khi: cuối tuần, ngày lễ NYSE,
      hoặc ngoài khung 9:00 AM – 8:00 PM ET.
    """
    key = (market_key or "").strip().upper()
    if key not in _NYSE_MARKETS and key not in _WEEKEND_ONLY_MARKETS:
        return True

    moment = when or datetime.now()
    # Quy đổi sang US/Eastern. Nếu `moment` naive (TZ=Asia/Ho_Chi_Minh do
    # service set), gán tzinfo VN trước khi đổi sang ET.
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=ZoneInfo("Asia/Ho_Chi_Minh"))
    et_moment = moment.astimezone(_US_EASTERN)
    et_date = et_moment.date()

    if key in _WEEKEND_ONLY_MARKETS:
        return et_date.weekday() < 5  # Thứ 2–6, không giới hạn giờ

    # NYSE: phải là ngày giao dịch VÀ trong khung giờ 9AM–8PM ET
    if not _is_us_trading_day(et_date):
        return False
    et_time = et_moment.time()
    return _NYSE_OPEN_ET <= et_time <= _NYSE_CLOSE_ET


def _is_us_trading_day(d: date) -> bool:
    """False nếu cuối tuần hoặc ngày lễ NYSE."""
    if d.weekday() >= 5:  # 5 = Thứ 7, 6 = Chủ nhật
        return False
    return d not in _nyse_holidays(d.year)


def _nyse_holidays(year: int) -> set[date]:
    """Tập ngày lễ NYSE (đã áp dụng quy tắc observed) cho một năm."""
    holidays: set[date] = set()

    # Ngày lễ cố định — áp dụng quy tắc observed khi rơi vào cuối tuần.
    for month, day in ((1, 1), (6, 19), (7, 4), (12, 25)):
        holidays.add(_observed(date(year, month, day)))

    # Ngày lễ theo thứ tự trong tuần.
    holidays.add(_nth_weekday(year, 1, 0, 3))    # MLK Day — Thứ 2 thứ 3 của tháng 1
    holidays.add(_nth_weekday(year, 2, 0, 3))    # Presidents' Day — Thứ 2 thứ 3 tháng 2
    holidays.add(_last_weekday(year, 5, 0))      # Memorial Day — Thứ 2 cuối tháng 5
    holidays.add(_nth_weekday(year, 9, 0, 1))    # Labor Day — Thứ 2 đầu tháng 9
    holidays.add(_nth_weekday(year, 11, 3, 4))   # Thanksgiving — Thứ 5 thứ 4 tháng 11

    # Good Friday — Thứ 6 trước Lễ Phục sinh (không áp dụng observed).
    holidays.add(_easter(year) - timedelta(days=2))

    return holidays


def _observed(d: date) -> date:
    """Quy tắc quan sát của NYSE: lễ rơi CN → quan sát Thứ 2; rơi Thứ 7 → Thứ 6."""
    if d.weekday() == 6:        # Chủ nhật
        return d + timedelta(days=1)
    if d.weekday() == 5:        # Thứ 7
        return d - timedelta(days=1)
    return d


def _nth_weekday(year: int, month: int, weekday: int, n: int) -> date:
    """Ngày `weekday` (0=Thứ 2) lần thứ `n` trong tháng."""
    first = date(year, month, 1)
    offset = (weekday - first.weekday()) % 7
    return first + timedelta(days=offset + (n - 1) * 7)


def _last_weekday(year: int, month: int, weekday: int) -> date:
    """Ngày `weekday` cuối cùng trong tháng."""
    if month == 12:
        last = date(year, 12, 31)
    else:
        last = date(year, month + 1, 1) - timedelta(days=1)
    offset = (last.weekday() - weekday) % 7
    return last - timedelta(days=offset)


def _easter(year: int) -> date:
    """Ngày Lễ Phục sinh (thuật toán computus Anonymous Gregorian)."""
    a = year % 19
    b = year // 100
    c = year % 100
    d = b // 4
    e = b % 4
    f = (b + 8) // 25
    g = (b - f + 1) // 3
    h = (19 * a + b - d - g + 15) % 30
    i = c // 4
    k = c % 4
    ll = (32 + 2 * e + 2 * i - h - k) % 7
    m = (a + 11 * h + 22 * ll) // 451
    month = (h + ll - 7 * m + 114) // 31
    day = ((h + ll - 7 * m + 114) % 31) + 1
    return date(year, month, day)
