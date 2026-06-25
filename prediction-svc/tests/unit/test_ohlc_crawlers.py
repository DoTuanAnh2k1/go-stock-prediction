"""Unit tests for OHLC extraction logic in the crypto and gold crawlers.

These tests are fully offline — no network, no DB, no Docker.
They test:
  - CoinGecko /ohlc response parsing (_fetch_ohlc, _aggregate_ohlc_to_daily)
  - Gold Yahoo Finance OHLC extraction (_crawl_xau, _import_xau_history, crawl_intraday)
  - Repository upsert signature compatibility (OHLC kwargs are optional)
"""
from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal
from unittest.mock import MagicMock, patch
from zoneinfo import ZoneInfo

import pytest

# ---------------------------------------------------------------------------
# Crypto helpers
# ---------------------------------------------------------------------------

from src.crawlers.crypto import _aggregate_ohlc_to_daily, _fetch_ohlc

_VN_TZ = ZoneInfo("Asia/Ho_Chi_Minh")


class TestAggregateOhlcToDaily:
    """_aggregate_ohlc_to_daily groups sub-daily OHLC candles into daily bars."""

    def _ts_ms(self, dt_str: str) -> int:
        """Parse 'YYYY-MM-DD HH:MM' in ICT → UTC ms."""
        dt = datetime.strptime(dt_str, "%Y-%m-%d %H:%M").replace(
            tzinfo=_VN_TZ
        )
        return int(dt.timestamp() * 1000)

    def test_single_candle_passthrough(self):
        ts = self._ts_ms("2024-06-15 10:00")
        candles = [[ts, 65000.0, 65500.0, 64800.0, 65200.0]]
        result = _aggregate_ohlc_to_daily(candles)
        assert "2024-06-15" in result
        o, h, l, c = result["2024-06-15"]
        assert o == Decimal("65000.0")
        assert h == Decimal("65500.0")
        assert l == Decimal("64800.0")
        assert c == Decimal("65200.0")

    def test_multiple_candles_same_day_aggregated(self):
        """Two 30-min candles on the same day → open=first, high=max, low=min, close=last."""
        ts1 = self._ts_ms("2024-06-15 10:00")
        ts2 = self._ts_ms("2024-06-15 10:30")
        candles = [
            [ts1, 65000.0, 65500.0, 64800.0, 65200.0],  # earlier
            [ts2, 65200.0, 66000.0, 65100.0, 65900.0],  # later
        ]
        result = _aggregate_ohlc_to_daily(candles)
        assert len(result) == 1
        o, h, l, c = result["2024-06-15"]
        assert o == Decimal("65000.0")   # open of first candle
        assert h == Decimal("66000.0")   # max high
        assert l == Decimal("64800.0")   # min low
        assert c == Decimal("65900.0")   # close of last candle

    def test_candles_split_across_two_days(self):
        ts_day1 = self._ts_ms("2024-06-14 23:30")
        ts_day2 = self._ts_ms("2024-06-15 00:00")
        candles = [
            [ts_day1, 64000.0, 64200.0, 63900.0, 64100.0],
            [ts_day2, 65000.0, 65500.0, 64800.0, 65200.0],
        ]
        result = _aggregate_ohlc_to_daily(candles)
        assert "2024-06-14" in result
        assert "2024-06-15" in result

    def test_empty_input_returns_empty_dict(self):
        result = _aggregate_ohlc_to_daily([])
        assert result == {}

    def test_returns_decimal_types(self):
        ts = self._ts_ms("2024-06-15 12:00")
        result = _aggregate_ohlc_to_daily([[ts, 100.0, 110.0, 90.0, 105.0]])
        o, h, l, c = list(result.values())[0]
        assert isinstance(o, Decimal)
        assert isinstance(h, Decimal)
        assert isinstance(l, Decimal)
        assert isinstance(c, Decimal)


