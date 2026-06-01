"""VN30 stock price crawler using VNDirect public API."""
from __future__ import annotations

import time
from datetime import datetime
from decimal import Decimal

import requests

from src.crawlers.base import DEFAULT_HEADERS, BaseCrawler
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("crawler.vn30")

VNDIRECT_BASE = "https://api-finfo.vndirect.com.vn/v4/stock_prices"
REQUEST_DELAY = 0.3  # 300ms

VN30_SYMBOLS = [
    "VIC", "VHM", "VNM", "VCB", "BID", "CTG", "TCB", "MBB", "HPG", "VPB",
    "GAS", "MSN", "SAB", "MWG", "FPT", "REE", "PLX", "HDB", "TPB", "STB",
    "PNJ", "VJC", "ACB", "VIB", "EIB", "BCM", "SSI", "SHB", "LPB", "BVH",
]


class VN30Crawler(BaseCrawler):
    def __init__(self, timeout: int = 15) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        """Fetch latest prices for all VN30 stocks."""
        log.info("vn30.crawl.start", symbols=len(VN30_SYMBOLS))
        saved = 0
        errors = 0

        for symbol in VN30_SYMBOLS:
            try:
                rows = self._fetch(symbol, size=1)
                for row in rows:
                    self._save(row)
                    saved += 1
            except Exception as exc:
                log.warning("vn30.crawl.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("vn30.crawl.done", saved=saved, errors=errors)
        return saved

    def crawl_history(self, days: int = 365) -> int:
        """Fetch historical prices for all VN30 stocks."""
        log.info("vn30.history.start", symbols=len(VN30_SYMBOLS), days=days)
        saved = 0
        errors = 0

        for symbol in VN30_SYMBOLS:
            try:
                rows = self._fetch(symbol, size=days)
                for row in rows:
                    self._save(row)
                    saved += 1
                log.debug("vn30.history.symbol", symbol=symbol, rows=len(rows))
            except Exception as exc:
                log.warning("vn30.history.error", symbol=symbol, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("vn30.history.done", saved=saved, errors=errors)
        return saved

    def crawl_single(self, symbol: str) -> int:
        """Fetch latest price for a single symbol."""
        rows = self._fetch(symbol, size=1)
        for row in rows:
            self._save(row)
        return len(rows)

    def _fetch(self, symbol: str, size: int = 1) -> list[dict]:
        url = f"{VNDIRECT_BASE}?sort=date&q=code:{symbol}&size={size}"
        resp = self._session.get(url, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", [])

    @staticmethod
    def _save(row: dict) -> None:
        symbol = row.get("code", "")
        date_str = row.get("date", "")
        try:
            trade_date = datetime.strptime(date_str, "%Y-%m-%d")
        except ValueError:
            trade_date = datetime.utcnow().replace(hour=0, minute=0, second=0, microsecond=0)

        nm_vol = float(row.get("nmVolume", 0) or 0)
        pt_vol = float(row.get("ptVolume", 0) or 0)
        nm_val = float(row.get("nmValue", 0) or 0)
        pt_val = float(row.get("ptValue", 0) or 0)

        repo.upsert_stock_price(
            symbol=symbol,
            trading_date=trade_date,
            open_price=Decimal(str(row.get("open", 0) or 0)),
            high_price=Decimal(str(row.get("high", 0) or 0)),
            low_price=Decimal(str(row.get("low", 0) or 0)),
            close_price=Decimal(str(row.get("close", 0) or 0)),
            volume=int(nm_vol + pt_vol),
            value=Decimal(str(nm_val + pt_val)),
            change=Decimal(str(row.get("change", 0) or 0)),
            change_percent=Decimal(str(row.get("pctChange", 0) or 0)),
        )
