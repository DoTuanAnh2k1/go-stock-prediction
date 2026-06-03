"""Unit tests for Portfolio class from prediction/src/simulation/portfolio.py.

No network, no DB, no Docker required — pure portfolio logic.
"""
from __future__ import annotations

from datetime import date

import pytest

from src.simulation.portfolio import Portfolio, Position, Trade


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_portfolio(
    initial_capital: float = 100_000.0,
    stop_loss_pct: float = 5.0,
    take_profit_pct: float = 8.0,
    max_position_pct: float = 15.0,
    max_positions: int = 5,
) -> Portfolio:
    return Portfolio(
        initial_capital=initial_capital,
        stop_loss_pct=stop_loss_pct,
        take_profit_pct=take_profit_pct,
        max_position_pct=max_position_pct,
        max_positions=max_positions,
    )


TODAY = date(2024, 6, 1)


# ---------------------------------------------------------------------------
# Portfolio initialization
# ---------------------------------------------------------------------------

class TestPortfolioInitialization:

    def test_initial_state(self):
        p = make_portfolio(initial_capital=200_000.0)
        assert p.cash == 200_000.0
        assert p.initial_capital == 200_000.0
        assert p.positions == {}

    def test_total_value_no_positions(self):
        p = make_portfolio(initial_capital=100_000.0)
        assert p.total_value({}) == 100_000.0


# ---------------------------------------------------------------------------
# BUY
# ---------------------------------------------------------------------------

class TestPortfolioBuy:

    def setup_method(self):
        self.p = make_portfolio()

    def test_buy_creates_position(self):
        self.p.buy("VCB", 100.0, TODAY)
        assert "VCB" in self.p.positions
        assert len(self.p.positions) == 1

    def test_buy_correct_quantity(self):
        # With 100k capital, 15% max_position_pct, 5 slots
        # max_by_pct = 100000 * 0.15 = 15000
        # max_by_slots = 100000 / 5 = 20000
        # position_cash = min(15000, 20000, 100000) = 15000
        trade = self.p.buy("VCB", 100.0, TODAY)
        expected_qty = 15000.0 / 100.0
        assert trade.quantity == pytest.approx(expected_qty, rel=1e-9)

    def test_buy_returns_trade_object(self):
        trade = self.p.buy("VCB", 100.0, TODAY)
        assert isinstance(trade, Trade)
        assert trade.action == "BUY"
        assert trade.symbol == "VCB"
        assert trade.price == 100.0

    def test_buy_cash_decreases(self):
        initial_cash = self.p.cash
        trade = self.p.buy("VCB", 100.0, TODAY)
        assert self.p.cash == pytest.approx(initial_cash - trade.trade_value, rel=1e-9)

    def test_buy_respects_max_positions(self):
        symbols = ["VCB", "BID", "CTG", "TCB", "MBB"]
        for sym in symbols:
            self.p.buy(sym, 50.0, TODAY)
        # Now at max_positions (5), next buy should return None
        result = self.p.buy("HPG", 50.0, TODAY)
        assert result is None

    def test_buy_same_symbol_twice_returns_none(self):
        self.p.buy("VCB", 100.0, TODAY)
        result = self.p.buy("VCB", 110.0, TODAY)
        assert result is None

    def test_buy_price_zero_returns_none(self):
        result = self.p.buy("VCB", 0.0, TODAY)
        assert result is None

    def test_buy_no_cash_returns_none(self):
        # Use up all cash by buying many symbols (force cash to 0 via constructor)
        p = make_portfolio(initial_capital=0.0)
        result = p.buy("VCB", 100.0, TODAY)
        assert result is None

    def test_buy_negative_price_returns_none(self):
        result = self.p.buy("VCB", -10.0, TODAY)
        assert result is None


# ---------------------------------------------------------------------------
# SELL
# ---------------------------------------------------------------------------

