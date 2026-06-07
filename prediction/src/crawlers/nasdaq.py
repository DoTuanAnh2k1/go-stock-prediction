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
YAHOO_INTRADAY_URL = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1h&range=2d"
REQUEST_DELAY = 0.5  # 500ms

NASDAQ_SYMBOLS = [
    "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA",
    "META", "TSLA", "AVGO", "COST", "NFLX",
    "AMD", "ADBE", "QCOM", "INTC", "CSCO",
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
                url = YAHOO_INTRADAY_URL.format(symbol=symbol)
                resp = self._session.get(url, timeout=self._timeout)
                resp.raise_for_status()
                data = resp.json()

                results = data.get("chart", {}).get("result", [])
                if not results:
                    continue

                result = results[0]
                timestamps = result.get("timestamp", [])
                quote = result.get("indicators", {}).get("quote", [{}])[0]

                opens = quote.get("open", [])
                highs = quote.get("high", [])
                lows = quote.get("low", [])
                closes = quote.get("close", [])
                volumes = quote.get("volume", [])

                for i, ts in enumerate(timestamps):
                    close = closes[i] if i < len(closes) else None
                    if close is None:
                        continue

                    dt = datetime.fromtimestamp(ts, tz=timezone.utc).astimezone(_VN_TZ).replace(
                        minute=0, second=0, microsecond=0, tzinfo=None
                    )
                    record = NasdaqIntradayPrice(
                        symbol=symbol,
                        timestamp=dt,
                        open_price=Decimal(str(opens[i] or 0)) if i < len(opens) else Decimal(0),
                        high_price=Decimal(str(highs[i] or 0)) if i < len(highs) else Decimal(0),
                        low_price=Decimal(str(lows[i] or 0)) if i < len(lows) else Decimal(0),
                        close_price=Decimal(str(close)),
                        volume=int(volumes[i] or 0) if i < len(volumes) else 0,
                    )
                    repo.upsert_nasdaq_intraday(record)
                    saved += 1

            except Exception as exc:
                log.warning("nasdaq.intraday.error", symbol=symbol, error=str(exc))

            time.sleep(REQUEST_DELAY)

        log.info("nasdaq.intraday.done", saved=saved)
        return saved

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
