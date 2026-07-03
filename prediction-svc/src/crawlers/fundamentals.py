"""Stock fundamentals crawler — yfinance (Yahoo Finance) snapshots.

Fetches the latest fundamental metrics (P/E, EPS, growth, margins, market cap,
…) for every NASDAQ and S&P 500 symbol and upserts one snapshot row per
(market_key, symbol) into `stock_fundamentals`.

Fundamentals are quarterly-cadence data, so a weekly crawl is plenty
(cron `crawler_fundamentals`, Saturday — US markets closed).  GOLD and CRYPTO
have no financial reports and get no rows; consumers must handle absence.

Consumed by the transformer_nn algorithm as static per-symbol context
features.  NOTE: rows are latest-snapshot only (not point-in-time history) —
training joins today's fundamentals onto past windows, which is acceptable
for slow-moving ratios but should not be mistaken for leak-free
point-in-time data.
"""
from __future__ import annotations

import time

from src.crawlers.base import BaseCrawler
from src.crawlers.nasdaq import NASDAQ_SYMBOLS
from src.crawlers.sp500 import SP500_SYMBOLS
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("crawler.fundamentals")

REQUEST_DELAY = 0.5  # 500ms between symbols — same etiquette as price crawlers

# yfinance Ticker.info key → stock_fundamentals column
_INFO_FIELD_MAP: dict[str, str] = {
    "trailingPE": "pe_ratio",
    "forwardPE": "forward_pe",
    "priceToBook": "price_to_book",
    "trailingEps": "eps_ttm",
    "revenueGrowth": "revenue_growth",
    "earningsGrowth": "earnings_growth",
    "profitMargins": "profit_margin",
    "debtToEquity": "debt_to_equity",
    "dividendYield": "dividend_yield",
    "beta": "beta",
    "marketCap": "market_cap",
}


def _extract_fields(info: dict) -> dict:
    """Map a yfinance info dict to stock_fundamentals columns (numeric only).

    Non-numeric / missing / non-finite values become None (stored as NULL).
    ETFs (QQQ/SPY) miss most ratios — that is expected, not an error.
    """
    out: dict = {}
    for info_key, col in _INFO_FIELD_MAP.items():
        val = info.get(info_key)
        try:
            f = float(val)
            out[col] = f if f == f and abs(f) != float("inf") else None
        except (TypeError, ValueError):
            out[col] = None
    return out


class FundamentalsCrawler(BaseCrawler):
    """Weekly fundamentals snapshot crawler for equity markets."""

    def crawl(self) -> int:
        try:
            import yfinance as yf
        except ImportError as exc:
            log.error("fundamentals.no_yfinance", error=str(exc))
            return 0

        targets = [("NASDAQ100", sym) for sym in NASDAQ_SYMBOLS] + [
            ("SP500", sym) for sym in SP500_SYMBOLS
        ]

        saved = 0
        for market_key, symbol in targets:
            try:
                info = yf.Ticker(symbol).info or {}
                fields = _extract_fields(info)
                if all(v is None for v in fields.values()):
                    log.warning("fundamentals.empty", market=market_key, symbol=symbol)
                    continue
                repo.upsert_stock_fundamental(market_key, symbol, fields)
                saved += 1
            except Exception as exc:
                log.warning("fundamentals.fetch_failed", market=market_key,
                            symbol=symbol, error=str(exc))
            time.sleep(REQUEST_DELAY)

        log.info("fundamentals.crawl.done", saved=saved, targets=len(targets))
        return saved

    def crawl_history(self, days: int = 180) -> int:
        """Fundamentals have no per-day history in this design — same as crawl()."""
        return self.crawl()
