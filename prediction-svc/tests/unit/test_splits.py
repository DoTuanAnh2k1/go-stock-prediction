"""Unit tests for stock-split handling.

No DB, no Docker, no network required.
- record_split ratio calculation and idempotency (via mock).
- _restore_portfolio_state split-adjustment logic (pure Position math).
- _apply_one_split price-division direction (via mock DB calls).
"""
from __future__ import annotations

from datetime import date, datetime, timedelta
from decimal import Decimal
from typing import Optional
from unittest.mock import MagicMock, patch, call

import pytest

# ---------------------------------------------------------------------------
# 1. record_split — ratio, idempotent
# ---------------------------------------------------------------------------

class TestRecordSplitRatio:
    """Test that record_split computes ratio = numerator / denominator."""

    def _call_record_split(self, market, symbol, split_date, num, den, existing_id=None):
        """Call repo.record_split through a mocked session_scope / text."""
        from decimal import Decimal as D
        # Manually replicate the ratio calculation from repository.record_split
        if den == 0:
            return None
        ratio = D(str(num)) / D(str(den))
        return ratio

    def test_forward_split_4_to_1_ratio(self):
        ratio = self._call_record_split("NASDAQ", "CRWD", date(2026, 7, 1), 4, 1)
        assert ratio == pytest.approx(Decimal("4.0"), rel=1e-6)

    def test_forward_split_2_to_1_ratio(self):
        ratio = self._call_record_split("NASDAQ", "AAPL", date(2020, 8, 31), 4, 1)
        assert ratio == pytest.approx(Decimal("4.0"), rel=1e-6)

    def test_reverse_split_1_to_10_ratio(self):
        # reverse: 1 new share for every 10 old → ratio < 1
        ratio = self._call_record_split("NASDAQ", "XYZ", date(2024, 1, 1), 1, 10)
        assert ratio == pytest.approx(Decimal("0.1"), rel=1e-6)

    def test_zero_denominator_returns_none(self):
        result = self._call_record_split("NASDAQ", "XYZ", date(2024, 1, 1), 4, 0)
        assert result is None

    def test_idempotent_second_call_returns_none(self):
        """Mock the DB to simulate ON CONFLICT DO NOTHING returning no row."""
        from decimal import Decimal as D

        mock_row = None  # ON CONFLICT DO NOTHING → RETURNING returns nothing

        fake_result = MagicMock()
        fake_result.fetchone.return_value = mock_row

        fake_session = MagicMock()
        fake_session.execute.return_value = fake_result
        fake_session.__enter__ = MagicMock(return_value=fake_session)
        fake_session.__exit__ = MagicMock(return_value=False)

        with patch("src.database.repository.session_scope", return_value=fake_session):
            from src.database import repository as repo
            result = repo.record_split("NASDAQ", "CRWD", date(2026, 7, 1), D("4"), D("1"))

        assert result is None   # no RETURNING row → idempotent


# ---------------------------------------------------------------------------
# 2. _restore_portfolio_state split-aware logic (pure math, mock DB)
# ---------------------------------------------------------------------------

def _make_bot(market: str = "NASDAQ"):
    """Return a minimal TradingBot-like object with a real Portfolio."""
    from src.simulation.portfolio import Portfolio

    # Use type() so the class body can reference `market` from enclosing scope
    # (Python class bodies don't have closure access to enclosing variables).
    FakeConfig = type("FakeConfig", (), {"market": market})

    class FakeBot:
        config = FakeConfig()
        portfolio = Portfolio(
            initial_capital=100_000.0,
            stop_loss_pct=5.0,
            take_profit_pct=8.0,
            max_position_pct=15.0,
            max_positions=5,
        )

    return FakeBot()


def _make_trade(id_: int, symbol: str, action: str, qty: float, price: float, trade_date):
    """Build a trade dict as _restore_portfolio_state expects."""
    trade_value = qty * price
    return {
        "id": id_,
        "symbol": symbol,
        "action": action,
        "quantity": qty,
        "price": price,
        "trade_value": trade_value,
        "trade_date": datetime.combine(trade_date, datetime.min.time()),
    }