class TestFetchOhlcGracefulFail:
    """_fetch_ohlc should return [] on network / parse errors without raising."""

    def test_returns_empty_on_http_error(self):
        mock_session = MagicMock()
        mock_session.get.return_value.raise_for_status.side_effect = Exception("HTTP 429")
        result = _fetch_ohlc(mock_session, "bitcoin", days=1, timeout=5)
        assert result == []

    def test_returns_empty_on_non_list_response(self):
        mock_session = MagicMock()
        mock_session.get.return_value.raise_for_status = MagicMock()
        mock_session.get.return_value.json.return_value = {"error": "unexpected"}
        result = _fetch_ohlc(mock_session, "bitcoin", days=1, timeout=5)
        assert result == []

    def test_returns_list_on_valid_response(self):
        ts = 1718428800000
        payload = [[ts, 65000.0, 65500.0, 64800.0, 65200.0]]
        mock_session = MagicMock()
        mock_session.get.return_value.raise_for_status = MagicMock()
        mock_session.get.return_value.json.return_value = payload
        result = _fetch_ohlc(mock_session, "bitcoin", days=1, timeout=5)
        assert result == payload


# ---------------------------------------------------------------------------
# Gold crawler OHLC extraction
# ---------------------------------------------------------------------------

from src.crawlers.gold import GoldCrawler


def _make_yahoo_response(
    price: float = 2340.0,
    open_: float = 2330.0,
    high: float = 2350.0,
    low: float = 2320.0,
) -> dict:
    """Build a minimal Yahoo Finance v8 chart response with OHLC."""
    return {
        "chart": {
            "result": [
                {
                    "meta": {"regularMarketPrice": price},
                    "indicators": {
                        "quote": [
                            {
                                "open": [open_],
                                "high": [high],
                                "low": [low],
                                "close": [price],
                            }
                        ]
                    },
                }
            ]
        }
    }


def _make_yahoo_history_response(
    entries: list[tuple[int, float, float, float, float]],
) -> dict:
    """Build a Yahoo Finance v8 history response with multiple OHLC bars.
    entries: list of (unix_ts, open, high, low, close)
    """
    timestamps = [e[0] for e in entries]
    opens = [e[1] for e in entries]
    highs = [e[2] for e in entries]
    lows = [e[3] for e in entries]
    closes = [e[4] for e in entries]
    return {
        "chart": {
            "result": [
                {
                    "timestamp": timestamps,
                    "indicators": {
                        "quote": [
                            {
                                "open": opens,
                                "high": highs,
                                "low": lows,
                                "close": closes,
                            }
                        ]
                    },
                }
            ]
        }
    }


class TestGoldCrawlerXauOhlc:
    """_crawl_xau should extract OHLC from Yahoo response and pass to upsert."""

    def setup_method(self):
        self.crawler = GoldCrawler(timeout=5)

    def test_crawl_xau_extracts_ohlc(self):
        """_crawl_xau returns dicts with open/high/low_price set for XAU source."""
        mock_resp = MagicMock()
        mock_resp.raise_for_status = MagicMock()
        mock_resp.json.return_value = _make_yahoo_response(
            price=2340.0, open_=2330.0, high=2350.0, low=2320.0
        )

        with patch.object(self.crawler._session, "get", return_value=mock_resp):
            # Also patch VND rate fetch to avoid second network call
            with patch.object(self.crawler, "_fetch_usd_vnd_rate", return_value=Decimal("25000")):
                xau_usd, xau_vnd = self.crawler._crawl_xau()

        assert xau_usd is not None
        assert xau_usd["open_price"] == Decimal("2330.0")
        assert xau_usd["high_price"] == Decimal("2350.0")
        assert xau_usd["low_price"] == Decimal("2320.0")
        assert xau_usd["buy_price"] == Decimal("2340.0")

    def test_crawl_xau_vnd_ohlc_scaled(self):
        """XAU_VND OHLC should be open/high/low × vnd_rate."""
        mock_resp = MagicMock()
        mock_resp.raise_for_status = MagicMock()
        mock_resp.json.return_value = _make_yahoo_response(
            price=2000.0, open_=1990.0, high=2010.0, low=1980.0
        )

        rate = Decimal("25000")
        with patch.object(self.crawler._session, "get", return_value=mock_resp):
            with patch.object(self.crawler, "_fetch_usd_vnd_rate", return_value=rate):
                _, xau_vnd = self.crawler._crawl_xau()

        assert xau_vnd is not None
        assert xau_vnd["open_price"] == (Decimal("1990.0") * rate).quantize(Decimal("1"))
        assert xau_vnd["high_price"] == (Decimal("2010.0") * rate).quantize(Decimal("1"))
        assert xau_vnd["low_price"] == (Decimal("1980.0") * rate).quantize(Decimal("1"))

    def test_crawl_xau_missing_quote_ohlc_returns_none(self):
        """When indicators.quote is absent, OHLC should default to None (not raise)."""
        resp_data = {
            "chart": {
                "result": [
                    {
                        "meta": {"regularMarketPrice": 2340.0},
                        "indicators": {"quote": [{}]},   # empty quote block
                    }
                ]
            }
        }
        mock_resp = MagicMock()
        mock_resp.raise_for_status = MagicMock()
        mock_resp.json.return_value = resp_data

        with patch.object(self.crawler._session, "get", return_value=mock_resp):
            with patch.object(self.crawler, "_fetch_usd_vnd_rate", side_effect=Exception("no rate")):
                xau_usd, xau_vnd = self.crawler._crawl_xau()

        assert xau_usd is not None
        assert xau_usd["open_price"] is None
        assert xau_usd["high_price"] is None
        assert xau_usd["low_price"] is None
        assert xau_vnd is None  # VND call failed


