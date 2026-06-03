"""Unit tests for PerformanceMetrics from prediction/src/simulation/metrics.py.

No network, no DB, no Docker required — pure computation logic.
"""
from __future__ import annotations

import math
from datetime import date, timedelta

import pytest

from src.simulation.metrics import PerformanceMetrics, SimKPIs


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_snapshots(
    n: int,
    start_value: float,
    end_value: float,
    start_date: date = date(2024, 1, 1),
) -> list[dict]:
    """Create list of snapshots increasing linearly from start_value to end_value."""
    snaps = []
    for i in range(n):
        d = start_date + timedelta(days=i)
        value = start_value + (end_value - start_value) * i / max(n - 1, 1)
        snaps.append({
            "snapshot_date": d,
            "total_value": value,
            "total_return_pct": (value - start_value) / start_value * 100,
        })
    return snaps


def make_sell_trade(
    pnl: float,
    pnl_pct: float,
    exit_date: date = date(2024, 2, 1),
    entry_date: date = date(2024, 1, 15),
) -> dict:
    return {
        "pnl": pnl,
        "pnl_pct": pnl_pct,
        "trade_date": exit_date,
        "entry_date": entry_date,
    }


# ---------------------------------------------------------------------------
# Empty / edge cases
# ---------------------------------------------------------------------------