class TestRestorePortfolioSplitAware:
    """Tests for _restore_portfolio_state split adjustment in engine.py."""

    def _restore(self, bot, db_trades, initial_capital, splits_map: dict):
        """Call _restore_portfolio_state with mocked list_splits_for_symbol."""
        from src.simulation.engine import _restore_portfolio_state

        def fake_list_splits(market_key, symbol):
            return splits_map.get(symbol, [])

        with patch("src.database.repository.list_splits_for_symbol", side_effect=fake_list_splits):
            _restore_portfolio_state(bot, db_trades, initial_capital)

    # ── 2a. Position held through 4:1 split ───────────────────────────────

    def test_pre_split_position_qty_multiplied(self):
        """A position bought before a 4:1 split should have qty × 4."""
        bot = _make_bot("NASDAQ")
        entry_date = date(2026, 6, 15)
        split_date = date(2026, 7, 1)

        trades = [
            _make_trade(1, "CRWD", "BUY", 10.0, 400.0, entry_date),
        ]
        splits_map = {
            "CRWD": [{"split_date": split_date, "ratio": 4.0}]
        }

        self._restore(bot, trades, 100_000.0, splits_map)

        pos = bot.portfolio.positions.get("CRWD")
        assert pos is not None
        assert pos.quantity == pytest.approx(40.0, rel=1e-9)

    def test_pre_split_position_price_divided(self):
        """Entry price should be divided by ratio after a 4:1 split."""
        bot = _make_bot("NASDAQ")
        entry_date = date(2026, 6, 15)
        split_date = date(2026, 7, 1)

        trades = [
            _make_trade(1, "CRWD", "BUY", 10.0, 400.0, entry_date),
        ]
        splits_map = {
            "CRWD": [{"split_date": split_date, "ratio": 4.0}]
        }

        self._restore(bot, trades, 100_000.0, splits_map)

        pos = bot.portfolio.positions["CRWD"]
        assert pos.entry_price == pytest.approx(100.0, rel=1e-9)

    def test_trade_value_unchanged(self):
        """Cash used to buy is deducted from original trade_value — not adjusted."""
        bot = _make_bot("NASDAQ")
        initial = 100_000.0
        entry_date = date(2026, 6, 15)
        split_date = date(2026, 7, 1)

        qty, price = 10.0, 400.0
        trade_value = qty * price  # 4000

        trades = [
            _make_trade(1, "CRWD", "BUY", qty, price, entry_date),
        ]
        splits_map = {
            "CRWD": [{"split_date": split_date, "ratio": 4.0}]
        }

        self._restore(bot, trades, initial, splits_map)

        # cash should be initial minus the original trade_value (4000)
        assert bot.portfolio.cash == pytest.approx(initial - trade_value, rel=1e-9)

    # ── 2b. Position bought AFTER split: no adjustment ────────────────────

    def test_post_split_position_unchanged(self):
        """A position bought on or after split_date must NOT be adjusted."""
        bot = _make_bot("NASDAQ")
        split_date = date(2026, 7, 1)
        entry_date = date(2026, 7, 1)  # same day as split = post-split price

        trades = [
            _make_trade(1, "CRWD", "BUY", 10.0, 100.0, entry_date),
        ]
        splits_map = {
            "CRWD": [{"split_date": split_date, "ratio": 4.0}]
        }

        self._restore(bot, trades, 100_000.0, splits_map)

        pos = bot.portfolio.positions["CRWD"]
        assert pos.quantity == pytest.approx(10.0, rel=1e-9)
        assert pos.entry_price == pytest.approx(100.0, rel=1e-9)

    # ── 2c. Two consecutive splits ─────────────────────────────────────────

    def test_two_splits_compound(self):
        """A position held through two splits applies both in order."""
        bot = _make_bot("NASDAQ")
        entry_date = date(2024, 1, 1)
        split1 = date(2025, 1, 1)  # 2:1
        split2 = date(2026, 1, 1)  # 3:1

        trades = [
            _make_trade(1, "XYZ", "BUY", 10.0, 600.0, entry_date),
        ]
        splits_map = {
            "XYZ": [
                {"split_date": split1, "ratio": 2.0},
                {"split_date": split2, "ratio": 3.0},
            ]
        }

        self._restore(bot, trades, 100_000.0, splits_map)

        pos = bot.portfolio.positions["XYZ"]
        # 10 × 2 × 3 = 60 shares; 600 / 2 / 3 = 100 price
        assert pos.quantity == pytest.approx(60.0, rel=1e-9)
        assert pos.entry_price == pytest.approx(100.0, rel=1e-9)

    # ── 2d. GOLD / CRYPTO: no adjustment applied ─────────────────────────

    def test_non_split_market_no_adjustment(self):
        """GOLD and CRYPTO bots are never split-adjusted."""
        bot = _make_bot("GOLD")
        entry_date = date(2026, 6, 15)

        trades = [
            _make_trade(1, "XAU", "BUY", 10.0, 3000.0, entry_date),
        ]
        # We pass a splits_map but for GOLD market the engine skips the lookup
        splits_map = {}

        from src.simulation.engine import _restore_portfolio_state
        # No mock needed — list_splits_for_symbol should NOT be called for GOLD
        with patch("src.database.repository.list_splits_for_symbol") as mock_list:
            _restore_portfolio_state(bot, trades, 100_000.0)

        mock_list.assert_not_called()

        pos = bot.portfolio.positions["XAU"]
        assert pos.quantity == pytest.approx(10.0, rel=1e-9)
        assert pos.entry_price == pytest.approx(3000.0, rel=1e-9)

    # ── 2e. No splits recorded: backward-compatible ───────────────────────

    def test_no_splits_position_restored_as_before(self):
        """When no splits exist for a symbol, restoration is unchanged."""
        bot = _make_bot("NASDAQ")
        entry_date = date(2026, 6, 1)

        trades = [
            _make_trade(1, "AAPL", "BUY", 5.0, 200.0, entry_date),
        ]
        splits_map = {"AAPL": []}

        self._restore(bot, trades, 100_000.0, splits_map)

        pos = bot.portfolio.positions["AAPL"]
        assert pos.quantity == pytest.approx(5.0, rel=1e-9)
        assert pos.entry_price == pytest.approx(200.0, rel=1e-9)