class TestGoldCrawlerHistoryOhlc:
    """_import_xau_history should extract per-bar OHLC and forward to upsert."""

    def setup_method(self):
        self.crawler = GoldCrawler(timeout=5)

    def test_history_ohlc_forwarded(self):
        # Two days: 2024-01-02 and 2024-01-03 (UTC midnight → any ICT date in Jan)
        ts1 = int(datetime(2024, 1, 2, 0, 0, tzinfo=timezone.utc).timestamp())
        ts2 = int(datetime(2024, 1, 3, 0, 0, tzinfo=timezone.utc).timestamp())
        resp_data = _make_yahoo_history_response([
            (ts1, 2300.0, 2310.0, 2290.0, 2305.0),
            (ts2, 2305.0, 2320.0, 2295.0, 2315.0),
        ])
        mock_resp = MagicMock()
        mock_resp.raise_for_status = MagicMock()
        mock_resp.json.return_value = resp_data

        upserted: list[dict] = []

        def fake_upsert(**kwargs):
            upserted.append(kwargs)

        with patch.object(self.crawler._session, "get", return_value=mock_resp):
            with patch.object(self.crawler, "_fetch_usd_vnd_rate", side_effect=Exception("no rate")):
                import src.database.repository as repo_mod
                with patch.object(repo_mod, "upsert_gold_price", side_effect=fake_upsert):
                    saved = self.crawler._import_xau_history()

        # 2 XAU rows (no VND because rate fetch failed)
        assert saved == 2
        for row in upserted:
            assert row["open_price"] is not None
            assert row["high_price"] is not None
            assert row["low_price"] is not None
            assert row["source"] == "XAU"

    def test_history_vnd_ohlc_scaled(self):
        ts1 = int(datetime(2024, 1, 2, 0, 0, tzinfo=timezone.utc).timestamp())
        resp_data = _make_yahoo_history_response([(ts1, 2300.0, 2310.0, 2290.0, 2305.0)])

        mock_resp = MagicMock()
        mock_resp.raise_for_status = MagicMock()
        mock_resp.json.return_value = resp_data

        upserted: list[dict] = []

        def fake_upsert(**kwargs):
            upserted.append(kwargs)

        rate = Decimal("25000")
        with patch.object(self.crawler._session, "get", return_value=mock_resp):
            with patch.object(self.crawler, "_fetch_usd_vnd_rate", return_value=rate):
                import src.database.repository as repo_mod
                with patch.object(repo_mod, "upsert_gold_price", side_effect=fake_upsert):
                    saved = self.crawler._import_xau_history()

        # 1 XAU + 1 XAU_VND
        assert saved == 2
        vnd_row = next(r for r in upserted if r["source"] == "XAU_VND")
        assert vnd_row["open_price"] == (Decimal("2300.0") * rate).quantize(Decimal("1"))
        assert vnd_row["high_price"] == (Decimal("2310.0") * rate).quantize(Decimal("1"))
        assert vnd_row["low_price"] == (Decimal("2290.0") * rate).quantize(Decimal("1"))