class TestPortfolioSell:

    def setup_method(self):
        self.p = make_portfolio()

    def test_sell_removes_position(self):
        self.p.buy("VCB", 100.0, TODAY)
        assert "VCB" in self.p.positions
        self.p.sell("VCB", 110.0, TODAY)
        assert "VCB" not in self.p.positions

    def test_sell_returns_cash(self):
        buy_trade = self.p.buy("VCB", 100.0, TODAY)
        cash_after_buy = self.p.cash
        sell_trade = self.p.sell("VCB", 110.0, TODAY)
        expected_cash = cash_after_buy + buy_trade.quantity * 110.0
        assert self.p.cash == pytest.approx(expected_cash, rel=1e-9)

    def test_sell_computes_pnl(self):
        buy_trade = self.p.buy("VCB", 100.0, TODAY)
        sell_trade = self.p.sell("VCB", 110.0, TODAY)
        expected_pnl = (110.0 - 100.0) * buy_trade.quantity
        assert sell_trade.pnl == pytest.approx(expected_pnl, rel=1e-9)

    def test_sell_computes_pnl_pct(self):
        self.p.buy("VCB", 100.0, TODAY)
        sell_trade = self.p.sell("VCB", 110.0, TODAY)
        # pnl_pct = (exit - entry) / entry * 100 = (110-100)/100*100 = 10%
        assert sell_trade.pnl_pct == pytest.approx(10.0, rel=1e-9)

    def test_sell_nonexistent_symbol_returns_none(self):
        result = self.p.sell("NONEXISTENT", 100.0, TODAY)
        assert result is None

    def test_sell_profit(self):
        self.p.buy("VCB", 100.0, TODAY)
        sell_trade = self.p.sell("VCB", 120.0, TODAY)
        assert sell_trade.pnl > 0

    def test_sell_loss(self):
        self.p.buy("VCB", 100.0, TODAY)
        sell_trade = self.p.sell("VCB", 80.0, TODAY)
        assert sell_trade.pnl < 0

    def test_sell_returns_trade_object(self):
        self.p.buy("VCB", 100.0, TODAY)
        sell_trade = self.p.sell("VCB", 110.0, TODAY)
        assert isinstance(sell_trade, Trade)
        assert sell_trade.action == "SELL"
        assert sell_trade.symbol == "VCB"
        assert sell_trade.price == 110.0

    def test_sell_breakeven_pnl_is_zero(self):
        self.p.buy("VCB", 100.0, TODAY)
        sell_trade = self.p.sell("VCB", 100.0, TODAY)
        assert sell_trade.pnl == pytest.approx(0.0, abs=1e-9)
        assert sell_trade.pnl_pct == pytest.approx(0.0, abs=1e-9)


# ---------------------------------------------------------------------------
# Stop Loss / Take Profit
# ---------------------------------------------------------------------------

class TestStopLossTakeProfit:

    def setup_method(self):
        # stop_loss=5%, take_profit=8%
        self.p = make_portfolio(stop_loss_pct=5.0, take_profit_pct=8.0)

    def test_stop_loss_triggers(self):
        self.p.buy("VCB", 100.0, TODAY)
        # price drops > 5% → triggers stop loss
        sl_price = 100.0 * (1 - 0.06)  # -6%, beyond 5% threshold
        trades = self.p.check_stop_loss_take_profit({"VCB": sl_price}, TODAY)
        assert len(trades) == 1
        assert trades[0].symbol == "VCB"

    def test_take_profit_triggers(self):
        self.p.buy("VCB", 100.0, TODAY)
        # price rises > 8% → triggers take profit
        tp_price = 100.0 * (1 + 0.09)  # +9%, beyond 8% threshold
        trades = self.p.check_stop_loss_take_profit({"VCB": tp_price}, TODAY)
        assert len(trades) == 1
        assert trades[0].symbol == "VCB"

    def test_sl_tp_does_not_trigger_within_threshold(self):
        self.p.buy("VCB", 100.0, TODAY)
        # price within ±4%, no trigger
        within_price = 100.0 * (1 + 0.03)  # +3%, within 8% TP threshold
        trades = self.p.check_stop_loss_take_profit({"VCB": within_price}, TODAY)
        assert len(trades) == 0

    def test_sl_tp_no_price_skips(self):
        self.p.buy("VCB", 100.0, TODAY)
        # no price provided for VCB — should skip
        trades = self.p.check_stop_loss_take_profit({}, TODAY)
        assert len(trades) == 0
        # Position should still be there
        assert "VCB" in self.p.positions

    def test_close_reason_stop_loss(self):
        self.p.buy("VCB", 100.0, TODAY)
        sl_price = 100.0 * 0.94  # -6%
        trades = self.p.check_stop_loss_take_profit({"VCB": sl_price}, TODAY)
        assert len(trades) == 1
        assert trades[0].close_reason == "stop_loss"

    def test_close_reason_take_profit(self):
        self.p.buy("VCB", 100.0, TODAY)
        tp_price = 100.0 * 1.09  # +9%
        trades = self.p.check_stop_loss_take_profit({"VCB": tp_price}, TODAY)
        assert len(trades) == 1
        assert trades[0].close_reason == "take_profit"

    def test_sl_exactly_at_threshold_does_not_trigger(self):
        self.p.buy("VCB", 100.0, TODAY)
        # exactly -5% — condition is <= -5, so this triggers
        exact_sl = 100.0 * 0.95
        trades = self.p.check_stop_loss_take_profit({"VCB": exact_sl}, TODAY)
        # change_pct = -5.0 → -5.0 <= -5.0 → triggers
        assert len(trades) == 1

    def test_sl_tp_position_removed_after_trigger(self):
        self.p.buy("VCB", 100.0, TODAY)
        sl_price = 100.0 * 0.90  # -10%
        self.p.check_stop_loss_take_profit({"VCB": sl_price}, TODAY)
        assert "VCB" not in self.p.positions

    def test_multiple_positions_independent_sl_tp(self):
        self.p.buy("VCB", 100.0, TODAY)
        self.p.buy("BID", 200.0, TODAY)
        # VCB triggers SL, BID stays
        prices = {"VCB": 90.0, "BID": 202.0}  # VCB -10%, BID +1%
        trades = self.p.check_stop_loss_take_profit(prices, TODAY)
        assert len(trades) == 1
        assert trades[0].symbol == "VCB"
        assert "BID" in self.p.positions