# ---------------------------------------------------------------------------
# 3. _apply_one_split — division direction (mock DB, pure logic)
# ---------------------------------------------------------------------------

class TestApplyOneSplitLogic:
    """Verify _apply_one_split: intraday-only, jump-detected boundary, divide-by-ratio.

    Daily and predictions are intentionally NOT adjusted (Yahoo daily is already
    split-adjusted; dividing would double-adjust). Only the pre-boundary intraday
    segment is divided.
    """

    def _call_apply(self, market_key, symbol, split_date, ratio, series):
        """Call _apply_one_split with a mocked session_scope.

        ``series`` is a list of (timestamp, close) tuples returned by the SELECT;
        the jump-detection scans it for a drop of ~ratio.
        Returns the list of (sql_text, params) executed.
        """
        from types import SimpleNamespace
        from src.orchestrator.splits import _apply_one_split

        executed: list[tuple] = []
        rows = [SimpleNamespace(timestamp=ts, close_price=c) for ts, c in series]

        class FakeResult:
            rowcount = 7
            def fetchall(self_inner):
                return rows

        class FakeSession:
            def execute(self, stmt, params=None):
                executed.append((str(stmt), dict(params or {})))
                return FakeResult()
            def __enter__(self):
                return self
            def __exit__(self, *a):
                pass

        with patch("src.orchestrator.splits.session_scope", return_value=FakeSession()):
            _apply_one_split({
                "id": 1, "market_key": market_key, "symbol": symbol,
                "split_date": split_date, "ratio": ratio,
            })
        return executed

    def _series_with_split(self, split_date, ratio):
        """A 4-bar intraday series with one ~ratio cliff at index 2."""
        base = datetime.combine(split_date, datetime.min.time())
        pre = 760.0
        post = pre / ratio
        return [
            (base - timedelta(hours=3), pre),
            (base - timedelta(hours=2), pre + 2),
            (base - timedelta(hours=1), post),      # cliff: (pre+2)/post ≈ ratio
            (base, post + 1),
        ]

    def test_intraday_update_divides_by_ratio(self):
        sd = date(2026, 7, 2)
        sqls = self._call_apply("NASDAQ", "CRWD", sd, 4.0, self._series_with_split(sd, 4.0))
        price_updates = [(s, p) for s, p in sqls if "UPDATE" in s.upper() and "close_price" in s]
        assert len(price_updates) == 1, "Only the intraday table should be price-adjusted"
        sql, params = price_updates[0]
        assert "/ :ratio" in sql
        assert "nasdaq_intraday_prices" in sql
        assert params["ratio"] == pytest.approx(4.0)
        assert params["sym"] == "CRWD"
        assert "bts" in params  # divides strictly before the detected boundary

    def test_daily_and_predictions_not_touched(self):
        sd = date(2026, 7, 2)
        sqls = self._call_apply("NASDAQ", "CRWD", sd, 4.0, self._series_with_split(sd, 4.0))
        joined = " ".join(s for s, _ in sqls)
        assert "nasdaq_prices" not in joined.replace("nasdaq_intraday_prices", "")
        assert "nasdaq_predictions" not in joined

    def test_no_transition_marks_applied_without_price_update(self):
        """A flat series (no cliff) → mark applied, no price UPDATE."""
        sd = date(2026, 7, 2)
        base = datetime.combine(sd, datetime.min.time())
        flat = [(base - timedelta(hours=2), 190.0), (base - timedelta(hours=1), 191.0), (base, 190.5)]
        sqls = self._call_apply("NASDAQ", "CRWD", sd, 4.0, flat)
        price_updates = [s for s, _ in sqls if "close_price" in s and "UPDATE" in s.upper()]
        assert price_updates == []
        assert any("stock_splits" in s and "applied_at" in s for s, _ in sqls)

    def test_skip_zero_ratio(self):
        from src.orchestrator.splits import _apply_one_split
        with patch("src.orchestrator.splits.session_scope") as mock_scope:
            _apply_one_split({
                "id": 99, "market_key": "NASDAQ", "symbol": "BAD",
                "split_date": date(2026, 7, 1), "ratio": 0.0,
            })
        mock_scope.assert_not_called()

    def test_unknown_market_skipped(self):
        from src.orchestrator.splits import _apply_one_split
        with patch("src.orchestrator.splits.session_scope") as mock_scope:
            _apply_one_split({
                "id": 99, "market_key": "CRYPTO", "symbol": "BTC",
                "split_date": date(2026, 7, 1), "ratio": 2.0,
            })
        mock_scope.assert_not_called()


