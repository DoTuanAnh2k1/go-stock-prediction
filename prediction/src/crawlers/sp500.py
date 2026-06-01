"""S&P 500 stock price crawler using Yahoo Finance API."""
from __future__ import annotations

import time
from datetime import datetime
from decimal import Decimal

import requests

from src.crawlers.base import DEFAULT_HEADERS, BaseCrawler
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("crawler.sp500")

YAHOO_CHART = "https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1d&range={range}"
REQUEST_DELAY = 0.5  # 500ms

SP500_SYMBOLS = [
    "SPY", "QQQ",           # Index ETFs
    "JPM", "BAC", "GS",     # Financials
    "JNJ", "UNH", "PFE",    # Healthcare
    "PG", "KO", "WMT",      # Consumer Staples
    "XOM", "CVX",            # Energy
    "V", "MA",               # Payments
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

        for symbol in SP500_SYMBOLS:
            try:
                rows = self._fetch(symbol, range_="2d")
                for row in rows:
                    repo.upsert_sp500_price(**row)
                    saved += 1
            except Exception as exc:
                log.warning("sp500.crawl.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("sp500.crawl.done", saved=saved, errors=errors)
        return saved

    def crawl_history(self, days: int = 180) -> int:
        log.info("sp500.history.start", symbols=len(SP500_SYMBOLS))
        saved = 0
        errors = 0

        for symbol in SP500_SYMBOLS:
            try:
                rows = self._fetch(symbol, range_="1y")
                for row in rows:
                    repo.upsert_sp500_price(**row)
                    saved += 1
                log.debug("sp500.history.symbol", symbol=symbol, rows=len(rows))
            except Exception as exc:
                log.warning("sp500.history.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("sp500.history.done", saved=saved, errors=errors)
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

            trading_date = datetime.utcfromtimestamp(ts).date()
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
