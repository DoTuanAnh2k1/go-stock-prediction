"""Unit tests for ARIMAGARCHPredictor.

Skips gracefully if statsmodels is not installed.
No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random

import pytest

statsmodels = pytest.importorskip(
    "statsmodels", reason="statsmodels not installed — skipping ARIMA-GARCH tests"
)

from src.algorithms.arima_garch import MIN_DATA_POINTS, ARIMAGARCHPredictor

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 200, base: float = 45_000.0, seed: int = 77) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.018, 0.018)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestARIMAGARCHPredictor:

    def setup_method(self):
        self.algo = ARIMAGARCHPredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "arima_garch"

    def test_get_name(self):
        assert self.algo.get_name() == "ARIMA-GARCH"

    def test_min_data_points_constant(self):
        """MIN_DATA_POINTS must be at least 50."""
        assert MIN_DATA_POINTS >= 50

    def test_insufficient_data_raises(self):
        """Fewer than MIN_DATA_POINTS (50) must raise ValueError."""
        with pytest.raises(ValueError, match="needs|at least"):
            self.algo.predict([100.0] * 30)

    def test_successful_prediction(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.algorithm_name == "arima_garch"

    def test_current_price_matches_last_price(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-9)

    def test_confidence_within_bounds(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert 0.0 <= result.confidence <= 1.0

    def test_predicted_price_within_7_percent(self):
        """ARIMA forecast must be clamped to ±7% daily limit."""
        prices = make_prices(200)
        result = self.algo.predict(prices)
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_predicted_price_positive(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result.predicted_price > 0

    def test_exactly_minimum_data(self):
        """Exactly MIN_DATA_POINTS prices must not raise."""
        prices = make_prices(MIN_DATA_POINTS, seed=55)
        result = self.algo.predict(prices)
        assert result is not None

    def test_ema_fallback_returns_valid_result(self):
        """_ema_fallback must always return a valid PredictionResult."""
        prices = make_prices(100)
        current = prices[-1]
        result = ARIMAGARCHPredictor()._ema_fallback(prices, current)
        assert result.algorithm_name == "arima_garch"
        assert 0.0 <= result.confidence <= 1.0
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_fallback_confidence_fixed_at_038(self):
        """EMA fallback confidence should be 0.38."""
        prices = make_prices(100)
        current = prices[-1]
        result = ARIMAGARCHPredictor()._ema_fallback(prices, current)
        assert result.confidence == pytest.approx(0.38, abs=0.01)
