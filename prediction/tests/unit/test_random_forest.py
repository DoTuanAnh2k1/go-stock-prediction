"""Unit tests for RandomForestPredictor.

No importorskip needed — scikit-learn is a base dependency.
No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random

import numpy as np
import pytest

from src.algorithms.random_forest import MIN_DATA_POINTS, RandomForestPredictor

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 250, base: float = 55_000.0, seed: int = 42) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.02, 0.02)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


def make_volumes(n: int = 250, seed: int = 7) -> list[float]:
    rng = random.Random(seed)
    return [rng.uniform(1_000, 500_000) for _ in range(n)]


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestRandomForestPredictor:

    def setup_method(self):
        self.algo = RandomForestPredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "random_forest"

    def test_get_name(self):
        assert self.algo.get_name() == "Random Forest"

    def test_min_data_points_constant(self):
        assert MIN_DATA_POINTS >= 80

    def test_insufficient_data_raises(self):
        with pytest.raises(ValueError, match="needs|at least"):
            self.algo.predict([100.0] * 50)

    def test_predict_returns_valid_price(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.predicted_price > 0

    def test_current_price_matches_last_price(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-9)

    def test_predict_confidence_range(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert 0.0 <= result.confidence <= 1.0

    def test_predicted_price_within_7_percent(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_with_volumes(self):
        prices = make_prices(250)
        volumes = make_volumes(250)
        result = self.algo.predict(prices, volumes)
        assert result is not None
        assert 0.0 <= result.confidence <= 1.0

    def test_predict_insufficient_data_fallback_not_raised(self):
        """Just under MIN_DATA_POINTS raises ValueError (not a silent fallback)."""
        with pytest.raises(ValueError):
            self.algo.predict([100.0] * (MIN_DATA_POINTS - 1))

    def test_build_features_shape(self):
        arr = np.array(make_prices(100), dtype=float)
        features, targets = RandomForestPredictor._build_features(arr, None)
        assert len(features) > 0
        assert len(features[0]) == 14
        assert len(features) == len(targets)

    def test_ema_fallback_returns_valid_result(self):
        prices = make_prices(100)
        current = prices[-1]
        result = RandomForestPredictor._ema_fallback(prices, current)
        assert result.algorithm_name == "random_forest"
        assert 0.0 <= result.confidence <= 1.0
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_fallback_confidence_fixed_at_035(self):
        prices = make_prices(100)
        current = prices[-1]
        result = RandomForestPredictor._ema_fallback(prices, current)
        assert result.confidence == pytest.approx(0.35, abs=0.01)
