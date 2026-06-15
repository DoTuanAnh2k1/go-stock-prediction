"""Unit tests for market_calendar.is_market_open.

No network, no DB, no Docker required — pure date logic.
"""
from __future__ import annotations

from datetime import datetime
from zoneinfo import ZoneInfo

import pytest

from src.utils.market_calendar import (
    _is_us_trading_day,
    _nyse_holidays,
    is_market_open,
)

_ET = ZoneInfo("America/New_York")


def _et(y, m, d, hour=12, minute=0):
    """Một datetime giờ US/Eastern (mặc định giữa trưa, tránh nhập nhằng vùng biên)."""
    return datetime(y, m, d, hour, minute, tzinfo=_ET)


# ---------------------------------------------------------------------------
# CRYPTO luôn mở (24/7)
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("market", ["CRYPTO", "crypto"])
def test_crypto_always_open(market):
    # 2025-06-14 là Thứ 7, 2025-06-15 là Chủ nhật
    assert is_market_open(market, _et(2025, 6, 14)) is True
    assert is_market_open(market, _et(2025, 6, 15)) is True


# ---------------------------------------------------------------------------
# GOLD — đóng cuối tuần, mở ngày thường (không theo ngày lễ NYSE)
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("market", ["GOLD", "gold"])
def test_gold_closed_weekend(market):
    assert is_market_open(market, _et(2025, 6, 14)) is False  # Thứ 7
    assert is_market_open(market, _et(2025, 6, 15)) is False  # Chủ nhật


@pytest.mark.parametrize("market", ["GOLD", "gold"])
def test_gold_open_weekday(market):
    assert is_market_open(market, _et(2025, 6, 13)) is True   # Thứ 6
    assert is_market_open(market, _et(2025, 6, 16)) is True   # Thứ 2


def test_gold_open_on_nyse_holiday():
    # Gold không theo ngày lễ NYSE — Labor Day 2025 (Thứ 2 1/9) vẫn mở
    assert is_market_open("GOLD", _et(2025, 9, 1)) is True


def test_unknown_market_defaults_open():
    assert is_market_open("FOREX", _et(2025, 6, 14)) is True


# ---------------------------------------------------------------------------
# NASDAQ / SP500 — cuối tuần đóng
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("market", ["NASDAQ", "NASDAQ100", "SP500", "sp500"])
def test_nyse_markets_closed_weekend(market):
    assert is_market_open(market, _et(2025, 6, 14)) is False  # Thứ 7
    assert is_market_open(market, _et(2025, 6, 15)) is False  # Chủ nhật


@pytest.mark.parametrize("market", ["NASDAQ", "NASDAQ100", "SP500"])
def test_nyse_markets_open_weekday(market):
    # 2025-06-13 là Thứ 6 (không phải lễ)
    assert is_market_open(market, _et(2025, 6, 13)) is True
    # 2025-06-16 là Thứ 2
    assert is_market_open(market, _et(2025, 6, 16)) is True


def test_nasdaq_key_normalization_matches():
    when = _et(2025, 6, 14)
    assert is_market_open("NASDAQ", when) == is_market_open("NASDAQ100", when)


# ---------------------------------------------------------------------------
# Ngày lễ NYSE
# ---------------------------------------------------------------------------

@pytest.mark.parametrize(
    "y,m,d",
    [
        (2025, 1, 1),    # New Year's Day
        (2025, 1, 20),   # MLK Day (Thứ 2 thứ 3 tháng 1)
        (2025, 2, 17),   # Presidents' Day
        (2025, 4, 18),   # Good Friday
        (2025, 5, 26),   # Memorial Day
        (2025, 6, 19),   # Juneteenth
        (2025, 7, 4),    # Independence Day
        (2025, 9, 1),    # Labor Day
        (2025, 11, 27),  # Thanksgiving
        (2025, 12, 25),  # Christmas
    ],
)
def test_nyse_holidays_closed(y, m, d):
    assert is_market_open("SP500", _et(y, m, d)) is False
    assert is_market_open("NASDAQ100", _et(y, m, d)) is False


def test_observed_holiday_shift():
    # 2021-07-04 rơi vào Chủ nhật → quan sát Thứ 2 2021-07-05
    from datetime import date
    hol = _nyse_holidays(2021)
    assert date(2021, 7, 5) in hol
    # 2022-12-25 rơi vào Chủ nhật → quan sát Thứ 2 2022-12-26
    assert date(2022, 12, 26) in _nyse_holidays(2022)


def test_trading_day_helper():
    from datetime import date
    assert _is_us_trading_day(date(2025, 6, 13)) is True   # Thứ 6
    assert _is_us_trading_day(date(2025, 6, 14)) is False  # Thứ 7
    assert _is_us_trading_day(date(2025, 12, 25)) is False  # Christmas


# ---------------------------------------------------------------------------
# NYSE time-of-day window (9:00 AM – 8:00 PM ET)
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("market", ["NASDAQ100", "SP500"])
def test_nyse_closed_before_window(market):
    # 7:00 AM ET — trước 9:00 AM, chưa trong window
    assert is_market_open(market, _et(2025, 6, 13, 7)) is False
    # 8:59 AM ET — sát giờ nhưng vẫn chưa vào
    assert is_market_open(market, _et(2025, 6, 13, 8)) is False


@pytest.mark.parametrize("market", ["NASDAQ100", "SP500"])
def test_nyse_open_during_window(market):
    # 9:00 AM ET — đúng giờ mở cửa window
    assert is_market_open(market, _et(2025, 6, 13, 9)) is True
    # 10:30 AM ET — giờ giao dịch
    assert is_market_open(market, _et(2025, 6, 13, 10)) is True
    # 4:30 PM ET — đúng giờ đóng window (inclusive boundary)
    assert is_market_open(market, _et(2025, 6, 13, 16, 30)) is True


@pytest.mark.parametrize("market", ["NASDAQ100", "SP500"])
def test_nyse_closed_after_window(market):
    # 4:31 PM ET — vừa qua window đóng cửa 4:30 PM
    assert is_market_open(market, _et(2025, 6, 13, 16, 31)) is False
    # 8:00 PM ET — sau after-hours, đóng
    assert is_market_open(market, _et(2025, 6, 13, 20)) is False
    # 8:01 PM ET — đêm, đóng
    assert is_market_open(market, _et(2025, 6, 13, 20, 1)) is False
    # 11:00 PM ET — đêm khuya, đóng
    assert is_market_open(market, _et(2025, 6, 13, 23)) is False


def test_gold_not_restricted_by_time():
    # GOLD không bị giới hạn giờ — 3 AM ET weekday vẫn open
    assert is_market_open("GOLD", _et(2025, 6, 13, 3)) is True
    assert is_market_open("GOLD", _et(2025, 6, 13, 23)) is True
