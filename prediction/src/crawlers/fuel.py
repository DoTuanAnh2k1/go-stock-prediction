"""Vietnamese fuel price crawler from giaxanghomnay.com."""
from __future__ import annotations

from datetime import datetime
from decimal import Decimal

import requests

from src.crawlers.base import DEFAULT_HEADERS, BaseCrawler
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("crawler.fuel")

FUEL_DAILY_URL = "https://giaxanghomnay.com/api/pvdate/{date}"
FUEL_HISTORY_URL = "https://giaxanghomnay.com/api/chart"

# Map: API field → product_type in DB
FUEL_PRODUCT_MAP = {
    "a": "ron95_iii",
    "b": "e5_ron92",
    "c": "do_005s",
    "d": "kerosene",
}


class FuelCrawler(BaseCrawler):
    def __init__(self, timeout: int = 15) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        today_str = datetime.utcnow().strftime("%Y-%m-%d")
        url = FUEL_DAILY_URL.format(date=today_str)
        log.info("fuel.crawl.start", date=today_str)

        try:
            resp = self._session.get(url, timeout=self._timeout)
            resp.raise_for_status()
            data = resp.json()

            if not data:
                log.info("fuel.crawl.no_update", date=today_str)
                return 0

            saved = self._save_entries(data if isinstance(data, list) else [data])
            log.info("fuel.crawl.done", saved=saved)
            return saved

        except Exception as exc:
            log.warning("fuel.crawl.error", error=str(exc))
            return 0

    def crawl_history(self, days: int = 365) -> int:
        log.info("fuel.history.start")

        try:
            resp = self._session.get(FUEL_HISTORY_URL, timeout=self._timeout)
            resp.raise_for_status()
            data = resp.json()

            if not isinstance(data, list):
                data = [data] if data else []

            saved = self._save_entries(data)
            log.info("fuel.history.done", saved=saved)
            return saved

        except Exception as exc:
            log.warning("fuel.history.error", error=str(exc))
            return 0

    @staticmethod
    def _save_entries(entries: list) -> int:
        saved = 0
        for entry in entries:
            if not isinstance(entry, dict):
                continue

            date_str = entry.get("date", "")
            if not date_str:
                continue

            try:
                trading_date = datetime.strptime(date_str, "%Y-%m-%d").date()
            except ValueError:
                continue

            for field, product_type in FUEL_PRODUCT_MAP.items():
                raw = entry.get(field)
                if raw is None:
                    continue
                try:
                    price = Decimal(str(float(raw)))
                    if price > 0:
                        repo.upsert_fuel_price(
                            product_type=product_type,
                            trading_date=trading_date,
                            price=price,
                        )
                        saved += 1
                except Exception:
                    pass

        return saved
