"""CoinGecko cryptocurrency price crawler (BTC, ETH, SOL).

OHLC strategy
-------------
CoinGecko free-tier OHLC endpoint: ``/coins/{id}/ohlc?vs_currency=usd&days=N``
Returns: ``[[timestamp_ms, open, high, low, close], ...]``

Bucket granularity enforced by CoinGecko (free tier):
  - days=1    → ~30-minute buckets
  - days=2–90 → 4-hour buckets
  - days>90   → daily (~4-day) buckets

For daily crawl we use ``days=1`` and take the *last* candle of the day as the
daily O/H/L — this matches how live dashboards typically show today's bar.
For history backfill we use ``days=max`` (e.g. 180) which gives daily buckets
for >90-day ranges; for shorter ranges we accept 4h buckets and aggregate to
the daily O/H/L in Python.

If the /ohlc call fails for any reason the pipeline continues with close_price
only and O/H/L are left NULL in the DB — graceful degradation.
"""
from __future__ import annotations

import time
from datetime import datetime, timezone
from decimal import Decimal
from zoneinfo import ZoneInfo

_VN_TZ = ZoneInfo("Asia/Ho_Chi_Minh")

import requests

from src.crawlers.base import DEFAULT_HEADERS, BaseCrawler
from src.database import repository as repo
from src.database.models import CryptoIntradayPrice
from src.utils.logger import get_logger

log = get_logger("crawler.crypto")

COINGECKO_SIMPLE_PRICE = (
    "https://api.coingecko.com/api/v3/simple/price"
    "?ids=bitcoin,ethereum,solana&vs_currencies=usd"
    "&include_market_cap=true&include_24hr_vol=true"
)
COINGECKO_HISTORY = (
    "https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
    "?vs_currency=usd&days=180&interval=daily"
)
COINGECKO_HOURLY = (
    "https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
    "?vs_currency=usd&days=2"
)
# OHLC endpoint: free tier, bucket depends on days param (see module docstring)
COINGECKO_OHLC = (
    "https://api.coingecko.com/api/v3/coins/{coin_id}/ohlc"
    "?vs_currency=usd&days={days}"
)

REQUEST_DELAY = 3  # 3s between coins for free tier rate limit

COINS = [
    ("bitcoin", "BTC"),
    ("ethereum", "ETH"),
    ("solana", "SOL"),
]


def _fetch_ohlc(
    session: requests.Session,
    coin_id: str,
    days: int,
    timeout: int,
) -> list[list]:
    """Fetch OHLC candles from CoinGecko.  Returns list of [ts_ms, o, h, l, c].
    Returns empty list on any error so callers can degrade gracefully."""
    try:
        url = COINGECKO_OHLC.format(coin_id=coin_id, days=days)
        resp = session.get(url, timeout=timeout)
        resp.raise_for_status()
        data = resp.json()
        if isinstance(data, list):
            return data
        log.warning("crypto.ohlc.unexpected_format", coin_id=coin_id, days=days)
    except Exception as exc:
        log.warning("crypto.ohlc.error", coin_id=coin_id, days=days, error=str(exc))
    return []


def _aggregate_ohlc_to_daily(
    candles: list[list],
) -> dict[str, tuple[Decimal, Decimal, Decimal, Decimal]]:
    """Group intraday OHLC candles into daily (ICT date) aggregated bars.

    For each trading_date in ICT timezone, returns:
        date_str -> (open_of_first_candle, max_high, min_low, close_of_last_candle)

    This lets us handle 4-hour and 30-minute buckets uniformly.
    """
    by_date: dict[str, list] = {}
    for row in candles:
        ts_ms, o, h, l, c = row[0], row[1], row[2], row[3], row[4]
        date_ict = (
            datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc)
            .astimezone(_VN_TZ)
            .date()
            .isoformat()
        )
        by_date.setdefault(date_ict, []).append((ts_ms, o, h, l, c))

    result: dict[str, tuple[Decimal, Decimal, Decimal, Decimal]] = {}
    for date_str, rows in by_date.items():
        rows.sort(key=lambda r: r[0])  # sort by timestamp ASC
        open_val = Decimal(str(rows[0][1]))
        high_val = max(Decimal(str(r[2])) for r in rows)
        low_val = min(Decimal(str(r[3])) for r in rows)
        close_val = Decimal(str(rows[-1][4]))
        result[date_str] = (open_val, high_val, low_val, close_val)
    return result


