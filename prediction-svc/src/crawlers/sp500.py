"""S&P 500 stock price crawler using Yahoo Finance API."""
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
from src.database.models import SP500IntradayPrice
from src.telemetry import metrics as _metrics
from src.utils.logger import get_logger

log = get_logger("crawler.sp500")

YAHOO_CHART = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1d&range={range}&events=split"
YAHOO_INTRADAY_URL = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1h&range={range}"
REQUEST_DELAY = 0.5  # 500ms
BACKFILL_DELAY = 0.3  # 300ms — slightly faster for batch backfill

SP500_SYMBOLS = [
    # Original 15
    "SPY", "QQQ",           # Index ETFs
    "JPM", "BAC", "GS",     # Financials
    "JNJ", "UNH", "PFE",    # Healthcare
    "PG", "KO", "WMT",      # Consumer Staples
    "XOM", "CVX",            # Energy
    "V", "MA",               # Payments
    # S&P 500 large-cap additions (~45 more across sectors)
    # Technology
    "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META",
    "ADBE", "CRM", "ACN", "TXN", "INTC", "IBM",
    "QCOM", "ORCL", "HPQ", "NOW",
    # Healthcare / Pharma
    "LLY", "ABBV", "MRK", "TMO", "ABT", "DHR",
    "AMGN", "MDLZ", "ZTS",
    # Consumer Discretionary
    "TSLA", "MCD", "SBUX", "NKE", "LOW", "TGT",
    "HD", "BKNG",
    # Industrials / Conglomerates
    "CAT", "GE", "BA", "HON", "UNP", "RTX",
    "DE", "MMM", "LMT",
    # Energy / Materials
    "SLB", "LIN",
    # Communication / Media
    "DIS", "T", "VZ", "NFLX",
    # Financials (additions)
    "BRK-B", "WFC", "C", "MS", "BLK",
    # Utilities / REIT
    "NEE", "AMT",
    # Consumer Staples (additions)
    "PM", "COST", "AVGO",
]