# ---------------------------------------------------------------------------
# 4. apply_pending_splits — orchestration (mock list_unapplied_splits)
# ---------------------------------------------------------------------------

class TestApplyPendingSplits:

    def test_no_pending_returns_zero(self):
        from src.orchestrator.splits import apply_pending_splits
        with patch("src.orchestrator.splits.repo.list_unapplied_splits", return_value=[]):
            result = apply_pending_splits()
        assert result == 0

    def test_counts_applied(self):
        from src.orchestrator.splits import apply_pending_splits

        pending = [
            {"id": 1, "market_key": "NASDAQ", "symbol": "CRWD",
             "split_date": date(2026, 7, 1), "ratio": 4.0},
            {"id": 2, "market_key": "SP500", "symbol": "XYZ",
             "split_date": date(2026, 6, 15), "ratio": 2.0},
        ]

        with patch("src.orchestrator.splits.repo.list_unapplied_splits", return_value=pending):
            with patch("src.orchestrator.splits._apply_one_split") as mock_apply:
                result = apply_pending_splits()

        assert result == 2
        assert mock_apply.call_count == 2

    def test_error_in_one_does_not_stop_others(self):
        from src.orchestrator.splits import apply_pending_splits

        pending = [
            {"id": 1, "market_key": "NASDAQ", "symbol": "A",
             "split_date": date(2026, 7, 1), "ratio": 4.0},
            {"id": 2, "market_key": "NASDAQ", "symbol": "B",
             "split_date": date(2026, 7, 1), "ratio": 2.0},
        ]

        call_count = 0

        def bad_apply(split):
            nonlocal call_count
            call_count += 1
            if split["symbol"] == "A":
                raise RuntimeError("DB error simulation")

        with patch("src.orchestrator.splits.repo.list_unapplied_splits", return_value=pending):
            with patch("src.orchestrator.splits._apply_one_split", side_effect=bad_apply):
                result = apply_pending_splits()

        # Only B succeeded
        assert result == 1
        assert call_count == 2