class TestPerformanceMetricsEmpty:

    def test_empty_snapshots_returns_zero_kpis(self):
        kpis = PerformanceMetrics.compute([], [], 100_000.0)
        assert isinstance(kpis, SimKPIs)
        assert kpis.total_return_pct == 0.0
        assert kpis.annualized_return_pct == 0.0
        assert kpis.sharpe_ratio == 0.0
        assert kpis.max_drawdown_pct == 0.0
        assert kpis.win_rate_pct == 0.0
        assert kpis.profit_factor == 0.0
        assert kpis.total_trades == 0
        assert kpis.avg_trade_duration_days == 0.0

    def test_no_trades_metrics_are_zero(self):
        snaps = make_snapshots(10, 100_000.0, 105_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.total_trades == 0
        assert kpis.win_rate_pct == 0.0
        assert kpis.profit_factor == 0.0
        assert kpis.avg_trade_duration_days == 0.0

    def test_single_snapshot_does_not_crash(self):
        snaps = make_snapshots(1, 100_000.0, 100_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert isinstance(kpis, SimKPIs)

    def test_sharpe_single_snapshot_is_zero(self):
        snaps = make_snapshots(1, 100_000.0, 100_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.sharpe_ratio == 0.0


# ---------------------------------------------------------------------------
# Total return
# ---------------------------------------------------------------------------

class TestTotalReturn:

    def test_total_return_positive(self):
        snaps = make_snapshots(30, 100_000.0, 110_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.total_return_pct > 0

    def test_total_return_negative(self):
        snaps = make_snapshots(30, 100_000.0, 90_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.total_return_pct < 0

    def test_total_return_zero(self):
        snaps = make_snapshots(30, 100_000.0, 100_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.total_return_pct == pytest.approx(0.0, abs=1e-9)

    def test_total_return_value_correct(self):
        snaps = make_snapshots(10, 100_000.0, 115_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        # (115000 - 100000) / 100000 * 100 = 15%
        assert kpis.total_return_pct == pytest.approx(15.0, rel=1e-6)


# ---------------------------------------------------------------------------
# Annualized return
# ---------------------------------------------------------------------------

class TestAnnualizedReturn:

    def test_annualized_return_calculation(self):
        # 10% return in exactly 365 days → annualized ≈ 10%
        snaps = make_snapshots(366, 100_000.0, 110_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        # (1 + 0.10)^(365/365) - 1 = 10%
        assert kpis.annualized_return_pct == pytest.approx(10.0, rel=1e-2)

    def test_annualized_return_single_day_is_zero(self):
        # Only 1 snapshot → days=0 → annualized=0
        snaps = make_snapshots(1, 100_000.0, 100_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.annualized_return_pct == 0.0

    def test_annualized_return_same_date_is_zero(self):
        # 2 snapshots on same date → days=0
        snaps = [
            {"snapshot_date": date(2024, 1, 1), "total_value": 100_000.0, "total_return_pct": 0.0},
            {"snapshot_date": date(2024, 1, 1), "total_value": 105_000.0, "total_return_pct": 5.0},
        ]
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.annualized_return_pct == 0.0


# ---------------------------------------------------------------------------
# Sharpe ratio
# ---------------------------------------------------------------------------

class TestSharpeRatio:

    def test_sharpe_ratio_computed(self):
        snaps = make_snapshots(100, 100_000.0, 115_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        # Should be a valid float (could be positive for upward trend)
        assert isinstance(kpis.sharpe_ratio, float)
        assert not math.isnan(kpis.sharpe_ratio)

    def test_sharpe_two_snapshots_same_value_is_zero(self):
        # Two snapshots with same value → std=0 → sharpe=0
        snaps = [
            {"snapshot_date": date(2024, 1, 1), "total_value": 100_000.0, "total_return_pct": 0.0},
            {"snapshot_date": date(2024, 1, 2), "total_value": 100_000.0, "total_return_pct": 0.0},
        ]
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.sharpe_ratio == 0.0

    def test_sharpe_positive_for_consistently_rising_portfolio(self):
        # Consistently rising portfolio should have positive Sharpe
        snaps = make_snapshots(50, 100_000.0, 120_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.sharpe_ratio > 0


# ---------------------------------------------------------------------------
# Max drawdown
# ---------------------------------------------------------------------------

class TestMaxDrawdown:

    def test_max_drawdown_is_negative(self):
        # Portfolio that drops
        snaps = [
            {"snapshot_date": date(2024, 1, 1), "total_value": 100_000.0, "total_return_pct": 0.0},
            {"snapshot_date": date(2024, 1, 2), "total_value": 120_000.0, "total_return_pct": 20.0},
            {"snapshot_date": date(2024, 1, 3), "total_value": 90_000.0, "total_return_pct": -10.0},
        ]
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.max_drawdown_pct < 0

    def test_max_drawdown_no_drop_is_zero(self):
        snaps = make_snapshots(20, 100_000.0, 130_000.0)
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        assert kpis.max_drawdown_pct == pytest.approx(0.0, abs=1e-9)

    def test_max_drawdown_magnitude(self):
        # peak=120, trough=96 → drawdown = (120-96)/120*100 = 20%
        snaps = [
            {"snapshot_date": date(2024, 1, 1), "total_value": 100_000.0, "total_return_pct": 0.0},
            {"snapshot_date": date(2024, 1, 2), "total_value": 120_000.0, "total_return_pct": 20.0},
            {"snapshot_date": date(2024, 1, 3), "total_value": 96_000.0, "total_return_pct": -4.0},
        ]
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        # drawdown = (120000 - 96000) / 120000 * 100 = 20%, stored as -20
        assert kpis.max_drawdown_pct == pytest.approx(-20.0, rel=1e-6)

    def test_max_drawdown_considers_all_troughs(self):
        snaps = [
            {"snapshot_date": date(2024, 1, 1), "total_value": 100_000.0, "total_return_pct": 0.0},
            {"snapshot_date": date(2024, 1, 2), "total_value": 110_000.0, "total_return_pct": 10.0},
            {"snapshot_date": date(2024, 1, 3), "total_value": 99_000.0, "total_return_pct": -1.0},
            {"snapshot_date": date(2024, 1, 4), "total_value": 115_000.0, "total_return_pct": 15.0},
            {"snapshot_date": date(2024, 1, 5), "total_value": 80_000.0, "total_return_pct": -20.0},
        ]
        kpis = PerformanceMetrics.compute(snaps, [], 100_000.0)
        # max drawdown = (115000 - 80000) / 115000 * 100 ≈ 30.4%
        assert kpis.max_drawdown_pct < -29.0


# ---------------------------------------------------------------------------
# Win rate
# ---------------------------------------------------------------------------

class TestWinRate:

    def setup_method(self):
        self.snaps = make_snapshots(30, 100_000.0, 110_000.0)

    def test_win_rate_all_wins(self):
        trades = [
            make_sell_trade(500.0, 5.0),
            make_sell_trade(300.0, 3.0),
            make_sell_trade(200.0, 2.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.win_rate_pct == pytest.approx(100.0, rel=1e-9)

    def test_win_rate_all_losses(self):
        trades = [
            make_sell_trade(-500.0, -5.0),
            make_sell_trade(-300.0, -3.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.win_rate_pct == pytest.approx(0.0, abs=1e-9)

    def test_win_rate_half_half(self):
        trades = [
            make_sell_trade(500.0, 5.0),
            make_sell_trade(300.0, 3.0),
            make_sell_trade(-200.0, -2.0),
            make_sell_trade(-400.0, -4.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.win_rate_pct == pytest.approx(50.0, rel=1e-9)

    def test_win_rate_breakeven_treated_as_loss(self):
        # pnl=0 is treated as loss (not profit) per the implementation
        trades = [
            make_sell_trade(100.0, 1.0),
            make_sell_trade(0.0, 0.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.win_rate_pct == pytest.approx(50.0, rel=1e-9)


# ---------------------------------------------------------------------------
# Profit factor
# ---------------------------------------------------------------------------

class TestProfitFactor:

    def setup_method(self):
        self.snaps = make_snapshots(30, 100_000.0, 110_000.0)

    def test_profit_factor_calculation(self):
        # total_profit=100, total_loss=50 → profit_factor=2.0
        trades = [
            make_sell_trade(100.0, 10.0),
            make_sell_trade(-50.0, -5.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.profit_factor == pytest.approx(2.0, rel=1e-6)

    def test_profit_factor_no_losses(self):
        trades = [
            make_sell_trade(500.0, 5.0),
            make_sell_trade(300.0, 3.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.profit_factor == float("inf")

    def test_profit_factor_all_losses(self):
        trades = [
            make_sell_trade(-200.0, -2.0),
            make_sell_trade(-100.0, -1.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        # total_profit = 0 → profit_factor = 0 / 300 = 0
        assert kpis.profit_factor == pytest.approx(0.0, abs=1e-9)


# ---------------------------------------------------------------------------
# Total trades
# ---------------------------------------------------------------------------

class TestTotalTrades:

    def setup_method(self):
        self.snaps = make_snapshots(30, 100_000.0, 110_000.0)

    def test_total_trades_count(self):
        n = 7
        trades = [make_sell_trade(100.0, 1.0) for _ in range(n)]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.total_trades == n

    def test_total_trades_zero_when_no_trades(self):
        kpis = PerformanceMetrics.compute(self.snaps, [], 100_000.0)
        assert kpis.total_trades == 0

    def test_trades_without_pnl_not_counted(self):
        # Trade with pnl=None should be excluded
        trades = [
            {"pnl": None, "pnl_pct": None, "trade_date": date(2024, 2, 1), "entry_date": date(2024, 1, 15)},
            make_sell_trade(100.0, 1.0),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.total_trades == 1


# ---------------------------------------------------------------------------
# Best / worst trade
# ---------------------------------------------------------------------------

class TestBestWorstTrade:

    def setup_method(self):
        self.snaps = make_snapshots(30, 100_000.0, 110_000.0)

    def test_best_worst_trade(self):
        trades = [
            make_sell_trade(500.0, 5.0),
            make_sell_trade(-200.0, -2.0),
            make_sell_trade(1000.0, 10.0),
            make_sell_trade(-50.0, -0.5),
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.best_trade_pct == pytest.approx(10.0, rel=1e-9)
        assert kpis.worst_trade_pct == pytest.approx(-2.0, rel=1e-9)

    def test_best_trade_single_trade(self):
        trades = [make_sell_trade(300.0, 3.0)]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.best_trade_pct == pytest.approx(3.0, rel=1e-9)
        assert kpis.worst_trade_pct == pytest.approx(3.0, rel=1e-9)


# ---------------------------------------------------------------------------
# Average trade duration
# ---------------------------------------------------------------------------

class TestAvgTradeDuration:

    def setup_method(self):
        self.snaps = make_snapshots(60, 100_000.0, 110_000.0)

    def test_avg_trade_duration(self):
        # Trade 1: 10 days, Trade 2: 20 days → avg = 15
        t1 = make_sell_trade(
            100.0, 1.0,
            exit_date=date(2024, 1, 25),
            entry_date=date(2024, 1, 15),  # 10 days
        )
        t2 = make_sell_trade(
            200.0, 2.0,
            exit_date=date(2024, 2, 4),
            entry_date=date(2024, 1, 15),  # 20 days
        )
        kpis = PerformanceMetrics.compute(self.snaps, [t1, t2], 100_000.0)
        assert kpis.avg_trade_duration_days == pytest.approx(15.0, rel=1e-9)

    def test_avg_trade_duration_no_dates_is_zero(self):
        trades = [
            {"pnl": 100.0, "pnl_pct": 1.0, "trade_date": None, "entry_date": None},
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.avg_trade_duration_days == 0.0

    def test_avg_trade_duration_single_day_trade(self):
        t = make_sell_trade(
            50.0, 0.5,
            exit_date=date(2024, 1, 16),
            entry_date=date(2024, 1, 15),  # 1 day
        )
        kpis = PerformanceMetrics.compute(self.snaps, [t], 100_000.0)
        assert kpis.avg_trade_duration_days == pytest.approx(1.0, rel=1e-9)

    def test_avg_trade_duration_string_dates(self):
        # Engine may pass ISO date strings
        trades = [
            {
                "pnl": 100.0,
                "pnl_pct": 1.0,
                "trade_date": "2024-02-01",
                "entry_date": "2024-01-22",  # 10 days
            }
        ]
        kpis = PerformanceMetrics.compute(self.snaps, trades, 100_000.0)
        assert kpis.avg_trade_duration_days == pytest.approx(10.0, rel=1e-9)
