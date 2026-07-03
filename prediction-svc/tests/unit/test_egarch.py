"""Unit tests for EGARCHPredictor.

Skips gracefully if statsmodels or arch is not installed.
No network, no DB, no Docker required — pure algorithm logic.
"""
from __future__ import annotations

import random
from unittest.mock import patch

import pytest

pytest.importorskip("statsmodels", reason="statsmodels not installed — skipping EGARCH tests")
pytest.importorskip("arch", reason="arch not installed — skipping EGARCH tests")

from src.algorithms.egarch import MIN_DATA_POINTS, EGARCHPredictor

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 200, base: float = 55_000.0, seed: int = 23) -> list[float]:
    rng = random.Random(seed)
    prices = [base]
    for _ in range(n - 1):
        change = rng.uniform(-0.015, 0.015)
        prices.append(max(1.0, prices[-1] * (1 + change)))
    return prices


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestEGARCHPredictor:

    def setup_method(self):
        self.algo = EGARCHPredictor()

    def test_get_key(self):
        assert self.algo.get_key() == "egarch"

    def test_get_name(self):
        assert self.algo.get_name() == "EGARCH"

    def test_min_data_points_constant(self):
        assert MIN_DATA_POINTS >= 60

    def test_insufficient_data_raises(self):
        with pytest.raises(ValueError, match="needs|at least"):
            self.algo.predict([100.0] * 30)

    def test_predict_returns_valid_price(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result is not None
        assert result.predicted_price > 0

    def test_current_price_matches_last_price(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-9)

    def test_predict_confidence_range(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        assert 0.0 <= result.confidence <= 1.0

    def test_predicted_price_within_7_percent(self):
        prices = make_prices(200)
        result = self.algo.predict(prices)
        current = prices[-1]
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_predict_insufficient_data_raises(self):
        with pytest.raises(ValueError):
            self.algo.predict([100.0] * (MIN_DATA_POINTS - 1))

    def test_predict_convergence_failure_raises(self):
        """Fail-loud policy: if _fit_and_predict raises, predict() re-raises
        (no fake EMA result) — see test_fail_loud_policy.py."""
        prices = make_prices(200)
        with patch.object(self.algo, "_fit_and_predict", side_effect=RuntimeError("convergence failed")):
            with pytest.raises(RuntimeError, match="convergence failed"):
                self.algo.predict(prices)

    def test_ema_fallback_returns_valid_result(self):
        prices = make_prices(100)
        current = prices[-1]
        result = EGARCHPredictor()._ema_fallback(prices, current)
        assert result.algorithm_name == "egarch"
        assert 0.0 <= result.confidence <= 1.0
        assert result.predicted_price >= current * 0.93 - 1e-9
        assert result.predicted_price <= current * 1.07 + 1e-9

    def test_fallback_confidence_fixed_at_036(self):
        prices = make_prices(100)
        current = prices[-1]
        result = EGARCHPredictor()._ema_fallback(prices, current)
        assert result.confidence == pytest.approx(0.36, abs=0.01)
