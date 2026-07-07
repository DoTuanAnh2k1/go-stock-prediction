"""Gold price crawler — XAU/USD, XAU/VND, BTMC, BTMH, vang.today, Phú Quý."""
from __future__ import annotations

import time
from datetime import datetime, timezone
from decimal import Decimal
from zoneinfo import ZoneInfo

_VN_TZ = ZoneInfo("Asia/Ho_Chi_Minh")

import requests
from bs4 import BeautifulSoup

from src.crawlers import sanity
from src.crawlers.base import DEFAULT_HEADERS, USER_AGENT, BaseCrawler
from src.database import repository as repo
from src.database.models import GoldIntradayPrice
from src.utils.logger import get_logger
from src.utils.number_parser import safe_parse_vnd

log = get_logger("crawler.gold")

YAHOO_GOLD_DAILY = "https://query1.finance.yahoo.com/v8/finance/chart/GC%3DF?interval=1d&range=1d"
YAHOO_GOLD_HISTORY = "https://query1.finance.yahoo.com/v8/finance/chart/GC%3DF?interval=1d&range=6mo"
YAHOO_GOLD_INTRADAY = "https://query1.finance.yahoo.com/v8/finance/chart/GC%3DF?interval=1h&range={range}"
BACKFILL_DELAY = 0.3  # 300ms
USD_VND_RATE_URL = "https://open.er-api.com/v6/latest/USD"
BTMC_API_URL = "http://api.btmc.vn/api/BTMCAPI/getpricebtmc?key=3kd8ub1llcg9t45hnoh8hmn7t5kc2v"
BTMH_URL = "https://giavang.org/trong-nuoc/bao-tin-manh-hai/"
VANG_TODAY_URL = "https://www.vang.today/api/prices"
VANG_TODAY_HISTORY_URL = "https://www.vang.today/api/prices?type={type_code}&days=30"
PHU_QUY_URL = "https://gold.phuquy.com.vn"

VANG_TODAY_TYPE_MAP = [
    ("SJL1L10", "SJC", "sjc"),
    ("SJ9999", "SJC", "nhan_tron"),
    ("DOHNL", "DOJI", "sjc"),
    ("PQHNVM", "PNJ", "nhan_tron"),
    ("BTSJC", "BTMC", "sjc"),
    ("BT9999NTT", "BTMC", "nhan_tron"),
]

BTMH_PRODUCT_MAP = [
    ("sjc", "sjc"),
    ("trang sức 24k", "trang_suc_24k"),
    ("nhẫn", "nhan_tron"),
]

PHU_QUY_PRODUCT_MAP = [
    ("sjc", "sjc"),
    ("nhẫn tròn", "nhan_tron"),
]


