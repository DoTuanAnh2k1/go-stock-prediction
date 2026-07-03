"""Unit tests for RLDQNPredictor.

Skips gracefully if PyTorch is not installed.
No network, no DB, no Docker required — pure algorithm logic.

Coverage targets:
  - Identity methods (get_key, get_name)
  - predict() with synthetic price series
  - Market-aware clamp correctness across markets
  - Safe fallback when no checkpoint exists (EMA path)
  - act() returns valid action in {0, 1, 2}
  - is_trained() False before training, True after train_batch()
  - Action → price direction mapping via sigma-scaling (_inference path)
  - train_batch() creates checkpoint + flips is_trained (monkeypatched dir)
"""
from __future__ import annotations

import math
import os

import numpy as np
import pytest

# Check PyTorch availability once at module level — skip entire file if missing.
torch = pytest.importorskip("torch", reason="PyTorch not installed — skipping RL DQN tests")

from src.algorithms.base import MARKET_MAX_CHANGE, PredictionResult, get_max_change_pct
from src.algorithms.rl_dqn import (
    ARCH_DUELING,
    ARCH_MLP,
    GAMMA,
    MIN_REPLAY,
    N_ACTIONS,
    N_ENHANCED_FEATURES,
    N_POS_FEATURES,
    N_STEP,
    RLDQNPredictor,
    _ReplayBuffer,
    _build_qnet,
    _checkpoint_path,
    _make_windows,
    _run_episode,
    _scale_feature_matrix,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def make_prices(n: int = 220, base: float = 50_000.0, seed: int = 7) -> list[float]:
    """Reproducible random-walk price series (ASC, oldest→newest)."""
    rng = np.random.default_rng(seed)
    log_ret = rng.normal(0.0, 0.01, size=n - 1)
    prices = [base]
    for r in log_ret:
        prices.append(max(1.0, prices[-1] * math.exp(r)))
    return prices


def make_volumes(n: int = 220, seed: int = 8) -> list[float]:
    rng = np.random.default_rng(seed)
    return list(rng.uniform(1_000.0, 100_000.0, size=n))


def _fresh_predictor(market_key: str = "GOLD") -> RLDQNPredictor:
    """Return a fresh, untrained predictor with _market_key set."""
    p = RLDQNPredictor()
    p._market_key = market_key
    return p


# ---------------------------------------------------------------------------
# 1. Identity
# ---------------------------------------------------------------------------

class TestIdentity:

    def test_get_key(self):
        assert RLDQNPredictor().get_key() == "rl_dqn"

    def test_get_name(self):
        name = RLDQNPredictor().get_name()
        assert isinstance(name, str)
        assert len(name) > 0
        # Should be "RL DQN" per implementation
        assert "RL" in name or "DQN" in name or "dqn" in name.lower()

    def test_get_name_exact(self):
        assert RLDQNPredictor().get_name() == "RL DQN"


# ---------------------------------------------------------------------------
# 2. is_trained() — False before any training
# ---------------------------------------------------------------------------

class TestIsTrainedInitialState:

    def test_is_trained_false_before_any_action(self):
        p = RLDQNPredictor()
        assert p.is_trained() is False

    def test_is_trained_false_after_failed_checkpoint_load(self, tmp_path, monkeypatch):
        """When RL_MODEL_DIR points to empty dir, is_trained stays False."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        # act() triggers lazy load
        obs = np.zeros(35, dtype=np.float32)  # rough dim; hold fallback if wrong
        _ = p.act(obs)
        assert p.is_trained() is False


# ---------------------------------------------------------------------------
# 3. Fallback — predict() without checkpoint returns EMA result, no raise
# ---------------------------------------------------------------------------

class TestFallbackWithoutCheckpoint:

    def test_predict_returns_result_no_checkpoint(self, tmp_path, monkeypatch):
        """predict() must not raise even with no checkpoint on disk."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        result = p.predict(prices)
        assert isinstance(result, PredictionResult)

    def test_fallback_algorithm_name_correct(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        result = p.predict(prices)
        assert result.algorithm_name == "rl_dqn"

    def test_fallback_predicted_price_positive(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        result = p.predict(make_prices(220))
        assert result.predicted_price > 0.0

    def test_fallback_predicted_price_finite(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        result = p.predict(make_prices(220))
        assert math.isfinite(result.predicted_price)

    def test_fallback_confidence_in_bounds(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        result = p.predict(make_prices(220))
        assert 0.0 <= result.confidence <= 1.0

    def test_fallback_current_price_is_last_price(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        result = p.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-5)


# ---------------------------------------------------------------------------
# 4. Insufficient data raises ValueError
# ---------------------------------------------------------------------------

class TestInsufficientData:

    def test_too_few_prices_raises(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        with pytest.raises(ValueError, match="at least|needs"):
            p.predict([100.0] * 30)

    def test_exactly_min_data_points_does_not_raise(self, tmp_path, monkeypatch):
        """MIN_DATA_POINTS should be the inclusive lower bound."""
        from src.algorithms.rl_dqn import MIN_DATA_POINTS as MIN_DP
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(MIN_DP)
        # Must not raise (fallback EMA handles it)
        result = p.predict(prices)
        assert result is not None


# ---------------------------------------------------------------------------
# 5. _ema_fallback() — direct unit test
# ---------------------------------------------------------------------------

class TestEMAFallback:

    def test_ema_fallback_returns_prediction_result(self):
        p = _fresh_predictor("GOLD")
        prices = make_prices(100)
        result = p._ema_fallback(prices)
        assert isinstance(result, PredictionResult)

    def test_ema_fallback_algorithm_name(self):
        p = _fresh_predictor("GOLD")
        result = p._ema_fallback(make_prices(100))
        assert result.algorithm_name == "rl_dqn"

    def test_ema_fallback_confidence_in_range(self):
        p = _fresh_predictor("GOLD")
        result = p._ema_fallback(make_prices(100))
        assert 0.0 <= result.confidence <= 1.0

    def test_ema_fallback_respects_gold_clamp(self):
        p = _fresh_predictor("GOLD")
        prices = make_prices(100)
        result = p._ema_fallback(prices)
        current = prices[-1]
        max_chg = get_max_change_pct("GOLD")
        assert result.predicted_price >= current * (1 - max_chg) - 1e-9
        assert result.predicted_price <= current * (1 + max_chg) + 1e-9

    def test_ema_fallback_positive_price(self):
        p = _fresh_predictor("GOLD")
        result = p._ema_fallback(make_prices(100))
        assert result.predicted_price > 0.0


# ---------------------------------------------------------------------------
# 6. act() returns valid action when no checkpoint (hold=0 fallback)
# ---------------------------------------------------------------------------

class TestActNoCheckpoint:

    def test_act_returns_int(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        obs = np.zeros(40, dtype=np.float32)
        action = p.act(obs)
        assert isinstance(action, int)

    def test_act_returns_hold_when_no_checkpoint(self, tmp_path, monkeypatch):
        """With no checkpoint, act() must fall back to 0 (hold)."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        obs = np.zeros(40, dtype=np.float32)
        assert p.act(obs) == 0

    def test_act_returns_valid_action_after_train(self, tmp_path, monkeypatch):
        """After train_batch, act() must return action in {0, 1, 2}."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        monkeypatch.setattr("src.algorithms.rl_dqn.TRAIN_EPOCHS", 1)
        monkeypatch.setattr("src.algorithms.rl_dqn.MIN_REPLAY", 10)

        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        vols = make_volumes(220)
        p.train_batch([(prices, vols)])

        # Build an obs matching the trained in_dim
        obs = np.zeros(p._in_dim, dtype=np.float32)
        action = p.act(obs)
        assert action in {0, 1, 2}


# ---------------------------------------------------------------------------
# 7. Market-aware clamp
# ---------------------------------------------------------------------------

class TestMarketClamp:
    """For each market, predicted_price must stay within [current*(1-max%), current*(1+max%)]."""

    @pytest.mark.parametrize("market,max_pct", [
        ("GOLD", 0.15),
        ("NASDAQ100", 0.20),
        ("SP500", 0.15),
        ("CRYPTO", 0.50),
    ])
    def test_clamp_in_ema_fallback(self, market, max_pct, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor(market)
        prices = make_prices(220)
        # EMA fallback is guaranteed (no checkpoint), so clamp applies
        result = p.predict(prices)
        current = prices[-1]
        lo = current * (1 - max_pct) - 1e-9
        hi = current * (1 + max_pct) + 1e-9
        assert result.predicted_price >= lo, (
            f"{market}: predicted {result.predicted_price} < lower bound {lo}"
        )
        assert result.predicted_price <= hi, (
            f"{market}: predicted {result.predicted_price} > upper bound {hi}"
        )

    def test_clamp_unknown_market_uses_default(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("UNKNOWN_MARKET")
        prices = make_prices(220)
        result = p.predict(prices)
        current = prices[-1]
        default_pct = 0.15
        assert result.predicted_price >= current * (1 - default_pct) - 1e-9
        assert result.predicted_price <= current * (1 + default_pct) + 1e-9


# ---------------------------------------------------------------------------
# 8. Action → price direction mapping (trained network, greedy inference)
# ---------------------------------------------------------------------------

class TestActionPriceMapping:
    """After training, the action→price mapping in _inference must respect the spec:
      buy  (1) → predicted >= current
      sell (2) → predicted <= current
      hold (0) → predicted near current (small sigma * 0.1 nudge)

    Because we cannot deterministically control which action the trained network
    picks, we mock act() to force specific actions and verify the price mapping.
    """

    def _make_trained_predictor(self, tmp_path, monkeypatch) -> RLDQNPredictor:
        """Helper: return a predictor with a live (freshly trained) qnet."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        monkeypatch.setattr("src.algorithms.rl_dqn.TRAIN_EPOCHS", 1)
        monkeypatch.setattr("src.algorithms.rl_dqn.MIN_REPLAY", 10)
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        p.train_batch([(prices, None)])
        return p

    def test_buy_action_predicts_up(self, tmp_path, monkeypatch):
        """When act() returns 1 (buy), predicted_price >= current_price."""
        p = self._make_trained_predictor(tmp_path, monkeypatch)
        prices = make_prices(220)
        # Force act() to return buy=1
        monkeypatch.setattr(p, "act", lambda obs: 1)
        result = p.predict(prices)
        assert result.predicted_price >= result.current_price - 1e-9, (
            f"BUY action: predicted {result.predicted_price} should be >= current {result.current_price}"
        )

    def test_sell_action_predicts_down(self, tmp_path, monkeypatch):
        """When act() returns 2 (sell), predicted_price <= current_price."""
        p = self._make_trained_predictor(tmp_path, monkeypatch)
        prices = make_prices(220)
        monkeypatch.setattr(p, "act", lambda obs: 2)
        result = p.predict(prices)
        assert result.predicted_price <= result.current_price + 1e-9, (
            f"SELL action: predicted {result.predicted_price} should be <= current {result.current_price}"
        )

    def test_hold_action_predicts_near_current(self, tmp_path, monkeypatch):
        """When act() returns 0 (hold), predicted_price ≈ current (within 3%)."""
        p = self._make_trained_predictor(tmp_path, monkeypatch)
        prices = make_prices(220)
        monkeypatch.setattr(p, "act", lambda obs: 0)
        result = p.predict(prices)
        current = result.current_price
        # Hold: predicted = current * (1 + 0.1 * sigma); sigma << 0.3 in typical market
        assert abs(result.predicted_price - current) / current < 0.03, (
            f"HOLD action: predicted {result.predicted_price} too far from current {current}"
        )

    def test_action_directions_all_within_clamp(self, tmp_path, monkeypatch):
        """All three forced-action results must still respect the GOLD clamp."""
        p = self._make_trained_predictor(tmp_path, monkeypatch)
        prices = make_prices(220)
        current = prices[-1]
        max_pct = get_max_change_pct("GOLD")

        for forced_action in (0, 1, 2):
            monkeypatch.setattr(p, "act", lambda obs, a=forced_action: a)
            result = p.predict(prices)
            lo = current * (1 - max_pct) - 1e-9
            hi = current * (1 + max_pct) + 1e-9
            assert lo <= result.predicted_price <= hi, (
                f"action={forced_action}: {result.predicted_price} outside [{lo}, {hi}]"
            )


# ---------------------------------------------------------------------------
# 9. train_batch() — creates checkpoint, flips is_trained
# ---------------------------------------------------------------------------

class TestTrainBatch:
    """train_batch() with accelerated hyper-parameters (patched TRAIN_EPOCHS, MIN_REPLAY)."""

    @pytest.fixture(autouse=True)
    def _fast_train(self, monkeypatch):
        """Reduce epochs and MIN_REPLAY so training finishes in <2 s."""
        monkeypatch.setattr("src.algorithms.rl_dqn.TRAIN_EPOCHS", 1)
        monkeypatch.setattr("src.algorithms.rl_dqn.MIN_REPLAY", 10)

    def test_train_batch_flips_is_trained(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        assert p.is_trained() is False
        prices = make_prices(220)
        p.train_batch([(prices, None)])
        assert p.is_trained() is True

    def test_train_batch_creates_checkpoint_file(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        p.train_batch([(prices, None)])
        ckpt = _checkpoint_path("GOLD")
        assert os.path.exists(ckpt), f"Checkpoint not found at {ckpt}"

    def test_train_batch_checkpoint_loadable(self, tmp_path, monkeypatch):
        """Saved checkpoint must be loadable via torch.load."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        p.train_batch([(prices, None)])

        ckpt = _checkpoint_path("GOLD")
        data = torch.load(ckpt, map_location="cpu", weights_only=False)
        assert "in_dim" in data
        assert "state_dict" in data
        assert data["in_dim"] > 0

    def test_train_batch_with_volumes(self, tmp_path, monkeypatch):
        """train_batch must not raise when volumes are provided."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        vols = make_volumes(220)
        p.train_batch([(prices, vols)])
        assert p.is_trained() is True

    def test_train_batch_empty_series_no_op(self, tmp_path, monkeypatch):
        """Empty series list: train_batch should return without error."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        p.train_batch([])
        assert p.is_trained() is False

    def test_train_batch_short_series_skipped(self, tmp_path, monkeypatch):
        """Series shorter than MIN_DATA_POINTS must be silently skipped."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        # Only 30 prices — below MIN_DATA_POINTS (80)
        p.train_batch([([100.0] * 30, None)])
        # Still untrained because all series were skipped
        assert p.is_trained() is False

    def test_predict_after_train_returns_valid_result(self, tmp_path, monkeypatch):
        """After train_batch, predict() should use the DQN path (inference path)."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        p.train_batch([(prices, None)])
        result = p.predict(prices)
        assert isinstance(result, PredictionResult)
        assert result.predicted_price > 0.0
        assert math.isfinite(result.predicted_price)
        assert 0.0 <= result.confidence <= 1.0

    def test_train_batch_multiple_series(self, tmp_path, monkeypatch):
        """train_batch with multiple series must succeed and create checkpoint."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("CRYPTO")
        series = [
            (make_prices(220, base=60_000.0, seed=i), None)
            for i in range(3)
        ]
        p.train_batch(series)
        assert p.is_trained() is True
        ckpt = _checkpoint_path("CRYPTO")
        assert os.path.exists(ckpt)


# ---------------------------------------------------------------------------
# 10. Checkpoint load — _try_load_checkpoint
# ---------------------------------------------------------------------------

class TestCheckpointLoad:

    def test_load_checkpoint_after_train(self, tmp_path, monkeypatch):
        """Fresh predictor should lazy-load from checkpoint saved by another instance."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        monkeypatch.setattr("src.algorithms.rl_dqn.TRAIN_EPOCHS", 1)
        monkeypatch.setattr("src.algorithms.rl_dqn.MIN_REPLAY", 10)

        # Train + save
        trainer = _fresh_predictor("GOLD")
        trainer.train_batch([(make_prices(220), None)])
        assert trainer.is_trained()

        # New instance should lazy-load on first act()
        loader = _fresh_predictor("GOLD")
        assert loader.is_trained() is False
        obs = np.zeros(trainer._in_dim, dtype=np.float32)
        action = loader.act(obs)
        assert action in {0, 1, 2}
        assert loader.is_trained() is True

    def test_load_missing_checkpoint_returns_false(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("SP500")
        success = p._try_load_checkpoint()
        assert success is False
        assert p.is_trained() is False


# ---------------------------------------------------------------------------
# 11. _ReplayBuffer — basic sanity
# ---------------------------------------------------------------------------

class TestReplayBuffer:

    def test_push_and_len(self):
        buf = _ReplayBuffer(100)
        assert len(buf) == 0
        buf.push(np.zeros(5), 0, 0.1, np.zeros(5), False)
        assert len(buf) == 1

    def test_capacity_overflow_evicts_oldest(self):
        buf = _ReplayBuffer(3)
        for i in range(5):
            buf.push(np.array([float(i)]), i % 3, float(i), np.array([float(i)]), False)
        assert len(buf) == 3  # capped at capacity

    def test_sample_returns_correct_count(self):
        buf = _ReplayBuffer(100)
        dummy = np.zeros(4, dtype=np.float32)
        for i in range(20):
            buf.push(dummy, i % 3, float(i), dummy, i == 19)
        batch = buf.sample(10)
        assert len(batch) == 10

    def test_sample_each_element_has_5_fields(self):
        buf = _ReplayBuffer(50)
        dummy = np.zeros(4, dtype=np.float32)
        for i in range(10):
            buf.push(dummy, 0, 0.0, dummy, False)
        for s, a, r, ns, d in buf.sample(5):
            assert isinstance(a, int)
            assert isinstance(r, float)
            assert isinstance(d, bool)


# ---------------------------------------------------------------------------
# 12. _build_qnet — shape correctness
# ---------------------------------------------------------------------------

class TestBuildQNet:

    def test_output_shape(self):
        net = _build_qnet(in_dim=40)
        x = torch.randn(1, 40)
        out = net(x)
        assert out.shape == (1, N_ACTIONS)

    def test_batch_output_shape(self):
        net = _build_qnet(in_dim=35)
        x = torch.randn(64, 35)
        out = net(x)
        assert out.shape == (64, N_ACTIONS)

    def test_different_in_dims_work(self):
        for d in (10, 33, 40, 100):
            net = _build_qnet(d)
            out = net(torch.randn(2, d))
            assert out.shape == (2, N_ACTIONS)


# ---------------------------------------------------------------------------
# 13. _checkpoint_path — naming convention
# ---------------------------------------------------------------------------

class TestCheckpointPath:

    def test_path_contains_market_key(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        path = _checkpoint_path("GOLD")
        assert "GOLD" in path

    def test_path_ends_with_pt(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        path = _checkpoint_path("GOLD")
        assert path.endswith(".pt")

    def test_path_includes_rl_dqn(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        path = _checkpoint_path("CRYPTO")
        assert "rl_dqn" in os.path.basename(path)

    def test_path_normalises_slash(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        path = _checkpoint_path("BTC/USD")
        assert "/" not in os.path.basename(path)


# ---------------------------------------------------------------------------
# 14. PredictionResult fields populated correctly (integration-lite)
# ---------------------------------------------------------------------------

class TestPredictionResultFields:

    def test_result_fields_with_fallback(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("SP500")
        prices = make_prices(220, base=4500.0)
        result = p.predict(prices)
        assert result.algorithm_name == "rl_dqn"
        assert isinstance(result.predicted_price, float)
        assert isinstance(result.current_price, float)
        assert isinstance(result.confidence, float)

    def test_current_price_equals_last_price(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor("GOLD")
        prices = make_prices(220)
        result = p.predict(prices)
        assert result.current_price == pytest.approx(prices[-1], rel=1e-5)

    @pytest.mark.parametrize("market", ["GOLD", "NASDAQ100", "SP500", "CRYPTO"])
    def test_predicted_price_positive_all_markets(self, market, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        p = _fresh_predictor(market)
        result = p.predict(make_prices(220))
        assert result.predicted_price > 0.0
        assert math.isfinite(result.predicted_price)


# ---------------------------------------------------------------------------
# 15. v3 — Dueling architecture + legacy MLP compat
# ---------------------------------------------------------------------------

class TestArchitectures:

    def test_default_arch_is_dueling_and_has_correct_shape(self):
        net = _build_qnet(40)
        assert hasattr(net, "advantage"), "default arch should be dueling"
        out = net(torch.randn(4, 40))
        assert out.shape == (4, N_ACTIONS)

    def test_mlp_arch_matches_legacy_state_dict_layout(self):
        net = _build_qnet(40, arch=ARCH_MLP)
        keys = set(net.state_dict().keys())
        # Legacy checkpoints have Sequential keys net.0/net.2/net.4
        assert "net.0.weight" in keys and "net.4.bias" in keys
        out = net(torch.randn(2, 40))
        assert out.shape == (2, N_ACTIONS)

    def test_legacy_checkpoint_without_arch_tag_loads(self, tmp_path, monkeypatch):
        """A pre-v3 checkpoint ({in_dim, state_dict} only) must load as MLP."""
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        in_dim = N_ENHANCED_FEATURES + N_POS_FEATURES
        legacy_net = _build_qnet(in_dim, arch=ARCH_MLP)
        os.makedirs(tmp_path, exist_ok=True)
        torch.save(
            {"in_dim": in_dim, "state_dict": legacy_net.state_dict()},
            _checkpoint_path("GOLD"),
        )

        p = _fresh_predictor("GOLD")
        obs = np.zeros(in_dim, dtype=np.float32)
        action = p.act(obs)
        assert action in {0, 1, 2}
        assert p.is_trained() is True
        assert p._arch == ARCH_MLP
        assert p._feat_scale is False  # legacy nets get raw features

    def test_v3_checkpoint_roundtrip_keeps_arch_and_scale(self, tmp_path, monkeypatch):
        monkeypatch.setenv("RL_MODEL_DIR", str(tmp_path))
        monkeypatch.setattr("src.algorithms.rl_dqn.TRAIN_EPOCHS", 1)
        monkeypatch.setattr("src.algorithms.rl_dqn.MIN_REPLAY", 10)
        trainer = _fresh_predictor("GOLD")
        trainer.train_batch([(make_prices(220), None)])

        loader = _fresh_predictor("GOLD")
        assert loader._try_load_checkpoint() is True
        assert loader._arch == ARCH_DUELING
        assert loader._feat_scale is True


# ---------------------------------------------------------------------------
# 16. v3 — Prioritized replay buffer
# ---------------------------------------------------------------------------

class TestPrioritizedReplay:

    def _filled(self, n=50):
        buf = _ReplayBuffer(100)
        dummy = np.zeros(4, dtype=np.float32)
        for i in range(n):
            buf.push(dummy, i % 3, float(i), dummy, 0.97)
        return buf

    def test_sample_per_returns_batch_indices_weights(self):
        buf = self._filled()
        batch, idx, w = buf.sample_per(16)
        assert len(batch) == 16 and len(idx) == 16 and len(w) == 16
        assert all(0 <= int(i) < len(buf) for i in idx)
        assert float(w.max()) == pytest.approx(1.0)
        assert (w > 0).all()

    def test_update_priorities_biases_sampling(self):
        buf = self._filled(20)
        # Give transition 0 a huge priority; it should dominate samples
        buf.update_priorities([0], [1000.0])
        _, idx, _ = buf.sample_per(10)
        assert (idx == 0).sum() >= 1

    def test_uniform_sample_api_still_works(self):
        buf = self._filled(30)
        batch = buf.sample(10)
        assert len(batch) == 10


# ---------------------------------------------------------------------------
# 17. v3 — Feature scaling
# ---------------------------------------------------------------------------

class TestFeatureScaling:

    def test_rsi_and_stoch_columns_rescaled(self):
        mat = np.zeros((2, N_ENHANCED_FEATURES), dtype=np.float32)
        mat[:, 17:20] = 50.0   # RSI + stoch at neutral 50
        mat[:, 10:14] = 1.0    # MA ratios at parity
        mat[:, 20] = 0.5       # bb %B mid
        mat[:, 29] = 1.0       # vol ratio parity
        out = _scale_feature_matrix(mat)
        # Neutral values must map to ~0 so all features share scale
        assert np.allclose(out[:, 17:20], 0.0)
        assert np.allclose(out[:, 10:14], 0.0)
        assert np.allclose(out[:, 20], 0.0)
        assert np.allclose(out[:, 29], 0.0)

    def test_non_canonical_width_passthrough(self):
        mat = np.full((3, 14), 77.0, dtype=np.float32)
        out = _scale_feature_matrix(mat)
        assert np.allclose(out, 77.0)

    def test_input_matrix_not_mutated(self):
        mat = np.full((2, N_ENHANCED_FEATURES), 50.0, dtype=np.float32)
        _ = _scale_feature_matrix(mat)
        assert np.allclose(mat, 50.0)


# ---------------------------------------------------------------------------
# 18. v3 — Window alignment fix
# ---------------------------------------------------------------------------

class TestWindowAlignment:

    def test_prices_tail_aligned_to_obs_rows(self):
        """obs_matrix rows are as-of prices[j+offset] — windows must pair them."""
        n_prices = 200
        offset = 20  # build_enhanced_features starts at index 20
        obs = np.arange((n_prices - offset) * 2, dtype=np.float32).reshape(-1, 2)
        prices = np.arange(n_prices, dtype=np.float32)
        wins = _make_windows(obs, prices, window=130, stride=35)
        for w_mat, w_prices in wins:
            assert len(w_mat) == len(w_prices)
        # First window's first price must be prices[offset], not prices[0]
        assert wins[0][1][0] == float(offset)

    def test_equal_length_inputs_unchanged(self):
        obs = np.zeros((100, 3), dtype=np.float32)
        prices = np.arange(100, dtype=np.float32)
        wins = _make_windows(obs, prices, window=130, stride=35)
        assert len(wins) == 1
        assert wins[0][1][0] == 0.0


# ---------------------------------------------------------------------------
# 19. v3 — n-step transitions
# ---------------------------------------------------------------------------

class TestNStepTransitions:

    def test_episode_pushes_nstep_discounts(self):
        rng = np.random.default_rng(3)
        n_steps, n_base = 40, 30
        obs = rng.normal(0, 0.1, size=(n_steps, n_base)).astype(np.float32)
        prices = np.cumprod(1 + rng.normal(0, 0.01, n_steps)).astype(np.float32) * 100
        net = _build_qnet(n_base + N_POS_FEATURES)
        buf = _ReplayBuffer(1000)

        total, gs = _run_episode(obs, prices, net, buf, epsilon=0.0, global_step=0)
        assert gs == n_steps - 1
        assert math.isfinite(total)
        assert len(buf) == n_steps - 1  # every raw step yields one transition

        gamma_n = GAMMA ** N_STEP
        discs = {round(float(entry[4]), 8) for entry in buf._data}
        # Bootstrapped transitions carry γ^n; the terminal flush carries 0.0
        assert round(gamma_n, 8) in discs
        assert 0.0 in discs
        for _, action, reward, _, disc in buf._data:
            assert action in {0, 1, 2}
            assert math.isfinite(reward)
            assert 0.0 <= float(disc) <= gamma_n + 1e-9
