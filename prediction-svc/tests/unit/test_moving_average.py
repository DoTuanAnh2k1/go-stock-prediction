"""Unit tests for MovingAveragePredictor (VWMA).

No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random

import numpy as np
import pytest

from src.algorithms.moving_average import MovingAveragePredictor

# ---------------------------------------------------------------------------
# Fixtures / helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 200, base: float = 50_000.0, seed: int = 42) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.02, 0.02)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


def make_volumes(n: int = 200, seed: int = 7) -> list[float]:
    rng = random.Random(seed)
    return [rng.uniform(1_000, 1_000_000) for _ in range(n)]


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestMovingAveragePredictor:

    def setup_method(self):
        self.algo = MovingAveragePredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "moving_average"

    def test_get_name(self):
        assert self.algo.get_name() == "Volume-Weighted Moving Average"

    def test_successful_prediction_returns_result(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.algorithm_name == "moving_average"

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

    def test_with_volumes(self):
        prices = make_prices(200)
        volumes = make_volumes(200)
        result = self.algo.predict(prices, volumes)
        assert result is not None
        assert 0.0 <= result.confidence <= 1.0

    def test_insufficient_data_raises(self):
        """Fewer than LONG_PERIOD (20) prices must raise ValueError."""
        with pytest.raises(ValueError, match="at least"):
            self.algo.predict([100.0] * 10)

    def test_exactly_minimum_data(self):
        """Exactly LONG_PERIOD prices must succeed."""
        prices = [float(i + 100) for i in range(20)]
        result = self.algo.predict(prices)
        assert result is not None

    def test_calc_rsi_returns_between_0_100(self):
        arr = np.array(make_prices(100), dtype=float)
        rsi = MovingAveragePredictor.calc_rsi(arr)
        assert 0.0 <= rsi <= 100.0

    def test_calc_rsi_insufficient_data_returns_50(self):
        arr = np.array([100.0] * 5, dtype=float)
        rsi = MovingAveragePredictor.calc_rsi(arr, period=14)
        assert rsi == 50.0

    def test_calc_bollinger_returns_three_bands(self):
        arr = np.array(make_prices(100), dtype=float)
        upper, mid, lower = MovingAveragePredictor.calc_bollinger(arr)
        assert upper >= mid >= lower

    def test_multiple_predictions_are_stable(self):
        """Run 5 predictions on same data — all should be within 7% of current."""
        prices = make_prices(300)
        current = prices[-1]
        for _ in range(5):
            result = self.algo.predict(prices)
            assert current * 0.93 - 1e-9 <= result.predicted_price <= current * 1.07 + 1e-9