class GoldCrawler(BaseCrawler):
    def __init__(self, timeout: int = 15) -> None:
        self._session = requests.Session()
        self._session.headers.update(DEFAULT_HEADERS)
        self._timeout = timeout

    def crawl(self) -> int:
        log.info("gold.crawl.start")
        saved = 0
        errors = 0

        # XAU/USD
        try:
            xau_usd, xau_vnd = self._crawl_xau()
            if xau_usd:
                repo.upsert_gold_price(**xau_usd)
                saved += 1
            if xau_vnd:
                repo.upsert_gold_price(**xau_vnd)
                saved += 1
        except Exception as exc:
            log.warning("gold.xau.error", error=str(exc))
            errors += 1

        # BTMC
        try:
            prices = self._crawl_btmc()
            for p in prices:
                repo.upsert_gold_price(**p)
                saved += 1
        except Exception as exc:
            log.warning("gold.btmc.error", error=str(exc))
            errors += 1

        # BTMH
        try:
            prices = self._crawl_btmh()
            for p in prices:
                repo.upsert_gold_price(**p)
                saved += 1
        except Exception as exc:
            log.warning("gold.btmh.error", error=str(exc))
            errors += 1

        # vang.today
        try:
            prices = self._crawl_vang_today()
            for p in prices:
                repo.upsert_gold_price(**p)
                saved += 1
        except Exception as exc:
            log.warning("gold.vangtoday.error", error=str(exc))
            errors += 1

        # Phú Quý
        try:
            prices = self._crawl_phu_quy()
            for p in prices:
                repo.upsert_gold_price(**p)
                saved += 1
        except Exception as exc:
            log.warning("gold.phuquy.error", error=str(exc))
            errors += 1

        log.info("gold.crawl.done", saved=saved, errors=errors)
        return saved

    def crawl_history(self, days: int = 180) -> int:
        """Import 6-month XAU/USD historical data + 30-day vang.today history."""
        log.info("gold.history.start")
        saved = 0

        # XAU/USD 6-month history from Yahoo Finance
        try:
            saved += self._import_xau_history()
        except Exception as exc:
            log.warning("gold.history.xau.error", error=str(exc))

        # vang.today 30-day history
        try:
            saved += self._import_vang_today_history()
        except Exception as exc:
            log.warning("gold.history.vangtoday.error", error=str(exc))

        log.info("gold.history.done", saved=saved)
        return saved

    def crawl_intraday(self) -> int:
        """Fetch last 2d of hourly XAU/USD bars from Yahoo Finance and persist them."""
        log.info("gold.intraday.start")
        saved = 0

        try:
            # Fetch USD/VND rate once for VND conversion
            try:
                vnd_rate = self._fetch_usd_vnd_rate()
            except Exception as exc:
                log.warning("gold.intraday.vnd_rate.error", error=str(exc))
                vnd_rate = None

            saved = self._fetch_and_upsert_xau_intraday(range_="2d", vnd_rate=vnd_rate)
        except Exception as exc:
            log.warning("gold.intraday.error", error=str(exc))

        log.info("gold.intraday.done", saved=saved)
        return saved

    def backfill_intraday(self, range_: str = "2y") -> int:
        """Backfill up to ~2 years of hourly XAU/USD bars from Yahoo Finance (GC=F only).

        VN sources (SJC, BTMC, Phu Quy, etc.) do not expose intraday history, so
        only the Yahoo Finance GC=F feed is used.  A single USD/VND rate is fetched
        once and applied to all bars for the XAU_VND series.

        Args:
            range_: Yahoo Finance range string — default "2y" (~730 days of 1h bars).
                    Valid alternatives: "1y", "6mo", "3mo".

        Returns:
            Total number of intraday rows upserted (USD + VND bars counted separately).
        """
        log.info("gold.backfill_intraday.start", range=range_)

        try:
            vnd_rate = self._fetch_usd_vnd_rate()
        except Exception as exc:
            log.warning("gold.backfill_intraday.vnd_rate.error", error=str(exc))
            vnd_rate = None

        try:
            saved = self._fetch_and_upsert_xau_intraday(range_=range_, vnd_rate=vnd_rate)
        except Exception as exc:
            log.warning("gold.backfill_intraday.error", error=str(exc))
            saved = 0

        log.info("gold.backfill_intraday.done", saved=saved, range=range_)
        return saved

    # -------------------------------------------------------------------
    # XAU intraday shared helper
    # -------------------------------------------------------------------

    def _fetch_and_upsert_xau_intraday(
        self, range_: str, vnd_rate: Decimal | None
    ) -> int:
        """Fetch Yahoo Finance hourly bars for GC=F and upsert XAU + XAU_VND rows.

        Shared by crawl_intraday() and backfill_intraday() so the parse/upsert
        logic lives in exactly one place.

        Returns the total number of rows upserted (USD + VND bars counted separately).
        """
        url = YAHOO_GOLD_INTRADAY.format(range=range_)
        resp = self._session.get(url, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        results = data.get("chart", {}).get("result", [])
        if not results:
            log.warning("gold.intraday.empty", range=range_)
            return 0

        result = results[0]
        timestamps = result.get("timestamp", [])
        quote = result.get("indicators", {}).get("quote", [{}])[0]

        opens = quote.get("open", [])
        highs = quote.get("high", [])
        lows = quote.get("low", [])
        closes = quote.get("close", [])

        # First pass: collect valid bars (non-None close).
        bars: list[tuple] = []  # (ts, close, open, high, low)
        for i, ts in enumerate(timestamps):
            close = closes[i] if i < len(closes) else None
            if close is None:
                continue
            bars.append((
                ts,
                close,
                opens[i] if i < len(opens) else None,
                highs[i] if i < len(highs) else None,
                lows[i] if i < len(lows) else None,
            ))

        # Apply bilateral spike filter on the close-price sequence.
        if bars:
            close_vals = [float(b[1]) for b in bars]
            spike_mask = sanity.batch_outlier_mask(close_vals, "GOLD")
        else:
            spike_mask = []

        # Second pass: build records and upsert, skipping flagged spikes.
        saved = 0
        for j, (ts, close, open_raw, high_raw, low_raw) in enumerate(bars):
            if spike_mask and spike_mask[j]:
                dt_spike = datetime.fromtimestamp(ts, tz=timezone.utc).astimezone(_VN_TZ).replace(
                    minute=0, second=0, microsecond=0, tzinfo=None
                )
                log.warning(
                    "crawl.sanity.spike_dropped",
                    symbol="XAU",
                    ts=str(dt_spike),
                    price=float(close),
                )
                continue

            dt = datetime.fromtimestamp(ts, tz=timezone.utc).astimezone(_VN_TZ).replace(
                minute=0, second=0, microsecond=0, tzinfo=None
            )
            open_price = Decimal(str(open_raw or 0)) if open_raw is not None else Decimal(0)
            high_price = Decimal(str(high_raw or 0)) if high_raw is not None else Decimal(0)
            low_price = Decimal(str(low_raw or 0)) if low_raw is not None else Decimal(0)
            close_price = Decimal(str(close))

            # Save USD bar using buy/sell = close (spot); attach OHLC from
            # Yahoo response — open/high/low may be 0 or None for some bars.
            _open = open_price if (open_price and open_price > 0) else None
            _high = high_price if (high_price and high_price > 0) else None
            _low = low_price if (low_price and low_price > 0) else None

            record_usd = GoldIntradayPrice(
                source="XAU",
                product_type="spot",
                timestamp=dt,
                open_price=_open,
                high_price=_high,
                low_price=_low,
                buy_price=close_price,
                sell_price=close_price,
                currency="USD",
            )
            repo.upsert_gold_intraday(record_usd)
            saved += 1

            # Also save VND converted bar — scale OHLC by USD/VND rate
            if vnd_rate:
                vnd_price = (close_price * vnd_rate).quantize(Decimal("1"))
                vnd_open = (_open * vnd_rate).quantize(Decimal("1")) if _open else None
                vnd_high = (_high * vnd_rate).quantize(Decimal("1")) if _high else None
                vnd_low = (_low * vnd_rate).quantize(Decimal("1")) if _low else None
                record_vnd = GoldIntradayPrice(
                    source="XAU_VND",
                    product_type="spot",
                    timestamp=dt,
                    open_price=vnd_open,
                    high_price=vnd_high,
                    low_price=vnd_low,
                    buy_price=vnd_price,
                    sell_price=vnd_price,
                    currency="VND",
                )
                repo.upsert_gold_intraday(record_vnd)
                saved += 1

        return saved

    # -------------------------------------------------------------------
    # XAU
    # -------------------------------------------------------------------

    def _crawl_xau(self) -> tuple[dict | None, dict | None]:
        resp = self._session.get(YAHOO_GOLD_DAILY, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        results = data.get("chart", {}).get("result", [])
        if not results:
            raise ValueError("Yahoo Finance returned empty result")

        result = results[0]
        price = float(result["meta"]["regularMarketPrice"])
        if price == 0:
            raise ValueError("Yahoo Finance returned zero price")

        trading_date = datetime.now(tz=_VN_TZ).replace(hour=0, minute=0, second=0, microsecond=0, tzinfo=None)

        # Extract OHLC from the quote block when available (Yahoo v8 chart returns
        # indicators.quote[0] with open/high/low/close arrays for the requested
        # range — even for range=1d there is usually one entry).
        open_price: Decimal | None = None
        high_price: Decimal | None = None
        low_price: Decimal | None = None
        try:
            quote = result.get("indicators", {}).get("quote", [{}])[0]
            opens = quote.get("open", [])
            highs = quote.get("high", [])
            lows = quote.get("low", [])
            if opens and opens[0] is not None:
                open_price = Decimal(str(opens[0]))
            if highs and highs[0] is not None:
                high_price = Decimal(str(highs[0]))
            if lows and lows[0] is not None:
                low_price = Decimal(str(lows[0]))
        except Exception as exc:
            log.warning("gold.xau.ohlc_extract.error", error=str(exc))

        close = Decimal(str(price))
        xau_usd = dict(
            source="XAU", product_type="spot", trading_date=trading_date,
            buy_price=close, sell_price=close, currency="USD",
            open_price=open_price, high_price=high_price, low_price=low_price,
        )

        # XAU/VND — scale OHLC by USD/VND rate
        xau_vnd = None
        try:
            vnd_rate = self._fetch_usd_vnd_rate()
            vnd_price = close * vnd_rate
            vnd_open = (open_price * vnd_rate).quantize(Decimal("1")) if open_price else None
            vnd_high = (high_price * vnd_rate).quantize(Decimal("1")) if high_price else None
            vnd_low = (low_price * vnd_rate).quantize(Decimal("1")) if low_price else None
            xau_vnd = dict(
                source="XAU_VND", product_type="spot", trading_date=trading_date,
                buy_price=vnd_price.quantize(Decimal("1")),
                sell_price=vnd_price.quantize(Decimal("1")),
                currency="VND",
                open_price=vnd_open, high_price=vnd_high, low_price=vnd_low,
            )
        except Exception as exc:
            log.warning("gold.xau_vnd.error", error=str(exc))

        return xau_usd, xau_vnd

    def _fetch_usd_vnd_rate(self) -> Decimal:
        resp = self._session.get(USD_VND_RATE_URL, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()
        vnd = data.get("rates", {}).get("VND")
        if not vnd:
            raise ValueError("VND rate not found")
        return Decimal(str(vnd))

    def _import_xau_history(self) -> int:
        resp = self._session.get(YAHOO_GOLD_HISTORY, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        results = data.get("chart", {}).get("result", [])
        if not results:
            return 0

        result = results[0]
        timestamps = result["timestamp"]
        quote = result["indicators"]["quote"][0]
        opens = quote.get("open", [])
        highs = quote.get("high", [])
        lows = quote.get("low", [])
        closes = quote.get("close", [])

        # Fetch USD/VND rate once for all conversions
        try:
            vnd_rate = self._fetch_usd_vnd_rate()
        except Exception:
            vnd_rate = None

        saved = 0
        for i, (ts, close) in enumerate(zip(timestamps, closes)):
            if not close or close == 0:
                continue
            trading_date = (
                datetime.fromtimestamp(ts, tz=timezone.utc)
                .astimezone(_VN_TZ)
                .replace(hour=0, minute=0, second=0, microsecond=0, tzinfo=None)
            )
            price = Decimal(str(close))

            # Extract per-bar OHLC
            open_price: Decimal | None = None
            high_price: Decimal | None = None
            low_price: Decimal | None = None
            try:
                if i < len(opens) and opens[i] is not None:
                    open_price = Decimal(str(opens[i]))
                if i < len(highs) and highs[i] is not None:
                    high_price = Decimal(str(highs[i]))
                if i < len(lows) and lows[i] is not None:
                    low_price = Decimal(str(lows[i]))
            except Exception:
                pass  # non-fatal; leave as None

            repo.upsert_gold_price(
                source="XAU", product_type="spot", trading_date=trading_date,
                buy_price=price, sell_price=price, currency="USD",
                open_price=open_price, high_price=high_price, low_price=low_price,
            )
            saved += 1

            if vnd_rate:
                vnd_price = (price * vnd_rate).quantize(Decimal("1"))
                vnd_open = (open_price * vnd_rate).quantize(Decimal("1")) if open_price else None
                vnd_high = (high_price * vnd_rate).quantize(Decimal("1")) if high_price else None
                vnd_low = (low_price * vnd_rate).quantize(Decimal("1")) if low_price else None
                repo.upsert_gold_price(
                    source="XAU_VND", product_type="spot", trading_date=trading_date,
                    buy_price=vnd_price, sell_price=vnd_price, currency="VND",
                    open_price=vnd_open, high_price=vnd_high, low_price=vnd_low,
                )
                saved += 1

        return saved

    # -------------------------------------------------------------------
    # BTMC
    # -------------------------------------------------------------------

    def _crawl_btmc(self) -> list[dict]:
        resp = self._session.get(BTMC_API_URL, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        items = data.get("DataList", {}).get("Data", [])
        ten = Decimal("10")
        today = datetime.now(tz=_VN_TZ).replace(hour=0, minute=0, second=0, microsecond=0, tzinfo=None)

        seen: set[str] = set()
        prices = []

        for item in items:
            row = item.get("@row", "").strip()
            if not row:
                continue

            name = item.get(f"@n_{row}", "").strip()
            name_lower = name.lower()

            product_type = None
            if "sjc" in name_lower:
                product_type = "sjc"
            elif "NHẪN TRÒN" in name:
                product_type = "nhan_tron"
            elif "VRTL" in name:
                product_type = "vrtl"
            elif "TRANG SỨC" in name:
                product_type = "trang_suc"

            if product_type is None or product_type in seen:
                continue

            buy_raw = item.get(f"@pb_{row}", "").strip()
            sell_raw = item.get(f"@ps_{row}", "").strip()
            date_raw = item.get(f"@d_{row}", "").strip()

            buy = safe_parse_vnd(buy_raw)
            sell = safe_parse_vnd(sell_raw)

            if buy == 0:
                continue

            # Per chỉ → per lượng
            buy = buy * ten
            sell = sell * ten

            trading_date = today
            if date_raw:
                try:
                    trading_date = datetime.strptime(date_raw, "%d/%m/%Y %H:%M").replace(
                        hour=0, minute=0, second=0, microsecond=0
                    )
                except ValueError:
                    pass

            seen.add(product_type)
            prices.append(dict(
                source="BTMC", product_type=product_type, trading_date=trading_date,
                buy_price=buy, sell_price=sell, currency="VND",
            ))

        return prices

    # -------------------------------------------------------------------
    # BTMH (giavang.org)
    # -------------------------------------------------------------------

    def _crawl_btmh(self) -> list[dict]:
        resp = self._session.get(
            BTMH_URL, timeout=self._timeout,
            headers={"User-Agent": USER_AGENT},
        )
        resp.raise_for_status()

        soup = BeautifulSoup(resp.text, "lxml")
        today = datetime.now(tz=_VN_TZ).replace(hour=0, minute=0, second=0, microsecond=0, tzinfo=None)
        thousand = Decimal("1000")
        prices = []

        for tr in soup.find_all("tr"):
            th = tr.find("th")
            if not th:
                continue
            name = th.get_text(strip=True)
            name_lower = name.lower()

            product_type = None
            for kw, pt in BTMH_PRODUCT_MAP:
                if kw in name_lower:
                    product_type = pt
                    break

            if not product_type:
                continue

            tds = tr.find_all("td")
            if len(tds) < 2:
                continue

            buy = safe_parse_vnd(tds[0].get_text(strip=True))
            sell = safe_parse_vnd(tds[1].get_text(strip=True))

            if buy == 0:
                continue

            prices.append(dict(
                source="BTMH", product_type=product_type, trading_date=today,
                buy_price=buy * thousand, sell_price=sell * thousand, currency="VND",
            ))

        return prices

    # -------------------------------------------------------------------
    # vang.today
    # -------------------------------------------------------------------

    def _crawl_vang_today(self) -> list[dict]:
        resp = self._session.get(VANG_TODAY_URL, timeout=self._timeout)
        resp.raise_for_status()
        data = resp.json()

        if not data.get("success"):
            raise ValueError("vang.today returned success=false")

        lookup = {tc: (src, pt) for tc, src, pt in VANG_TODAY_TYPE_MAP}
        today = datetime.now(tz=_VN_TZ).replace(hour=0, minute=0, second=0, microsecond=0, tzinfo=None)
        prices = []

        for item in data.get("data", []):
            tc = item.get("type_code", "")
            mapping = lookup.get(tc)
            if not mapping:
                continue

            buy = float(item.get("buy", 0) or 0)
            sell = float(item.get("sell", 0) or 0)
            if buy == 0:
                continue

            src, pt = mapping
            prices.append(dict(
                source=src, product_type=pt, trading_date=today,
                buy_price=Decimal(str(buy)), sell_price=Decimal(str(sell)), currency="VND",
            ))

        return prices

    def _import_vang_today_history(self) -> int:
        saved = 0
        for type_code, source, product_type in VANG_TODAY_TYPE_MAP:
            url = VANG_TODAY_HISTORY_URL.format(type_code=type_code)
            try:
                resp = self._session.get(url, timeout=self._timeout)
                resp.raise_for_status()
                data = resp.json()

                if not data.get("success"):
                    continue

                for day in data.get("history", []):
                    product = day.get("prices", {}).get(type_code, {})
                    if not product or float(product.get("buy", 0) or 0) == 0:
                        continue
                    try:
                        trading_date = datetime.strptime(day["date"], "%Y-%m-%d")
                    except (ValueError, KeyError):
                        continue

                    repo.upsert_gold_price(
                        source=source, product_type=product_type, trading_date=trading_date,
                        buy_price=Decimal(str(product["buy"])),
                        sell_price=Decimal(str(product.get("sell", product["buy"]))),
                        currency="VND",
                    )
                    saved += 1
            except Exception as exc:
                log.warning("gold.history.vangtoday.type.error", type_code=type_code, error=str(exc))

            time.sleep(0.5)

        return saved

    # -------------------------------------------------------------------
    # Phú Quý
    # -------------------------------------------------------------------

    def _crawl_phu_quy(self) -> list[dict]:
        resp = self._session.get(PHU_QUY_URL, timeout=self._timeout, headers={"User-Agent": USER_AGENT})
        resp.raise_for_status()

        soup = BeautifulSoup(resp.text, "lxml")
        today = datetime.now(tz=_VN_TZ).replace(hour=0, minute=0, second=0, microsecond=0, tzinfo=None)
        ten = Decimal("10")
        seen: set[str] = set()
        prices = []

        for tr in soup.find_all("tr"):
            tds = tr.find_all("td")
            if len(tds) < 3:
                continue

            name = tds[0].get_text(strip=True)
            name_lower = name.lower()

            product_type = None
            for kw, pt in PHU_QUY_PRODUCT_MAP:
                if kw in name_lower:
                    product_type = pt
                    break

            if not product_type or product_type in seen:
                continue

            buy = safe_parse_vnd(tds[1].get_text(strip=True))
            sell = safe_parse_vnd(tds[2].get_text(strip=True))

            if buy == 0:
                continue

            seen.add(product_type)
            prices.append(dict(
                source="PHUQUY", product_type=product_type, trading_date=today,
                buy_price=buy * ten, sell_price=sell * ten, currency="VND",
            ))

        return prices
