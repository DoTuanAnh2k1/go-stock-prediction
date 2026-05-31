"""Unit tests for LightGBMPredictor.

Skips gracefully if lightgbm is not installed.
No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random

import numpy as np
import pytest

lgb = pytest.importorskip("lightgbm", reason="lightgbm not installed — skipping LightGBM tests")

from src.algorithms.lightgbm_model import LightGBMPredictor, MIN_DATA_POINTS


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 250, base: float = 55_000.0, seed: int = 33) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.02, 0.02)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


def make_volumes(n: int = 250, seed: int = 55) -> list[float]:
    rng = random.Random(seed)
    return [rng.uniform(1_000, 500_000) for _ in range(n)]


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestLightGBMPredictor:

    def setup_method(self):
        self.algo = LightGBMPredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "lightgbm"

    def test_get_name(self):
        assert self.algo.get_name() == "LightGBM"

    def test_min_data_points_constant(self):
        """MIN_DATA_POINTS must be at least 80."""
        assert MIN_DATA_POINTS >= 80

    def test_insufficient_data_raises(self):
        """Fewer than MIN_DATA_POINTS (80) must raise ValueError."""
        with pytest.raises(ValueError, match="needs|at least"):
            self.algo.predict([100.0] * 50)

    def test_successful_prediction(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.algorithm_name == "lightgbm"

    def test_current_price_matches_last_price(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-9)

    def test_confidence_within_bounds(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert 0.0 <= result.confidence <= 1.0

    def test_predicted_price_within_7_percent(self):
        """LightGBM forecast must be clamped to ±7% daily limit."""
        prices = make_prices(250)
        result = self.algo.predict(prices)
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_predicted_price_positive(self):
        prices = make_prices(250)
        result = self.algo.predict(prices)
        assert result.predicted_price > 0

    def test_with_volumes(self):
        prices = make_prices(250)
        volumes = make_volumes(250)
        result = self.algo.predict(prices, volumes)
        assert result is not None
        assert 0.0 <= result.confidence <= 1.0

    def test_build_features_returns_correct_shape(self):
        arr = np.array(make_prices(100), dtype=float)
        features, targets = LightGBMPredictor._build_features(arr, None)
        # Should have: 14 features per row (10 lags + RSI + MA5 + MA20 + vol_ratio)
        assert len(features) > 0
        assert len(features[0]) == 14
        assert len(features) == len(targets)

    def test_build_features_with_volumes(self):
        arr = np.array(make_prices(100), dtype=float)
        vol_arr = np.array(make_volumes(100), dtype=float)
        features, targets = LightGBMPredictor._build_features(arr, vol_arr)
        assert len(features) > 0
        assert len(features[0]) == 14

    def test_ema_fallback_returns_valid_result(self):
        """_ema_fallback must always return a valid PredictionResult."""
        prices = make_prices(100)
        current = prices[-1]
        result = LightGBMPredictor._ema_fallback(prices, current)
        assert result.algorithm_name == "lightgbm"
        assert 0.0 <= result.confidence <= 1.0
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_fallback_confidence_fixed_at_035(self):
        """EMA fallback confidence should be 0.35."""
        prices = make_prices(100)
        current = prices[-1]
        result = LightGBMPredictor._ema_fallback(prices, current)
        assert result.confidence == pytest.approx(0.35, abs=0.01)
