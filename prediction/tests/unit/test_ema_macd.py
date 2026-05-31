"""Unit tests for EMAMACDPredictor (MACD algorithm).

No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random

import numpy as np
import pytest

from src.algorithms.ema_macd import EMAMACDPredictor


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 200, base: float = 80_000.0, seed: int = 11) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.025, 0.025)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestEMAMACDPredictor:

    def setup_method(self):
        self.algo = EMAMACDPredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "ema"

    def test_get_name(self):
        assert self.algo.get_name() == "Exponential Moving Average"

    def test_successful_prediction(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.algorithm_name == "ema"

    def test_current_price_matches_last_price(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-9)

    def test_confidence_within_bounds(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert 0.0 <= result.confidence <= 1.0

    def test_predicted_price_within_7_percent(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_predicted_price_positive(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result.predicted_price > 0

    def test_insufficient_data_raises(self):
        """Fewer than LONG_PERIOD (26) prices must raise ValueError."""
        with pytest.raises(ValueError, match="at least"):
            self.algo.predict([100.0] * 20)

    def test_exactly_minimum_data(self):
        """Exactly 26 prices must succeed."""
        prices = [float(i + 100) for i in range(26)]
        result = self.algo.predict(prices)
        assert result is not None

    def test_ema_scalar_returns_float(self):
        arr = np.array(make_prices(50), dtype=float)
        val = EMAMACDPredictor._ema_scalar(arr, 12)
        assert isinstance(val, float)
        assert val > 0

    def test_ema_scalar_insufficient_returns_zero(self):
        arr = np.array([100.0, 200.0], dtype=float)
        val = EMAMACDPredictor._ema_scalar(arr, 12)
        assert val == 0.0

    def test_ema_series_length(self):
        arr = np.array(make_prices(50), dtype=float)
        series = EMAMACDPredictor._ema_series(arr, 12)
        assert len(series) == 50 - 12 + 1

    def test_ema_series_insufficient_returns_empty(self):
        arr = np.array([100.0, 200.0], dtype=float)
        series = EMAMACDPredictor._ema_series(arr, 12)
        assert len(series) == 0

    def test_macd_signal_buy(self):
        sig = EMAMACDPredictor._macd_signal(1.0, 0.5, 0.5)
        assert sig["direction"] == "BUY"

    def test_macd_signal_sell(self):
        sig = EMAMACDPredictor._macd_signal(-1.0, -0.5, -0.5)
        assert sig["direction"] == "SELL"

    def test_macd_signal_hold(self):
        # HOLD when macd > signal but histogram <= 0, OR macd < signal but histogram >= 0
        # macd_line=1.5, signal_line=1.0 → macd > signal, but histogram=-0.5 → NOT (both positive) → HOLD
        sig = EMAMACDPredictor._macd_signal(1.5, 1.0, -0.5)
        assert sig["direction"] == "HOLD"

    def test_multiple_predictions_stable(self):
        prices = make_prices(300)
        current = prices[-1]
        for _ in range(5):
            result = self.algo.predict(prices)
            assert current * 0.93 - 1e-9 <= result.predicted_price <= current * 1.07 + 1e-9