# ---------------------------------------------------------------------------
# Snapshot
# ---------------------------------------------------------------------------

class TestPortfolioSnapshot:

    def setup_method(self):
        self.p = make_portfolio(initial_capital=100_000.0)

    def test_snapshot_returns_correct_fields(self):
        snap = self.p.snapshot({}, TODAY)
        required_keys = {"snapshot_date", "cash_balance", "positions_value", "total_value", "total_return_pct", "open_positions"}
        assert required_keys.issubset(snap.keys())

    def test_snapshot_total_return_zero_at_start(self):
        snap = self.p.snapshot({}, TODAY)
        assert snap["total_return_pct"] == pytest.approx(0.0, abs=1e-9)

    def test_snapshot_positive_return(self):
        self.p.buy("VCB", 100.0, TODAY)
        # price rose → total value > initial
        snap = self.p.snapshot({"VCB": 120.0}, TODAY)
        assert snap["total_return_pct"] > 0

    def test_snapshot_negative_return(self):
        self.p.buy("VCB", 100.0, TODAY)
        # price fell → total value < initial
        snap = self.p.snapshot({"VCB": 80.0}, TODAY)
        assert snap["total_return_pct"] < 0

    def test_snapshot_open_positions_count(self):
        self.p.buy("VCB", 100.0, TODAY)
        self.p.buy("BID", 50.0, TODAY)
        snap = self.p.snapshot({"VCB": 100.0, "BID": 50.0}, TODAY)
        assert snap["open_positions"] == 2

    def test_snapshot_no_positions_open_positions_is_zero(self):
        snap = self.p.snapshot({}, TODAY)
        assert snap["open_positions"] == 0

    def test_snapshot_date_is_set(self):
        snap_date = date(2024, 3, 15)
        snap = self.p.snapshot({}, snap_date)
        assert snap["snapshot_date"] == snap_date

    def test_snapshot_cash_balance_correct(self):
        trade = self.p.buy("VCB", 100.0, TODAY)
        snap = self.p.snapshot({"VCB": 100.0}, TODAY)
        assert snap["cash_balance"] == pytest.approx(self.p.cash, rel=1e-9)

    def test_snapshot_positions_value_uses_current_prices(self):
        buy_trade = self.p.buy("VCB", 100.0, TODAY)
        current_price = 130.0
        snap = self.p.snapshot({"VCB": current_price}, TODAY)
        expected_pos_value = buy_trade.quantity * current_price
        assert snap["positions_value"] == pytest.approx(expected_pos_value, rel=1e-9)

    def test_snapshot_total_value_equals_cash_plus_positions(self):
        buy_trade = self.p.buy("VCB", 100.0, TODAY)
        current_price = 110.0
        snap = self.p.snapshot({"VCB": current_price}, TODAY)
        assert snap["total_value"] == pytest.approx(snap["cash_balance"] + snap["positions_value"], rel=1e-9)
