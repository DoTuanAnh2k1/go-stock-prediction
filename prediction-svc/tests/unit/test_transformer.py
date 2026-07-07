"""Unit tests for TransformerPredictor (algorithm #13 — PatchTST-lite).

Skips gracefully if PyTorch is not installed.
No network, no DB, no Docker required — fundamentals lookups are monkeypatched.
"""
from __future__ import annotations

import math
import os

import numpy as np
import pytest

torch = pytest.importorskip("torch", reason="PyTorch not installed — skipping transformer tests")

from src.algorithms.base import PredictionResult, get_max_change_pct
from src.algorithms.transformer_model import (
    FUND_DIM,
    MIN_DATA_POINTS_TF,
    SEQ_LEN,
    TransformerPredictor,
    _build_model,
    _direction_targets,
    _normalize_symbol,
    _raw_fund_vector,
    _tf_checkpoint_path,
)
from src.crawlers.fundamentals import _extract_fields


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 250, base: float = 100.0, seed: int = 7, drift: float = 0.0) -> list[float]:
    rng = np.random.default_rng(seed)
    rets = [0.0]
    for _ in range(n - 1):
        rets.append(0.4 * rets[-1] + drift + rng.normal(0.0, 0.01))
    return list(base * np.exp(np.cumsum(rets)))


@pytest.fixture(autouse=True)
def _no_db_fundamentals(monkeypatch, tmp_path):
    """Isolate from DB and real checkpoint dir."""
    monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
    monkeypatch.setattr(
        "src.algorithms.transformer_model._get_fundamentals_for_market",
        lambda market: {},
    )


@pytest.fixture(autouse=True)
def _fast_train(monkeypatch):
    monkeypatch.setattr("src.algorithms.transformer_model.EPOCHS", 5)
    monkeypatch.setattr("src.algorithms.transformer_model.QUICK_EPOCHS", 3)


def _fresh(market="NASDAQ100") -> TransformerPredictor:
    p = TransformerPredictor()
    p._market_key = market
    return p


# ---------------------------------------------------------------------------
# 1. Identity
# ---------------------------------------------------------------------------

class TestIdentity:
    def test_key(self):
        assert TransformerPredictor().get_key() == "transformer_nn"

    def test_name(self):
        assert "Transformer" in TransformerPredictor().get_name()

    def test_untrained_initially(self):
        assert TransformerPredictor().is_trained() is False


# ---------------------------------------------------------------------------
# 2. Input validation
# ---------------------------------------------------------------------------

class TestInputValidation:
    def test_too_few_prices_raises(self):
        p = _fresh()
        with pytest.raises(ValueError, match="at least"):
            p.predict([100.0] * (MIN_DATA_POINTS_TF - 1))


# ---------------------------------------------------------------------------
# 3. Model shapes
# ---------------------------------------------------------------------------

class TestModelShapes:
    def test_forward_shape_dual_default(self):
        m = _build_model()
        x = torch.randn(4, SEQ_LEN)
        f = torch.randn(4, FUND_DIM)
        ret, logit = m(x, f)
        assert ret.shape == (4,)
        assert logit.shape == (4,)

    def test_forward_shape_single_legacy(self):
        from src.algorithms.transformer_model import HEAD_SINGLE
        m = _build_model(head=HEAD_SINGLE)
        out = m(torch.randn(2, SEQ_LEN), torch.zeros(2, FUND_DIM))
        assert not isinstance(out, tuple)
        assert out.shape == (2,)

    def test_batch_one(self):
        m = _build_model()
        ret, logit = m(torch.randn(1, SEQ_LEN), torch.zeros(1, FUND_DIM))
        assert ret.shape == (1,) and logit.shape == (1,)


# ---------------------------------------------------------------------------
# 4. Training + checkpoint
# ---------------------------------------------------------------------------

class TestTraining:
    def test_train_batch_labeled_flips_trained_and_saves(self, tmp_path):
        p = _fresh()
        series = [(make_prices(250, seed=s), None, f"nasdaq/SYM{s}") for s in range(4)]
        p.train_batch_labeled(series)
        assert p.is_trained() is True
        assert os.path.exists(_tf_checkpoint_path("NASDAQ100"))

    def test_train_batch_unlabeled_delegates(self):
        p = _fresh("CRYPTO")
        p.train_batch([(make_prices(250, seed=s), None) for s in range(3)])
        assert p.is_trained() is True

    def test_short_series_skipped(self):
        p = _fresh()
        p.train_batch_labeled([([100.0] * 50, None, "nasdaq/X")])
        assert p.is_trained() is False

    def test_checkpoint_roundtrip_same_prediction(self):
        trainer = _fresh()
        prices = make_prices(250, seed=42)
        trainer.train_batch_labeled([(make_prices(250, seed=s), None, None) for s in range(4)])
        r1 = trainer.predict(prices)

        loader = _fresh()
        r2 = loader.predict(prices)   # lazy-loads checkpoint
        assert loader.is_trained() is True
        assert r2.predicted_price == pytest.approx(r1.predicted_price, rel=1e-6)


