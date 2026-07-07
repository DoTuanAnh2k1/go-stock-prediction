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

from src.crawlers import sanity
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

# Binance REST base for klines (no auth required for public market data)
BINANCE_KLINES = "https://api.binance.com/api/v3/klines"

# Map CoinGecko coin_id → Binance base symbol (appended with USDT for the pair)
_COIN_TO_BINANCE: dict[str, str] = {
    "bitcoin": "BTCUSDT",
    "ethereum": "ETHUSDT",
    "solana": "SOLUSDT",
}

# Binance kline field indices (each element is a 12-item list)
# [0] openTime(ms), [1] open, [2] high, [3] low, [4] close,
# [5] volume(base), [6] closeTime(ms), [7] quoteAssetVolume, ...
_K_OPEN_TIME = 0
_K_OPEN = 1
_K_HIGH = 2
_K_LOW = 3
_K_CLOSE = 4
_K_VOLUME = 5  # base asset volume (e.g. BTC quantity traded)


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

                # Apply bilateral spike filter on the close-price sequence.
                price_vals = [float(p) for _, p in prices_data]
                spike_mask = sanity.batch_outlier_mask(price_vals, "CRYPTO")

                for i, (ts_ms, price) in enumerate(prices_data):
                    if spike_mask[i]:
                        dt_spike = datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc).astimezone(_VN_TZ).replace(
                            minute=0, second=0, microsecond=0, tzinfo=None
                        )
                        log.warning(
                            "crawl.sanity.spike_dropped",
                            symbol=coin_id,
                            ts=str(dt_spike),
                            price=float(price),
                        )
                        continue

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

    def backfill_intraday_binance(self, interval: str = "1h", years: int = 2) -> int:
        """Backfill many years of hourly OHLCV candles from Binance klines API.

        CoinGecko's free-tier hourly endpoint only covers ~2 days (``days=2``),
        making it unsuitable as a training source.  Binance's public klines
        endpoint has no such cap — it returns up to 1 000 candles per call and
        allows paging back years in time via the ``startTime`` parameter.

        For each coin the method pages forward in 1 000-candle windows starting
        from ``now - years`` until it reaches the present, calling
        ``repo.upsert_crypto_intraday(record)`` for every candle so the DB's
        unique constraint ``(coin_id, timestamp)`` acts as the dedup guard —
        existing rows are updated with the latest OHLCV values, new rows are
        inserted.

        Timestamp handling
        ------------------
        Binance returns ``openTime`` in UTC milliseconds.  The codebase stores
        ICT wallclock (``TIMESTAMP WITHOUT TIME ZONE``).  We convert exactly as
        ``crawl_intraday`` does::

            datetime.fromtimestamp(ms / 1000, tz=timezone.utc)
                    .astimezone(_VN_TZ)
                    .replace(minute=0, second=0, microsecond=0, tzinfo=None)

        The ``replace(tzinfo=None)`` strips the tzinfo so the stored value is a
        naive ICT wallclock, matching all other rows in the table.

        Fields populated on ``CryptoIntradayPrice``
        --------------------------------------------
        * ``coin_id``   — "bitcoin" / "ethereum" / "solana"
        * ``timestamp`` — naive ICT datetime (hour-truncated, matches CoinGecko rows)
        * ``open_price`` — kline open
        * ``high_price`` — kline high
        * ``low_price``  — kline low
        * ``price``      — kline close (the line-chart / close column; the model has
                           no separate ``close_price`` column on the intraday table)
        * ``market_cap`` — ``Decimal("0")`` (Binance klines do not carry mcap)
        * ``volume``     — kline base-asset volume (e.g. quantity of BTC traded)

        Parameters
        ----------
        interval:
            Binance kline interval string.  Default ``"1h"`` (hourly).
            Other valid values: ``"4h"``, ``"1d"``, etc.
        years:
            How many years of history to fetch.  Default ``2``.

        Returns
        -------
        int
            Total number of rows passed to ``upsert_crypto_intraday`` across
            all coins.
        """
        from datetime import timedelta

        log.info("crypto.binance_backfill.start", interval=interval, years=years)
        total = 0
        limit = 1000  # Binance max candles per request

        now_ts_ms = int(datetime.now(tz=_VN_TZ).timestamp() * 1000)
        start_offset_ms = int(timedelta(days=365 * years).total_seconds() * 1000)
        global_start_ms = now_ts_ms - start_offset_ms

        for coin_id, _symbol in COINS:
            binance_symbol = _COIN_TO_BINANCE.get(coin_id)
            if binance_symbol is None:
                log.warning("crypto.binance_backfill.unknown_coin", coin_id=coin_id)
                continue

            log.info(
                "crypto.binance_backfill.coin.start",
                coin_id=coin_id,
                symbol=binance_symbol,
                interval=interval,
            )
            coin_saved = 0
            coin_errors = 0
            start_ms = global_start_ms

            while start_ms < now_ts_ms:
                params = {
                    "symbol": binance_symbol,
                    "interval": interval,
                    "startTime": start_ms,
                    "limit": limit,
                }
                try:
                    resp = self._session.get(
                        BINANCE_KLINES, params=params, timeout=self._timeout
                    )
                    resp.raise_for_status()
                    klines = resp.json()
                except Exception as exc:
                    log.warning(
                        "crypto.binance_backfill.http_error",
                        coin_id=coin_id,
                        start_ms=start_ms,
                        error=str(exc),
                    )
                    coin_errors += 1
                    # Advance by one full window to avoid getting stuck on a
                    # bad time range and retrying the same segment forever.
                    interval_ms = _interval_to_ms(interval)
                    start_ms += limit * interval_ms
                    time.sleep(0.25)
                    continue

                if not klines:
                    # No more data from Binance for this coin; exit inner loop.
                    break

                for kline in klines:
                    open_time_ms = int(kline[_K_OPEN_TIME])
                    # Convert UTC ms → naive ICT wallclock, hour-truncated
                    dt_ict = (
                        datetime.fromtimestamp(open_time_ms / 1000, tz=timezone.utc)
                        .astimezone(_VN_TZ)
                        .replace(minute=0, second=0, microsecond=0, tzinfo=None)
                    )
                    record = CryptoIntradayPrice(
                        coin_id=coin_id,
                        timestamp=dt_ict,
                        open_price=Decimal(str(kline[_K_OPEN])),
                        high_price=Decimal(str(kline[_K_HIGH])),
                        low_price=Decimal(str(kline[_K_LOW])),
                        price=Decimal(str(kline[_K_CLOSE])),
                        market_cap=Decimal("0"),
                        volume=Decimal(str(kline[_K_VOLUME])),
                    )
                    repo.upsert_crypto_intraday(record)
                    coin_saved += 1

                # Advance startTime to just after the last candle's open time
                last_open_ms = int(klines[-1][_K_OPEN_TIME])
                start_ms = last_open_ms + 1

                # Polite pause between paginated requests (Binance is generous
                # at 1 200 weight/min on the public endpoint, but stay civil)
                time.sleep(0.25)

            total += coin_saved
            log.info(
                "crypto.binance_backfill.coin.done",
                coin_id=coin_id,
                saved=coin_saved,
                errors=coin_errors,
            )

        log.info("crypto.binance_backfill.done", total=total)
        return total


def _interval_to_ms(interval: str) -> int:
    """Convert a Binance interval string to its duration in milliseconds.

    Used as a fallback step when an HTTP error is encountered mid-page so the
    loop can still advance rather than retrying the same time window forever.

    Only common intervals are mapped; unknown strings default to 1 hour.
    """
    _table = {
        "1m": 60_000,
        "3m": 180_000,
        "5m": 300_000,
        "15m": 900_000,
        "30m": 1_800_000,
        "1h": 3_600_000,
        "2h": 7_200_000,
        "4h": 14_400_000,
        "6h": 21_600_000,
        "8h": 28_800_000,
        "12h": 43_200_000,
        "1d": 86_400_000,
        "3d": 259_200_000,
        "1w": 604_800_000,
    }
    return _table.get(interval, 3_600_000)