# ---------------------------------------------------------------------------
# Repository signature compatibility (no DB needed)
# ---------------------------------------------------------------------------

class TestRepositorySignatureCompat:
    """Verify that upsert functions accept OHLC kwargs gracefully."""

    def test_upsert_gold_price_accepts_ohlc_kwargs(self):
        """upsert_gold_price should accept open/high/low_price without raising."""
        import src.database.repository as repo_mod
        # Patch session_scope to avoid real DB
        with patch("src.database.repository.session_scope") as mock_scope:
            mock_session = MagicMock()
            mock_session.query.return_value.filter_by.return_value.first.return_value = None
            mock_scope.return_value.__enter__ = MagicMock(return_value=mock_session)
            mock_scope.return_value.__exit__ = MagicMock(return_value=False)

            # Should not raise
            repo_mod.upsert_gold_price(
                source="XAU",
                product_type="spot",
                trading_date=datetime(2024, 6, 15),
                buy_price=Decimal("2340"),
                sell_price=Decimal("2340"),
                currency="USD",
                open_price=Decimal("2330"),
                high_price=Decimal("2350"),
                low_price=Decimal("2320"),
            )

    def test_upsert_gold_price_works_without_ohlc(self):
        """Legacy callers that omit OHLC should still work (defaults to None)."""
        import src.database.repository as repo_mod
        with patch("src.database.repository.session_scope") as mock_scope:
            mock_session = MagicMock()
            mock_session.query.return_value.filter_by.return_value.first.return_value = None
            mock_scope.return_value.__enter__ = MagicMock(return_value=mock_session)
            mock_scope.return_value.__exit__ = MagicMock(return_value=False)

            repo_mod.upsert_gold_price(
                source="BTMC",
                product_type="sjc",
                trading_date=datetime(2024, 6, 15),
                buy_price=Decimal("8000000"),
                sell_price=Decimal("8100000"),
                currency="VND",
                # No OHLC args — should work with defaults
            )

    def test_upsert_crypto_price_accepts_ohlc_kwargs(self):
        """upsert_crypto_price should accept open/high/low_price without raising."""
        import src.database.repository as repo_mod
        with patch("src.database.repository.session_scope") as mock_scope:
            mock_session = MagicMock()
            mock_session.query.return_value.filter_by.return_value.first.return_value = None
            mock_scope.return_value.__enter__ = MagicMock(return_value=mock_session)
            mock_scope.return_value.__exit__ = MagicMock(return_value=False)

            repo_mod.upsert_crypto_price(
                coin_id="bitcoin",
                symbol="BTC",
                trading_date=datetime(2024, 6, 15).date(),
                close_price=Decimal("65000"),
                market_cap=Decimal("1200000000000"),
                volume_24h=Decimal("30000000000"),
                currency="USD",
                open_price=Decimal("64000"),
                high_price=Decimal("65500"),
                low_price=Decimal("63800"),
            )

    def test_upsert_crypto_price_works_without_ohlc(self):
        """Legacy callers omitting OHLC still work (defaults to None)."""
        import src.database.repository as repo_mod
        with patch("src.database.repository.session_scope") as mock_scope:
            mock_session = MagicMock()
            mock_session.query.return_value.filter_by.return_value.first.return_value = None
            mock_scope.return_value.__enter__ = MagicMock(return_value=mock_session)
            mock_scope.return_value.__exit__ = MagicMock(return_value=False)

            repo_mod.upsert_crypto_price(
                coin_id="ethereum",
                symbol="ETH",
                trading_date=datetime(2024, 6, 15).date(),
                close_price=Decimal("3500"),
                market_cap=None,
                volume_24h=None,
                currency="USD",
            )