# ---------------------------------------------------------------------------
# 5. Prediction semantics
# ---------------------------------------------------------------------------

class TestPrediction:
    def _trained(self) -> TransformerPredictor:
        p = _fresh()
        p.train_batch_labeled([(make_prices(250, seed=s), None, None) for s in range(4)])
        return p

    def test_result_fields(self):
        p = self._trained()
        prices = make_prices(250, seed=9)
        r = p.predict(prices)
        assert isinstance(r, PredictionResult)
        assert r.algorithm_name == "transformer_nn"
        assert r.current_price == pytest.approx(prices[-1], rel=1e-6)
        assert math.isfinite(r.predicted_price) and r.predicted_price > 0
        assert 0.0 <= r.confidence <= 1.0

    @pytest.mark.parametrize("market,max_pct", [
        ("GOLD", 0.15), ("NASDAQ100", 0.20), ("SP500", 0.15), ("CRYPTO", 0.50),
    ])
    def test_market_clamp(self, market, max_pct):
        p = _fresh(market)
        p.train_batch_labeled([(make_prices(250, seed=s), None, None) for s in range(4)])
        prices = make_prices(250, seed=11)
        r = p.predict(prices)
        current = prices[-1]
        assert current * (1 - max_pct) - 1e-9 <= r.predicted_price <= current * (1 + max_pct) + 1e-9

    def test_cold_start_quick_train(self):
        """No checkpoint → quick single-series train, low fixed confidence."""
        p = _fresh("SP500")
        r = p.predict(make_prices(250, seed=3))
        assert isinstance(r, PredictionResult)
        assert r.confidence == pytest.approx(0.45)


# ---------------------------------------------------------------------------
# 5b. Dual head — direction from P(up), selective write, legacy compat
# ---------------------------------------------------------------------------

class _FakeDualModel(torch.nn.Module):
    """Deterministic dual-head stand-in: fixed return and direction logit."""

    def __init__(self, ret: float, logit: float) -> None:
        super().__init__()
        self._ret = ret
        self._logit = logit

    def forward(self, x, fund):
        b = x.shape[0]
        return (torch.full((b,), self._ret), torch.full((b,), self._logit))


class TestDualHeadSemantics:
    def _rigged(self, ret: float, logit: float) -> TransformerPredictor:
        p = _fresh("NASDAQ100")
        p._model = _FakeDualModel(ret, logit)
        p._trained = True
        p._load_attempted = True
        p._r_mean, p._r_std = 0.0, 0.01
        return p

    def test_coin_flip_sets_skip_write(self):
        p = self._rigged(ret=1.0, logit=0.0)   # P(up) = 0.5 exactly
        r = p.predict(make_prices(250, seed=5))
        assert r.skip_write is True

    def test_confident_up_not_skipped_and_predicts_up(self):
        p = self._rigged(ret=-1.0, logit=3.0)  # P(up) ≈ 0.95 overrides ret sign
        r = p.predict(make_prices(250, seed=5))
        assert r.skip_write is False
        assert r.predicted_price > r.current_price
        assert r.confidence > 0.7

    def test_confident_down_predicts_down(self):
        p = self._rigged(ret=1.0, logit=-3.0)  # P(up) ≈ 0.05 → down
        r = p.predict(make_prices(250, seed=5))
        assert r.predicted_price < r.current_price

    def test_legacy_single_head_checkpoint_loads_and_predicts(self, tmp_path):
        """Pre-dual checkpoints (config without 'head') load as single-head."""
        from src.algorithms.transformer_model import HEAD_SINGLE
        legacy = _build_model(head=HEAD_SINGLE)
        torch.save({
            "state_dict": legacy.state_dict(),
            "config": {"seq_len": SEQ_LEN, "patch_len": 8, "d_model": 64,
                       "n_heads": 4, "n_layers": 2, "ff_dim": 128,
                       "fund_dim": FUND_DIM, "dropout": 0.1},
            "r_mean": 0.0, "r_std": 0.01,
            "fund_mean": np.zeros(FUND_DIM), "fund_std": np.ones(FUND_DIM),
        }, _tf_checkpoint_path("NASDAQ100"))

        p = _fresh("NASDAQ100")
        r = p.predict(make_prices(250, seed=6))
        assert p.is_trained() is True
        assert math.isfinite(r.predicted_price)
        assert r.skip_write is False   # legacy head never sets skip


# ---------------------------------------------------------------------------
# 6. Fundamentals plumbing
# ---------------------------------------------------------------------------

