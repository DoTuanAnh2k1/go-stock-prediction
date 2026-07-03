"""NASDAQ 100 stock price crawler using Yahoo Finance API."""
from __future__ import annotations

import time
from datetime import datetime, timezone
from decimal import Decimal
from zoneinfo import ZoneInfo

_VN_TZ = ZoneInfo("Asia/Ho_Chi_Minh")

import requests

from src.crawlers.base import DEFAULT_HEADERS, BaseCrawler
from src.database import repository as repo
from src.database.models import NasdaqIntradayPrice
from src.utils.logger import get_logger

log = get_logger("crawler.nasdaq")

YAHOO_CHART = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1d&range={range}"
YAHOO_INTRADAY_URL = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1h&range={range}"
REQUEST_DELAY = 0.5  # 500ms
BACKFILL_DELAY = 0.3  # 300ms — slightly faster for batch backfill

NASDAQ_SYMBOLS = [
    # Original 15
    "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA",
    "META", "TSLA", "AVGO", "COST", "NFLX",
    "AMD", "ADBE", "QCOM", "INTC", "CSCO",
    # NASDAQ-100 additions (~70 more validated constituents)
    "PYPL", "AMAT", "MU", "ADI", "LRCX",
    "KLAC", "SNPS", "CDNS", "PANW", "CRWD",
    "MRVL", "ORLY", "MELI", "ABNB", "FTNT",
    "ADSK", "CTAS", "MNST", "KDP", "PCAR",
    "ROP", "NXPI", "AEP", "EXC", "XEL",
    "CPRT", "PAYX", "ODFL", "FAST", "ROST",
    "IDXX", "DXCM", "BKR", "EA", "CSGP",
    "TTD", "TEAM", "DDOG", "ZS", "ANSS",
    "VRSK", "GEHC", "WBD", "CCEP", "BIIB",
    "ILMN", "WBA", "ISRG", "REGN", "GILD",
    "MDLZ", "SBUX", "AZN", "VRTX", "BMRN",
    "SGEN", "INCY", "ALGN", "SIRI", "TTWO",
    "ZM", "OKTA", "SPLK", "WDAY", "VEEV",
    "DOCU", "PTON", "ROKU", "COIN", "APP",
    "ARM", "SMCI", "CEG", "ON", "MCHP",
    "TXN", "ASML", "LULU", "PDD", "BKNG",
]


class NasdaqCrawler(BaseCrawler):
    def __init__(self, timeout: int = 20) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        log.info("nasdaq.crawl.start", symbols=len(NASDAQ_SYMBOLS))
        saved = 0
        errors = 0

        for symbol in NASDAQ_SYMBOLS:
            try:
                rows = self._fetch(symbol, range_="2d")
                for row in rows:
                    repo.upsert_nasdaq_price(**row)
                    saved += 1
            except Exception as exc:
                log.warning("nasdaq.crawl.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("nasdaq.crawl.done", saved=saved, errors=errors)
        return saved

    def crawl_history(self, days: int = 180) -> int:
        log.info("nasdaq.history.start", symbols=len(NASDAQ_SYMBOLS))
        saved = 0
        errors = 0

        for symbol in NASDAQ_SYMBOLS:
            try:
                rows = self._fetch(symbol, range_="6mo")
                for row in rows:
                    repo.upsert_nasdaq_price(**row)
                    saved += 1
                log.debug("nasdaq.history.symbol", symbol=symbol, rows=len(rows))
            except Exception as exc:
                log.warning("nasdaq.history.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("nasdaq.history.done", saved=saved, errors=errors)
        return saved

    def crawl_intraday(self) -> int:
        """Fetch last 2d of hourly bars for each NASDAQ symbol and persist them."""
        log.info("nasdaq.intraday.start", symbols=len(NASDAQ_SYMBOLS))
        saved = 0

        for symbol in NASDAQ_SYMBOLS:
            try:
                url = YAHOO_INTRADAY_URL.format(symbol=symbol, range="2d")
                records = self._fetch_intraday_bars(symbol, url)
                for record in records:
                    repo.upsert_nasdaq_intraday(record)
                    saved += 1
            except Exception as exc:
                log.warning("nasdaq.intraday.error", symbol=symbol, error=str(exc))

            time.sleep(REQUEST_DELAY)

        log.info("nasdaq.intraday.done", saved=saved)
        return saved

    def backfill_intraday(self, range_: str = "2y") -> int:
        """Backfill up to ~2 years of hourly bars for all NASDAQ symbols.

        Yahoo Finance allows up to ~730 days of 1h bars when range=2y.
        Iterates the full symbol list with a 300ms delay between requests
        to respect rate limits.

        Args:
            range_: Yahoo Finance range string — default "2y" (~730 days of 1h bars).
                    Valid alternatives: "1y", "6mo", "3mo".

        Returns:
            Total number of intraday rows upserted.
        """
        log.info("nasdaq.backfill_intraday.start", symbols=len(NASDAQ_SYMBOLS), range=range_)
        saved = 0
        errors = 0

        for symbol in NASDAQ_SYMBOLS:
            try:
                url = YAHOO_INTRADAY_URL.format(symbol=symbol, range=range_)
                records = self._fetch_intraday_bars(symbol, url)
                for record in records:
                    repo.upsert_nasdaq_intraday(record)
                    saved += 1
                log.debug("nasdaq.backfill_intraday.symbol", symbol=symbol, rows=len(records))
            except Exception as exc:
                log.warning("nasdaq.backfill_intraday.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(BACKFILL_DELAY)

        log.info("nasdaq.backfill_intraday.done", saved=saved, errors=errors, range=range_)
        return saved

    # -------------------------------------------------------------------
    # Internal helpers
    # -------------------------------------------------------------------

    def _fetch_intraday_bars(self, symbol: str, url: str) -> list[NasdaqIntradayPrice]:
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

        records: list[NasdaqIntradayPrice] = []
        for i, ts in enumerate(timestamps):
            close = closes[i] if i < len(closes) else None
            if close is None:
                continue

            dt = datetime.fromtimestamp(ts, tz=timezone.utc).astimezone(_VN_TZ).replace(
                minute=0, second=0, microsecond=0, tzinfo=None
            )
            records.append(NasdaqIntradayPrice(
                symbol=symbol,
                timestamp=dt,
                open_price=Decimal(str(opens[i] or 0)) if i < len(opens) else Decimal(0),
                high_price=Decimal(str(highs[i] or 0)) if i < len(highs) else Decimal(0),
                low_price=Decimal(str(lows[i] or 0)) if i < len(lows) else Decimal(0),
                close_price=Decimal(str(close)),
                volume=int(volumes[i] or 0) if i < len(volumes) else 0,
            ))

        return records

    def _fetch(self, symbol: str, range_: str = "2d") -> list[dict]:
        url = YAHOO_CHART.format(symbol=symbol, range=range_)
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

        return rows
