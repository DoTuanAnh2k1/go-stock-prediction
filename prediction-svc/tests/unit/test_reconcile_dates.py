"""Regression tests for the date/datetime normalisation in reconcile.

Guards the bug where crypto/nasdaq/sp500 `trading_date` is returned as a
``datetime`` (DB column is TIMESTAMP) while the prediction target was reduced to
a ``date``. Because ``datetime`` subclasses ``date``, an ``isinstance(x, date)``
guard did NOT prevent a ``datetime >= date`` comparison, which raises
``TypeError: can't compare datetime.datetime to datetime.date``.

``_to_date`` normalises both sides to plain ``date`` so the comparison is safe.
"""
from __future__ import annotations

from datetime import date, datetime

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
