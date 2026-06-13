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


def _et(y, m, d, hour=12):
    """Một datetime giữa trưa giờ US/Eastern (tránh nhập nhằng vùng biên)."""
    return datetime(y, m, d, hour, tzinfo=_ET)


# ---------------------------------------------------------------------------
# GOLD / CRYPTO luôn mở
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("market", ["GOLD", "CRYPTO", "gold", "crypto"])
def test_gold_crypto_always_open(market):
    # 2025-06-14 là Thứ 7, 2025-06-15 là Chủ nhật
    assert is_market_open(market, _et(2025, 6, 14)) is True
    assert is_market_open(market, _et(2025, 6, 15)) is True


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
