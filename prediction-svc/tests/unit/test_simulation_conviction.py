"""Unit tests for the transformer_nn conviction branch (TradingBot._step_conviction).

No DB, no torch — the model and data fetchers are faked.
"""
from __future__ import annotations

from datetime import date, datetime

import pytest

from src.simulation.bot import (
    _CONVICTION_BUY_FLOOR,
    _CONVICTION_SELL_CEIL,
    BotConfig,
    TradingBot,
)


class _FakeModel:
    """Stands in for TransformerPredictor: fixed P(up)."""

    def __init__(self, p_up):
        self._p_up = p_up
        self._context_symbol = None

    def predict_direction_proba(self, prices):
        return self._p_up


def _make_bot(p_up, monkeypatch, buy_threshold=0.5, sell_threshold=0.3,
              symbol=None) -> TradingBot:
    config = BotConfig(
        bot_id="nasdaq_transformer_nn",
        market="NASDAQ",
        algorithm="transformer_nn" if symbol is None else "transformer_nn__ps",
        initial_capital=1000.0,
        buy_threshold=buy_threshold,
        sell_threshold=sell_threshold,
        min_confidence=0.40,
        stop_loss=5.0,
        take_profit=8.0,
        max_position_pct=15.0,
        max_positions=5,
        symbol=symbol,
    )
    bot = TradingBot(config)

    fake = _FakeModel(p_up)
    monkeypatch.setattr(
        "src.algorithms.registry.get_algos_for_market",
        lambda mk: {"transformer_nn": fake},
    )
    monkeypatch.setattr(
        "src.algorithms.registry.get_algos_for_symbol",
        lambda mk, sym: {"transformer_nn": fake},
    )
    monkeypatch.setattr(
        bot, "_get_symbols_and_prices_intraday_as_of",
        lambda now: {"AAPL": 100.0},
    )
    monkeypatch.setattr(
        bot, "_get_price_history_intraday_as_of",
        lambda sym, as_of, limit=400: [100.0 + 0.01 * i for i in range(150)],
    )
    return bot


NOW = datetime(2026, 7, 2, 10, 0, 0)
TODAY = date(2026, 7, 2)


class TestConvictionEntries:
    def test_confident_up_buys(self, monkeypatch):
        bot = _make_bot(0.60, monkeypatch)
        trades = bot._step_conviction(TODAY, set(), now=NOW)
        assert len(trades) == 1
        assert trades[0].action == "BUY"
        assert trades[0].symbol == "AAPL"
        assert trades[0].confidence == pytest.approx(0.45)

    def test_below_absolute_floor_holds(self, monkeypatch):
        # Default delta 0.005 → threshold = max(0.505, 0.52) = floor 0.52
        bot = _make_bot(0.515, monkeypatch)
        trades = bot._step_conviction(TODAY, set(), now=NOW)
        assert trades == []

    def test_exactly_at_floor_buys(self, monkeypatch):
        bot = _make_bot(_CONVICTION_BUY_FLOOR, monkeypatch)
        trades = bot._step_conviction(TODAY, set(), now=NOW)
        assert len(trades) == 1 and trades[0].action == "BUY"

    def test_variant_threshold_raises_floor(self, monkeypatch):
        # buy_threshold=3.0 → threshold = max(0.53, 0.52) = 0.53
        bot = _make_bot(0.525, monkeypatch, buy_threshold=3.0)
        assert bot._step_conviction(TODAY, set(), now=NOW) == []
        bot2 = _make_bot(0.54, monkeypatch, buy_threshold=3.0)
        assert len(bot2._step_conviction(TODAY, set(), now=NOW)) == 1

    def test_none_proba_holds(self, monkeypatch):
        bot = _make_bot(None, monkeypatch)
        assert bot._step_conviction(TODAY, set(), now=NOW) == []

    def test_closed_this_step_blocks_reentry(self, monkeypatch):
        bot = _make_bot(0.60, monkeypatch)
        assert bot._step_conviction(TODAY, {"AAPL"}, now=NOW) == []


class TestConvictionExits:
    def _holding_bot(self, p_up, monkeypatch):
        bot = _make_bot(p_up, monkeypatch)
        bot.portfolio.buy(symbol="AAPL", price=100.0, trade_date=TODAY,
                          signal_strength=1.0, confidence=0.5, trade_at=NOW)
        return bot

    def test_confident_down_sells_holding(self, monkeypatch):
        bot = self._holding_bot(0.40, monkeypatch)
        trades = bot._step_conviction(TODAY, set(), now=NOW)
        assert len(trades) == 1
        assert trades[0].action == "SELL"
        assert trades[0].close_reason == "conviction_signal"

    def test_down_but_not_holding_does_nothing(self, monkeypatch):
        bot = _make_bot(0.40, monkeypatch)
        assert bot._step_conviction(TODAY, set(), now=NOW) == []

    def test_mild_down_above_ceiling_holds_position(self, monkeypatch):
        # p=0.49 > ceiling 0.48 (default delta_sell 0.003 → min(0.497, 0.48)=0.48)
        bot = self._holding_bot(0.49, monkeypatch)
        assert bot._step_conviction(TODAY, set(), now=NOW) == []
        assert "AAPL" in bot.portfolio.positions

    def test_ceiling_value(self):
        assert _CONVICTION_SELL_CEIL == pytest.approx(0.48)
        assert _CONVICTION_BUY_FLOOR == pytest.approx(0.52)


class TestStepRouting:
    def test_step_routes_transformer_to_conviction(self, monkeypatch):
        bot = _make_bot(0.60, monkeypatch)
        called = {}

        def fake_conv(sim_date, closed, now=None):
            called["yes"] = True
            return []

        monkeypatch.setattr(bot, "_step_conviction", fake_conv)
        bot.step(TODAY, cache=None, now=NOW)
        assert called.get("yes") is True