class CryptoCrawler(BaseCrawler):
    def __init__(self, timeout: int = 30) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        """Daily crawl: fetch close price via simple/price, then enrich with OHLC
        from /ohlc?days=1 (30-min buckets aggregated to today's bar)."""
        log.info("crypto.crawl.start")
        saved = 0
        errors = 0

        try:
            resp = self._session.get(COINGECKO_SIMPLE_PRICE, timeout=self._timeout)
            resp.raise_for_status()
            data = resp.json()

            today = datetime.now(tz=_VN_TZ).date()
            today_str = today.isoformat()

            for coin_id, symbol in COINS:
                coin_data = data.get(coin_id, {})
                if not coin_data:
                    continue

                price = float(coin_data.get("usd", 0) or 0)
                if price == 0:
                    continue

                market_cap = float(coin_data.get("usd_market_cap", 0) or 0)
                vol_24h = float(coin_data.get("usd_24h_vol", 0) or 0)

                # Fetch OHLC for today (days=1 → 30-min buckets, aggregate to daily bar)
                # sleep before the second request to respect rate limit
                time.sleep(REQUEST_DELAY)
                candles = _fetch_ohlc(self._session, coin_id, days=1, timeout=self._timeout)
                daily = _aggregate_ohlc_to_daily(candles)
                ohlc = daily.get(today_str)

                repo.upsert_crypto_price(
                    coin_id=coin_id,
                    symbol=symbol,
                    trading_date=today,
                    close_price=Decimal(str(price)),
                    market_cap=Decimal(str(market_cap)),
                    volume_24h=Decimal(str(vol_24h)),
                    currency="USD",
                    open_price=ohlc[0] if ohlc else None,
                    high_price=ohlc[1] if ohlc else None,
                    low_price=ohlc[2] if ohlc else None,
                )
                saved += 1

        except Exception as exc:
            log.warning("crypto.crawl.error", error=str(exc))
            errors += 1

        log.info("crypto.crawl.done", saved=saved, errors=errors)
        return saved

    def crawl_intraday(self) -> int:
        """Fetch last 2d of hourly close data from market_chart and enrich with
        OHLC from /ohlc?days=1 (30-min buckets) merged by nearest hour."""
        log.info("crypto.intraday.start")
        saved = 0

        for coin_id, _symbol in COINS:
            try:
                # --- close / mcap / volume from market_chart ---
                url = COINGECKO_HOURLY.format(coin_id=coin_id)
                resp = self._session.get(url, timeout=self._timeout)
                resp.raise_for_status()
                data = resp.json()

                prices_data = data.get("prices", [])
                mcaps_data = data.get("market_caps", [])
                volumes_data = data.get("total_volumes", [])

                # --- OHLC from /ohlc?days=1 ---
                # days=1 → CoinGecko returns ~30-min candles for the last ~24h.
                # We group them by truncated-to-hour timestamp so they can be
                # merged with the hourly close series from market_chart.
                time.sleep(REQUEST_DELAY)
                candles = _fetch_ohlc(self._session, coin_id, days=1, timeout=self._timeout)

                # Build hour-keyed OHLC lookup: hour_dt(naive ICT) -> (o, h, l, c)
                # For each hour bucket, aggregate all 30-min candles inside it.
                hour_ohlc: dict[datetime, tuple[Decimal, Decimal, Decimal, Decimal]] = {}
                by_hour: dict[datetime, list] = {}
                for row in candles:
                    ts_ms, o, h, l, c = row[0], row[1], row[2], row[3], row[4]
                    dt_hour = (
                        datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc)
                        .astimezone(_VN_TZ)
                        .replace(minute=0, second=0, microsecond=0, tzinfo=None)
                    )
                    by_hour.setdefault(dt_hour, []).append((ts_ms, o, h, l, c))

                for dt_hour, rows in by_hour.items():
                    rows.sort(key=lambda r: r[0])
                    hour_ohlc[dt_hour] = (
                        Decimal(str(rows[0][1])),           # open  = first candle
                        max(Decimal(str(r[2])) for r in rows),  # high = max
                        min(Decimal(str(r[3])) for r in rows),  # low  = min
                        Decimal(str(rows[-1][4])),          # close = last candle
                    )

                for i, (ts_ms, price) in enumerate(prices_data):
                    dt = datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc).astimezone(_VN_TZ).replace(
                        minute=0, second=0, microsecond=0, tzinfo=None
                    )
                    mcap = mcaps_data[i][1] if i < len(mcaps_data) else 0
                    vol = volumes_data[i][1] if i < len(volumes_data) else 0

                    ohlc = hour_ohlc.get(dt)

                    record = CryptoIntradayPrice(
                        coin_id=coin_id,
                        timestamp=dt,
                        open_price=ohlc[0] if ohlc else None,
                        high_price=ohlc[1] if ohlc else None,
                        low_price=ohlc[2] if ohlc else None,
                        price=Decimal(str(price)),
                        market_cap=Decimal(str(mcap or 0)),
                        volume=Decimal(str(vol or 0)),
                    )
                    repo.upsert_crypto_intraday(record)
                    saved += 1

                log.debug("crypto.intraday.coin", coin_id=coin_id, rows=len(prices_data))

            except Exception as exc:
                log.warning("crypto.intraday.error", coin_id=coin_id, error=str(exc))

            time.sleep(REQUEST_DELAY)

        log.info("crypto.intraday.done", saved=saved)
        return saved

    def crawl_history(self, days: int = 180) -> int:
        """Backfill historical daily data.

        Uses market_chart for close/mcap/volume (already working) and
        /ohlc?days=180 (free tier returns daily buckets for >90d range) for
        O/H/L per calendar date.  Both responses are merged by ICT trading date.

        CoinGecko free bucket note:
          days ≤ 1  → 30-min candles
          days 2-90 → 4-hour candles
          days > 90 → daily (~4-day) candles — effectively 1 candle per day
        For 180d this gives us 1 candle/day which maps cleanly to trading_date.
        """
        log.info("crypto.history.start")
        saved = 0
        errors = 0

        for coin_id, symbol in COINS:
            try:
                # --- close / mcap / volume ---
                url = COINGECKO_HISTORY.format(coin_id=coin_id)
                resp = self._session.get(url, timeout=self._timeout)
                resp.raise_for_status()
                data = resp.json()

                prices_raw = data.get("prices", [])
                market_caps = {entry[0]: entry[1] for entry in data.get("market_caps", [])}
                volumes = {entry[0]: entry[1] for entry in data.get("total_volumes", [])}

                # --- OHLC ---
                # For 180 days, CoinGecko free tier gives daily candles (>90d range).
                time.sleep(REQUEST_DELAY)
                candles = _fetch_ohlc(self._session, coin_id, days=days, timeout=self._timeout)
                daily_ohlc = _aggregate_ohlc_to_daily(candles)

                for ts_ms, price in prices_raw:
                    ts_sec = ts_ms / 1000
                    trading_date = (
                        datetime.fromtimestamp(ts_sec, tz=timezone.utc)
                        .astimezone(_VN_TZ)
                        .date()
                    )
                    ohlc = daily_ohlc.get(trading_date.isoformat())

                    repo.upsert_crypto_price(
                        coin_id=coin_id,
                        symbol=symbol,
                        trading_date=trading_date,
                        close_price=Decimal(str(price)),
                        market_cap=Decimal(str(market_caps.get(ts_ms, 0) or 0)),
                        volume_24h=Decimal(str(volumes.get(ts_ms, 0) or 0)),
                        currency="USD",
                        open_price=ohlc[0] if ohlc else None,
                        high_price=ohlc[1] if ohlc else None,
                        low_price=ohlc[2] if ohlc else None,
                    )
                    saved += 1

                log.debug("crypto.history.coin", coin_id=coin_id, rows=len(prices_raw))

            except Exception as exc:
                log.warning("crypto.history.error", coin_id=coin_id, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("crypto.history.done", saved=saved, errors=errors)
        return saved
