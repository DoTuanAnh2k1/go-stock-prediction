"""Unit tests for LSTMPredictor.

Skips gracefully if PyTorch is not installed.
No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random

import pytest

# Check PyTorch availability once at module level for skip markers
torch = pytest.importorskip("torch", reason="PyTorch not installed — skipping LSTM tests")

from src.algorithms.lstm import MIN_DATA_POINTS, LSTMPredictor

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 200, base: float = 60_000.0, seed: int = 99) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.015, 0.015)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestLSTMPredictor:

    def setup_method(self):
        self.algo = LSTMPredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "lstm_nn"

    def test_get_name(self):
        assert self.algo.get_name() == "LSTM Neural Network"

    def test_min_data_points_constant(self):
        """MIN_DATA_POINTS must be at least 70 (SEQUENCE_LENGTH + 10)."""
        assert MIN_DATA_POINTS >= 70

    def test_insufficient_data_raises(self):
        """Fewer than MIN_DATA_POINTS must raise ValueError."""
        with pytest.raises(ValueError, match="at least|needs"):
            self.algo.predict([100.0] * 30)

    def test_successful_prediction(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.algorithm_name == "lstm_nn"

    def test_current_price_matches_last_price(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        # PyTorch converts prices to float32 tensors, so allow float32 precision loss
        assert result.current_price == pytest.approx(prices[-1], rel=1e-4)

    def test_confidence_within_bounds(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert 0.0 <= result.confidence <= 1.0

    def test_predicted_price_within_7_percent(self):
        """Result must respect the ±7% daily clamping."""
        prices = make_prices(200)
        result = self.algo.predict(prices)
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_predicted_price_positive(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result.predicted_price > 0

    def test_fallback_on_bad_data(self):
        """Flat constant prices — model should still return (via fallback)."""
        prices = [50000.0] * 200
        # Either succeeds or falls back gracefully — must not raise
        result = self.algo.predict(prices)
        assert result is not None
        assert result.algorithm_name == "lstm_nn"

    def test_ema_fallback_returns_result(self):
        """_ema_fallback must always return a valid PredictionResult."""
        prices = make_prices(100)
        result = LSTMPredictor._ema_fallback(prices)
        assert result.algorithm_name == "lstm_nn"
        assert 0.0 <= result.confidence <= 1.0
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9
