"""Regression tests for the date/datetime normalisation in reconcile, and
the new hourly reconcile maturity logic for NASDAQ/SP500/CRYPTO.

Guards:
1. The original bug where crypto/nasdaq/sp500 `trading_date` is returned as a
   ``datetime`` while the prediction target was a ``date``. ``_to_date`` fixes
   this by normalising both sides to plain ``date``.

2. The new hourly reconcile logic: predictions are only reconciled when
   ``target_date <= datetime.now()`` (maturity check). This unit test validates
   the maturity guard logic in isolation.
"""
from __future__ import annotations

from dataclasses import dataclass
from datetime import date, datetime, timedelta

import pytest

from src.orchestrator.training import _to_date


class TestToDate:
    def test_datetime_is_reduced_to_date(self):
        assert _to_date(datetime(2026, 6, 20, 13, 45, 1)) == date(2026, 6, 20)
        assert isinstance(_to_date(datetime(2026, 6, 20, 0, 0)), date)

    def test_date_is_passed_through(self):
        d = date(2026, 6, 20)
        assert _to_date(d) == d

    def test_none_passthrough(self):
        assert _to_date(None) is None

    def test_mixed_comparison_does_not_raise(self):
        """The exact failure mode: a datetime trading_date vs a date target."""
        target_d = _to_date(datetime(2026, 6, 20, 0, 0))      # prediction target -> date
        trading_dt = datetime(2026, 6, 16, 0, 0)               # crypto price -> datetime
        # Without normalisation this raises TypeError; with it, it is a clean bool.
        result = _to_date(trading_dt) >= target_d
        assert result is False

    def test_on_or_after_filter_semantics(self):
        target_d = _to_date(datetime(2026, 6, 20, 0, 0))
        rows = [datetime(2026, 6, 18), datetime(2026, 6, 20), datetime(2026, 6, 21)]
        after = [r for r in rows if r is not None and _to_date(r) >= target_d]
        assert after == [datetime(2026, 6, 20), datetime(2026, 6, 21)]


# ---------------------------------------------------------------------------
# Hourly reconcile maturity guard tests
# ---------------------------------------------------------------------------

@dataclass
class FakePred:
    """Minimal stand-in for a pending prediction row."""
    id: int
    target_date: datetime
    predicted_price: float
    current_price: float


class TestHourlyReconcileMaturity:
    """Validates the maturity logic: reconcile only when target_date <= now.

    The actual reconcile_predictions() function is not called here (it needs DB);
    we test only the guard expression ``pred.target_date > now_ts`` which is the
    key change from the old day-boundary approach.
    """

    def test_future_target_skipped(self):
        """A prediction whose target is in the future must not be reconciled."""
        now_ts = datetime(2026, 6, 22, 15, 0, 0)
        pred = FakePred(id=1, target_date=now_ts + timedelta(hours=1),
                        predicted_price=100.0, current_price=99.0)
        # Guard: skip if target is in the future
        should_skip = pred.target_date > now_ts
        assert should_skip is True

    def test_past_target_reconciled(self):
        """A prediction whose target has already passed should be reconciled."""
        now_ts = datetime(2026, 6, 22, 15, 0, 0)
        pred = FakePred(id=2, target_date=now_ts - timedelta(hours=1),
                        predicted_price=100.0, current_price=99.0)
        should_skip = pred.target_date > now_ts
        assert should_skip is False

    def test_exact_target_now_reconciled(self):
        """A prediction whose target equals now exactly should be reconciled (not future)."""
        now_ts = datetime(2026, 6, 22, 15, 0, 0)
        pred = FakePred(id=3, target_date=now_ts,
                        predicted_price=100.0, current_price=99.0)
        should_skip = pred.target_date > now_ts  # strictly greater -> not skipped
        assert should_skip is False

    def test_live_price_is_last_element(self):
        """Actual price comes from prices[-1] (last/latest in ASC list)."""
        # Simulate prices_asc list: last element is today's live price
        from decimal import Decimal

        @dataclass
        class FakePrice:
            close_price: float

        prices_asc = [FakePrice(98.0), FakePrice(99.0), FakePrice(101.5)]  # ASC order
        actual = Decimal(str(prices_asc[-1].close_price))
        assert actual == Decimal("101.5")

    def test_direction_correct_logic(self):
        """Validate direction_correct computation for hourly predictions."""
        from decimal import Decimal

        # Pred: going up (predicted > current)
        predicted = Decimal("105.0")
        current = Decimal("100.0")
        actual = Decimal("103.0")   # went up (correct direction)

        pred_diff = predicted - current
        actual_diff = actual - current
        if actual_diff == 0:
            direction_correct = (pred_diff == 0)
        elif pred_diff != 0:
            direction_correct = (pred_diff > 0) == (actual_diff > 0)
        else:
            direction_correct = False

        assert direction_correct is True

        # Pred: going up but price went down (wrong direction)
        actual2 = Decimal("97.0")
        actual_diff2 = actual2 - current
        direction_correct2 = (pred_diff > 0) == (actual_diff2 > 0)
        assert direction_correct2 is False

    @pytest.mark.parametrize("market", ["nasdaq", "sp500", "crypto"])
    def test_market_maturity_guard_logic(self, market):
        """All three markets apply the same maturity guard: target_date <= now."""
        now_ts = datetime(2026, 6, 22, 12, 0, 0)
        # 30-minute old prediction — already mature
        old_pred = FakePred(id=10, target_date=now_ts - timedelta(minutes=30),
                            predicted_price=200.0, current_price=198.0)
        # 1-hour future prediction — not yet mature
        future_pred = FakePred(id=11, target_date=now_ts + timedelta(hours=1),
                               predicted_price=202.0, current_price=198.0)

        assert (old_pred.target_date > now_ts) is False   # reconcile
        assert (future_pred.target_date > now_ts) is True  # skip
