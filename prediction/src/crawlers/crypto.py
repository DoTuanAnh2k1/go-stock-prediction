"""CoinGecko cryptocurrency price crawler (BTC, ETH)."""
from __future__ import annotations

import time
from datetime import date, datetime
from decimal import Decimal
from typing import Optional

import requests

from src.crawlers.base import BaseCrawler, DEFAULT_HEADERS
from src.database import repository as repo
from src.utils.logger import get_logger

log = get_logger("crawler.crypto")

COINGECKO_SIMPLE_PRICE = (
    "https://api.coingecko.com/api/v3/simple/price"
    "?ids=bitcoin,ethereum&vs_currencies=usd"
    "&include_market_cap=true&include_24hr_vol=true"
)
COINGECKO_HISTORY = (
    "https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
    "?vs_currency=usd&days=180&interval=daily"
)
REQUEST_DELAY = 3  # 3s between coins for free tier rate limit

COINS = [
    ("bitcoin", "BTC"),
    ("ethereum", "ETH"),
]


class CryptoCrawler(BaseCrawler):
    def __init__(self, timeout: int = 30) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        log.info("crypto.crawl.start")
        saved = 0
        errors = 0

        try:
            resp = self._session.get(COINGECKO_SIMPLE_PRICE, timeout=self._timeout)
            resp.raise_for_status()
            data = resp.json()

            today = datetime.utcnow().replace(hour=0, minute=0, second=0, microsecond=0).date()

            for coin_id, symbol in COINS:
                coin_data = data.get(coin_id, {})
                if not coin_data:
                    continue

                price = float(coin_data.get("usd", 0) or 0)
                if price == 0:
                    continue

                market_cap = float(coin_data.get("usd_market_cap", 0) or 0)
                vol_24h = float(coin_data.get("usd_24h_vol", 0) or 0)

                repo.upsert_crypto_price(
                    coin_id=coin_id,
                    symbol=symbol,
                    trading_date=today,
                    close_price=Decimal(str(price)),
                    market_cap=Decimal(str(market_cap)),
                    volume_24h=Decimal(str(vol_24h)),
                    currency="USD",
                )
                saved += 1

        except Exception as exc:
            log.warning("crypto.crawl.error", error=str(exc))
            errors += 1

        log.info("crypto.crawl.done", saved=saved, errors=errors)
        return saved

    def crawl_history(self, days: int = 180) -> int:
        log.info("crypto.history.start")
        saved = 0
        errors = 0

        for coin_id, symbol in COINS:
            try:
                url = COINGECKO_HISTORY.format(coin_id=coin_id)
                resp = self._session.get(url, timeout=self._timeout)
                resp.raise_for_status()
                data = resp.json()

                prices_raw = data.get("prices", [])
                market_caps = {entry[0]: entry[1] for entry in data.get("market_caps", [])}
                volumes = {entry[0]: entry[1] for entry in data.get("total_volumes", [])}

                for ts_ms, price in prices_raw:
                    ts_sec = ts_ms / 1000
                    trading_date = datetime.utcfromtimestamp(ts_sec).date()

                    repo.upsert_crypto_price(
                        coin_id=coin_id,
                        symbol=symbol,
                        trading_date=trading_date,
                        close_price=Decimal(str(price)),
                        market_cap=Decimal(str(market_caps.get(ts_ms, 0) or 0)),
                        volume_24h=Decimal(str(volumes.get(ts_ms, 0) or 0)),
                        currency="USD",
                    )
                    saved += 1

                log.debug("crypto.history.coin", coin_id=coin_id, rows=len(prices_raw))

            except Exception as exc:
                log.warning("crypto.history.error", coin_id=coin_id, error=str(exc))
                errors += 1

            time.sleep(REQUEST_DELAY)

        log.info("crypto.history.done", saved=saved, errors=errors)
        return saved