class SP500Crawler(BaseCrawler):
    def __init__(self, timeout: int = 20) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        log.info("sp500.crawl.start", symbols=len(SP500_SYMBOLS))
        saved = 0
        errors = 0
        new_splits = 0

        for symbol in SP500_SYMBOLS:
            try:
                rows, splits = self._fetch(symbol, range_="2d")
                for row in rows:
                    repo.upsert_sp500_price(**row)
                    saved += 1
                for sp in splits:
                    sid = repo.record_split(
                        "SP500", symbol, sp["split_date"],
                        sp["numerator"], sp["denominator"],
                    )
                    if sid is not None:
                        log.info(
                            "crawl.split.detected",
                            symbol=symbol,
                            ratio=float(sp["numerator"]) / float(sp["denominator"]),
                            split_date=str(sp["split_date"]),
                        )
                        new_splits += 1
            except Exception as exc:
                log.warning("sp500.crawl.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("sp500.crawl.done", saved=saved, errors=errors, new_splits=new_splits)
        _metrics.record_crawl_saved("SP500", saved)

        if new_splits > 0:
            try:
                from src.orchestrator.splits import apply_pending_splits
                apply_pending_splits()
            except Exception as exc:
                log.error("sp500.crawl.split_apply_error", error=str(exc))

        return saved

    def crawl_history(self, days: int = 180) -> int:
        log.info("sp500.history.start", symbols=len(SP500_SYMBOLS))
        saved = 0
        errors = 0
        new_splits = 0

        for symbol in SP500_SYMBOLS:
            try:
                rows, splits = self._fetch(symbol, range_="1y")
                for row in rows:
                    repo.upsert_sp500_price(**row)
                    saved += 1
                log.debug("sp500.history.symbol", symbol=symbol, rows=len(rows))
                for sp in splits:
                    sid = repo.record_split(
                        "SP500", symbol, sp["split_date"],
                        sp["numerator"], sp["denominator"],
                    )
                    if sid is not None:
                        log.info(
                            "crawl.split.detected",
                            symbol=symbol,
                            ratio=float(sp["numerator"]) / float(sp["denominator"]),
                            split_date=str(sp["split_date"]),
                        )
                        new_splits += 1
            except Exception as exc:
                log.warning("sp500.history.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("sp500.history.done", saved=saved, errors=errors, new_splits=new_splits)

        if new_splits > 0:
            try:
                from src.orchestrator.splits import apply_pending_splits
                apply_pending_splits()
            except Exception as exc:
                log.error("sp500.history.split_apply_error", error=str(exc))

        return saved

    def crawl_intraday(self) -> int:
        """Fetch last 2d of hourly bars for each S&P 500 symbol and persist them."""
        log.info("sp500.intraday.start", symbols=len(SP500_SYMBOLS))
        saved = 0

        for symbol in SP500_SYMBOLS:
            try:
                url = YAHOO_INTRADAY_URL.format(symbol=symbol, range="2d")
                records = self._fetch_intraday_bars(symbol, url)
                for record in records:
                    repo.upsert_sp500_intraday(record)
                    saved += 1
            except Exception as exc:
                log.warning("sp500.intraday.error", symbol=symbol, error=str(exc))

            time.sleep(REQUEST_DELAY)

        log.info("sp500.intraday.done", saved=saved)
        return saved

    def backfill_intraday(self, range_: str = "2y") -> int:
        """Backfill up to ~2 years of hourly bars for all S&P 500 symbols.

        Yahoo Finance allows up to ~730 days of 1h bars when range=2y.
        Iterates the full symbol list with a 300ms delay between requests
        to respect rate limits.

        Args:
            range_: Yahoo Finance range string — default "2y" (~730 days of 1h bars).
                    Valid alternatives: "1y", "6mo", "3mo".

        Returns:
            Total number of intraday rows upserted.
        """
        log.info("sp500.backfill_intraday.start", symbols=len(SP500_SYMBOLS), range=range_)
        saved = 0
        errors = 0

        for symbol in SP500_SYMBOLS:
            try:
                url = YAHOO_INTRADAY_URL.format(symbol=symbol, range=range_)
                records = self._fetch_intraday_bars(symbol, url)
                for record in records:
                    repo.upsert_sp500_intraday(record)
                    saved += 1
                log.debug("sp500.backfill_intraday.symbol", symbol=symbol, rows=len(records))
            except Exception as exc:
                log.warning("sp500.backfill_intraday.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(BACKFILL_DELAY)

        log.info("sp500.backfill_intraday.done", saved=saved, errors=errors, range=range_)
        return saved

    # -------------------------------------------------------------------
    # Internal helpers
    # -------------------------------------------------------------------

    def _fetch_intraday_bars(self, symbol: str, url: str) -> list[SP500IntradayPrice]:
        """Fetch and parse Yahoo Finance chart JSON for hourly bars.

        Shared by crawl_intraday() and backfill_intraday() so the parse
        logic lives in exactly one place.
        """
        resp = self._session.get(url, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        results = data.get("chart", {}).get("result", [])
        if not results:
            return []

        result = results[0]
        timestamps = result.get("timestamp", [])
        quote = result.get("indicators", {}).get("quote", [{}])[0]

        opens = quote.get("open", [])
        highs = quote.get("high", [])
        lows = quote.get("low", [])
        closes = quote.get("close", [])
        volumes = quote.get("volume", [])

        records: list[SP500IntradayPrice] = []
        for i, ts in enumerate(timestamps):
            close = closes[i] if i < len(closes) else None
            if close is None:
                continue

            dt = datetime.fromtimestamp(ts, tz=timezone.utc).astimezone(_VN_TZ).replace(
                minute=0, second=0, microsecond=0, tzinfo=None
            )
            records.append(SP500IntradayPrice(
                symbol=symbol,
                timestamp=dt,
                open_price=Decimal(str(opens[i] or 0)) if i < len(opens) else Decimal(0),
                high_price=Decimal(str(highs[i] or 0)) if i < len(highs) else Decimal(0),
                low_price=Decimal(str(lows[i] or 0)) if i < len(lows) else Decimal(0),
                close_price=Decimal(str(close)),
                volume=int(volumes[i] or 0) if i < len(volumes) else 0,
            ))

        # Apply bilateral spike filter before returning.
        if records:
            close_vals = [float(r.close_price) for r in records]
            mask = sanity.batch_outlier_mask(close_vals, "SP500")
            filtered: list[SP500IntradayPrice] = []
            for i, (rec, is_spike) in enumerate(zip(records, mask)):
                if is_spike:
                    log.warning(
                        "crawl.sanity.spike_dropped",
                        symbol=symbol,
                        ts=str(rec.timestamp),
                        price=float(rec.close_price),
                    )
                else:
                    filtered.append(rec)
            return filtered

        return records

    def _fetch(self, symbol: str, range_: str = "2d") -> tuple[list[dict], list[dict]]:
        """Fetch daily prices and split events for *symbol*.

        Returns:
            (rows, splits) where:
            - rows: list of price dicts suitable for ``upsert_sp500_price(**row)``
            - splits: list of dicts with keys split_date (datetime.date),
              numerator (Decimal), denominator (Decimal)
        """
        url = YAHOO_CHART.format(symbol=symbol, range=range_)
        resp = self._session.get(url, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        results = data.get("chart", {}).get("result", [])
        if not results:
            return [], []

        result = results[0]
        timestamps = result.get("timestamp", [])
        quote = result.get("indicators", {}).get("quote", [{}])[0]

        opens = quote.get("open", [])
        highs = quote.get("high", [])
        lows = quote.get("low", [])
        closes = quote.get("close", [])
        volumes = quote.get("volume", [])

        rows = []
        for i, ts in enumerate(timestamps):
            close = closes[i] if i < len(closes) else None
            if not close:
                continue

            trading_date = datetime.fromtimestamp(ts, tz=timezone.utc).astimezone(_VN_TZ).date()
            rows.append(dict(
                symbol=symbol,
                trading_date=trading_date,
                open_price=Decimal(str(opens[i])) if i < len(opens) and opens[i] else Decimal(0),
                high_price=Decimal(str(highs[i])) if i < len(highs) and highs[i] else Decimal(0),
                low_price=Decimal(str(lows[i])) if i < len(lows) and lows[i] else Decimal(0),
                close_price=Decimal(str(close)),
                volume=int(volumes[i]) if i < len(volumes) and volumes[i] else 0,
                currency="USD",
            ))

        # Parse split events — Yahoo returns them as a dict keyed by unix timestamp
        splits: list[dict] = []
        split_events = result.get("events", {}).get("splits", {})
        for _key, ev in split_events.items():
            try:
                ts = ev.get("date") or _key
                split_dt = datetime.fromtimestamp(int(ts), tz=timezone.utc).astimezone(_VN_TZ).date()
                num = Decimal(str(ev.get("numerator", 1)))
                den = Decimal(str(ev.get("denominator", 1)))
                if den > 0:
                    splits.append({"split_date": split_dt, "numerator": num, "denominator": den})
            except Exception as exc:
                log.warning("sp500.split_parse.error", symbol=symbol, event=str(ev), error=str(exc))

        return rows, splits
