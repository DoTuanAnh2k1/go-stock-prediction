"""Unit tests for SignalGenerator signal classification logic.

Tests the pure classification logic without any DB access.
The helper function `classify_signal` mirrors the logic inside
SignalGenerator.get_signals() so we can test it in isolation.
"""
from __future__ import annotations

from datetime import date

import pytest


# ---------------------------------------------------------------------------
# Pure helper mirroring SignalGenerator.get_signals() logic
# ---------------------------------------------------------------------------

def classify_signal(
    ss: float,
    conf: float,
    buy_thr: float,
    sell_thr: float,
    min_conf: float,
) -> str:
    """Mirror of the classification logic in SignalGenerator.get_signals()."""
    if ss > buy_thr and conf > min_conf:
        return "BUY"
    elif ss < -sell_thr and conf > min_conf:
        return "SELL"
    return "HOLD"


def compute_signal_strength(predicted: float, current: float) -> float:
    """Mirror of signal_strength computation in SignalGenerator.get_signals()."""
    if current == 0:
        raise ZeroDivisionError("current price is 0")
    return (predicted - current) / current * 100


# ---------------------------------------------------------------------------
# Signal classification tests
# ---------------------------------------------------------------------------

class TestSignalClassification:

    # Default thresholds matching BotConfig defaults from seeder
    BUY_THR = 1.5
    SELL_THR = 1.0
    MIN_CONF = 0.60

    def test_buy_signal_above_threshold(self):
        action = classify_signal(2.0, 0.7, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "BUY"

    def test_sell_signal_below_threshold(self):
        action = classify_signal(-1.5, 0.7, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "SELL"

    def test_hold_signal_below_buy_threshold(self):
        action = classify_signal(1.0, 0.7, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_hold_signal_low_confidence(self):
        # signal_strength > buy threshold but confidence too low
        action = classify_signal(2.0, 0.5, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_hold_signal_near_zero(self):
        action = classify_signal(0.1, 0.8, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_sell_low_confidence_gives_hold(self):
        # Sell signal strength but confidence below min
        action = classify_signal(-2.0, 0.5, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_buy_exactly_at_threshold_gives_hold(self):
        # ss == buy_thr → not strictly greater → HOLD
        action = classify_signal(1.5, 0.8, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_sell_exactly_at_threshold_gives_hold(self):
        # ss == -sell_thr → not strictly less than -sell_thr → HOLD
        action = classify_signal(-1.0, 0.8, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_buy_confidence_exactly_at_min_gives_hold(self):
        # conf == min_conf → not strictly greater → HOLD
        action = classify_signal(2.0, 0.60, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_strong_buy_high_confidence(self):
        action = classify_signal(5.0, 0.95, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "BUY"

    def test_strong_sell_high_confidence(self):
        action = classify_signal(-3.0, 0.95, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "SELL"

    def test_zero_signal_gives_hold(self):
        action = classify_signal(0.0, 0.9, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_negative_confidence_gives_hold(self):
        action = classify_signal(2.0, -0.1, self.BUY_THR, self.SELL_THR, self.MIN_CONF)
        assert action == "HOLD"

    def test_custom_thresholds_buy(self):
        # Custom thresholds: buy_thr=3.0, min_conf=0.5
        action = classify_signal(3.5, 0.6, 3.0, 2.0, 0.5)
        assert action == "BUY"

    def test_custom_thresholds_sell(self):
        action = classify_signal(-2.5, 0.6, 3.0, 2.0, 0.5)
        assert action == "SELL"

    def test_custom_thresholds_hold_due_to_weak_signal(self):
        action = classify_signal(2.0, 0.9, 3.0, 2.0, 0.5)
        assert action == "HOLD"


# ---------------------------------------------------------------------------
# Signal strength computation tests
# ---------------------------------------------------------------------------

class TestSignalStrengthComputation:

    def test_signal_strength_positive(self):
        ss = compute_signal_strength(predicted=105.0, current=100.0)
        assert ss == pytest.approx(5.0, rel=1e-9)

    def test_signal_strength_negative(self):
        ss = compute_signal_strength(predicted=95.0, current=100.0)
        assert ss == pytest.approx(-5.0, rel=1e-9)

    def test_signal_strength_zero(self):
        ss = compute_signal_strength(predicted=100.0, current=100.0)
        assert ss == pytest.approx(0.0, abs=1e-9)

    def test_signal_strength_large_gain(self):
        ss = compute_signal_strength(predicted=200.0, current=100.0)
        assert ss == pytest.approx(100.0, rel=1e-9)

    def test_signal_strength_large_loss(self):
        ss = compute_signal_strength(predicted=50.0, current=100.0)
        assert ss == pytest.approx(-50.0, rel=1e-9)

    def test_signal_strength_fractional(self):
        ss = compute_signal_strength(predicted=100.5, current=100.0)
        assert ss == pytest.approx(0.5, rel=1e-9)

    def test_signal_strength_zero_current_raises(self):
        with pytest.raises(ZeroDivisionError):
            compute_signal_strength(predicted=100.0, current=0.0)

    def test_signal_strength_large_price(self):
        # High absolute price (e.g. gold) — relative change still works
        ss = compute_signal_strength(predicted=2020.0, current=2000.0)
        assert ss == pytest.approx(1.0, rel=1e-9)


# ---------------------------------------------------------------------------
# TradeSignal dataclass import test (structural check)
# ---------------------------------------------------------------------------

class TestTradeSignalImport:

    def test_trade_signal_can_be_imported(self):
        from src.simulation.signal import TradeSignal
        assert TradeSignal is not None

    def test_trade_signal_dataclass_fields(self):
        from src.simulation.signal import TradeSignal
        sig = TradeSignal(
            symbol="VCB",
            action="BUY",
            signal_strength=2.5,
            confidence=0.75,
            predicted_price=105.0,
            current_price=100.0,
            prediction_date=date(2024, 1, 15),
        )
        assert sig.symbol == "VCB"
        assert sig.action == "BUY"
        assert sig.signal_strength == pytest.approx(2.5)
        assert sig.confidence == pytest.approx(0.75)
        assert sig.predicted_price == pytest.approx(105.0)
        assert sig.current_price == pytest.approx(100.0)
        assert sig.prediction_date == date(2024, 1, 15)

    def test_signal_generator_can_be_instantiated(self):
        from src.simulation.signal import SignalGenerator
        sg = SignalGenerator()
        assert sg is not None

    def test_signal_generator_has_market_to_table(self):
        from src.simulation.signal import SignalGenerator
        sg = SignalGenerator()
        assert "VN30" in sg.MARKET_TO_TABLE
        assert "GOLD" in sg.MARKET_TO_TABLE
        assert "NASDAQ" in sg.MARKET_TO_TABLE
        assert "SP500" in sg.MARKET_TO_TABLE
        assert "CRYPTO" in sg.MARKET_TO_TABLE