class TestFundamentals:
    def test_normalize_symbol(self):
        assert _normalize_symbol("nasdaq/AAPL") == "AAPL"
        assert _normalize_symbol("sp500/JPM") == "JPM"
        assert _normalize_symbol("AAPL") == "AAPL"
        assert _normalize_symbol(None) is None
        assert _normalize_symbol("gold/XAU/spot") == "spot"

    def test_raw_fund_vector_missing_is_nan(self):
        vec = _raw_fund_vector(None)
        assert vec.shape == (FUND_DIM,)
        assert np.all(np.isnan(vec))

    def test_raw_fund_vector_market_cap_log_scaled(self):
        vec = _raw_fund_vector({"market_cap": 1e12, "pe_ratio": 30.0})
        fields_idx = {"pe_ratio": 0, "market_cap": 10}
        assert vec[fields_idx["market_cap"]] == pytest.approx(12.0)
        assert vec[fields_idx["pe_ratio"]] == pytest.approx(30.0)

    def test_fund_vector_used_in_training(self, monkeypatch):
        """Symbols with fundamentals train without error and produce a model."""
        fake_map = {
            f"SYM{s}": {"pe_ratio": 20.0 + s, "market_cap": 1e11 * (s + 1), "beta": 1.0}
            for s in range(4)
        }
        monkeypatch.setattr(
            "src.algorithms.transformer_model._get_fundamentals_for_market",
            lambda market: fake_map,
        )
        p = _fresh()
        p.train_batch_labeled([(make_prices(250, seed=s), None, f"nasdaq/SYM{s}") for s in range(4)])
        assert p.is_trained() is True
        r = p.predict(make_prices(250, seed=1))
        assert math.isfinite(r.predicted_price)


# ---------------------------------------------------------------------------
# 7. Fundamentals crawler field extraction (no network)
# ---------------------------------------------------------------------------

class TestCrawlerExtraction:
    def test_extract_maps_yfinance_keys(self):
        info = {
            "trailingPE": 35.5, "forwardPE": 30.6, "priceToBook": 40.5,
            "trailingEps": 8.27, "revenueGrowth": 0.166, "earningsGrowth": 0.218,
            "profitMargins": 0.27, "debtToEquity": 79.5, "dividendYield": 0.37,
            "beta": 1.086, "marketCap": 4.3e12,
        }
        out = _extract_fields(info)
        assert out["pe_ratio"] == pytest.approx(35.5)
        assert out["market_cap"] == pytest.approx(4.3e12)
        assert out["profit_margin"] == pytest.approx(0.27)

    def test_extract_handles_missing_and_garbage(self):
        out = _extract_fields({"trailingPE": "Infinity", "beta": None, "marketCap": float("nan")})
        assert out["pe_ratio"] is None
        assert out["beta"] is None
        assert out["market_cap"] is None
        # All mapped keys present even when absent from info
        assert set(out.keys()) == {
            "pe_ratio", "forward_pe", "price_to_book", "eps_ttm",
            "revenue_growth", "earnings_growth", "profit_margin",
            "debt_to_equity", "dividend_yield", "beta", "market_cap",
        }


# ---------------------------------------------------------------------------
# Direction-label threshold (regression guard for the down-bias bug)
# ---------------------------------------------------------------------------

def test_direction_targets_use_raw_return_sign_not_mean():
    """The direction head must be labelled on raw-return sign (ret > 0), which
    the system scores, NOT on standardized sign (ret > mean). In an up-drifting
    market r_mean > 0, so a small POSITIVE return that is still below the mean
    must label UP (1), not DOWN — the old (y_std > 0) threshold got this wrong.
    """
    r_mean, r_std = 0.001, 0.01  # positive drift
    # raw returns: +0.0005 (positive but below mean), -0.0005 (negative)
    raw = np.array([0.0005, -0.0005, 0.0, 0.002])
    y_std = (raw - r_mean) / r_std

    labels = np.asarray(_direction_targets(y_std, r_mean, r_std))

    # label must equal raw-return-positive, element by element
    assert labels[0] == True   # +0.0005 > 0  → UP  (old code wrongly said DOWN)
    assert labels[1] == False  # -0.0005 < 0  → DOWN
    assert labels[2] == False  # exactly 0    → not > 0
    assert labels[3] == True   # +0.002 > 0   → UP
    # exactly equivalent to thresholding raw returns at zero
    assert np.array_equal(labels, raw > 0)


def test_direction_targets_zero_mean_reduces_to_zero_threshold():
    """With no drift (r_mean = 0) the correct threshold is plain sign(ret)."""
    r_mean, r_std = 0.0, 0.02
    y_std = np.array([-1.0, 0.0, 1.0])
    labels = np.asarray(_direction_targets(y_std, r_mean, r_std))
    assert np.array_equal(labels, np.array([False, False, True]))
