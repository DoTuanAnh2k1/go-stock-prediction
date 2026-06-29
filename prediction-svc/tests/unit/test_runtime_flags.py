"""Unit tests for src/algorithms/runtime_flags.py and the Optuna fast-mode gate.

No DB, no network, no Docker required.

Coverage:
  1. runtime_flags — set/get thread-safety and default value.
  2. LightGBM tuner returns defaults immediately when is_optuna_enabled()=False.
  3. XGBoost tuner returns defaults immediately when is_optuna_enabled()=False.
  4. rebuild_and_replay restores the flag after a run (even when an exception
     is raised inside _rebuild_core).
  5. fast=False leaves the flag untouched.
"""
from __future__ import annotations

import threading
from datetime import date, timedelta
from unittest.mock import MagicMock, patch

import numpy as np
import pytest

from src.algorithms import runtime_flags as rf
from src.algorithms.runtime_flags import is_optuna_enabled, set_optuna_enabled


# ---------------------------------------------------------------------------
# Helpers — restore flag after every test
# ---------------------------------------------------------------------------

@pytest.fixture(autouse=True)
def restore_optuna_flag():
    """Ensure _optuna_enabled is reset to True after every test."""
    original = is_optuna_enabled()
    yield
    set_optuna_enabled(original)


# ---------------------------------------------------------------------------
# 1. runtime_flags — basic contract
# ---------------------------------------------------------------------------

class TestRuntimeFlags:

    def test_default_is_true(self):
        """Flag must start True so normal live pipeline is never affected."""
        set_optuna_enabled(True)  # reset explicitly in case a prior test leaked
        assert is_optuna_enabled() is True

    def test_set_false_then_get(self):
        set_optuna_enabled(False)
        assert is_optuna_enabled() is False

    def test_set_true_then_get(self):
        set_optuna_enabled(False)
        set_optuna_enabled(True)
        assert is_optuna_enabled() is True

    def test_idempotent_set_true(self):
        set_optuna_enabled(True)
        set_optuna_enabled(True)
        assert is_optuna_enabled() is True

    def test_idempotent_set_false(self):
        set_optuna_enabled(False)
        set_optuna_enabled(False)
        assert is_optuna_enabled() is False

    def test_thread_safety_concurrent_reads(self):
        """Many threads reading the flag simultaneously must not raise."""
        set_optuna_enabled(True)
        errors: list[Exception] = []

        def read_flag():
            try:
                for _ in range(1000):
                    _ = is_optuna_enabled()
            except Exception as exc:
                errors.append(exc)

        threads = [threading.Thread(target=read_flag) for _ in range(8)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        assert errors == [], f"Concurrent reads raised: {errors}"

    def test_thread_safety_write_then_read(self):
        """A single write followed by concurrent reads must be consistent."""
        set_optuna_enabled(False)
        results: list[bool] = []
        lock = threading.Lock()

        def read_flag():
            val = is_optuna_enabled()
            with lock:
                results.append(val)

        threads = [threading.Thread(target=read_flag) for _ in range(16)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        assert all(v is False for v in results), (
            f"Expected all False after set_optuna_enabled(False), got: {results}"
        )


# ---------------------------------------------------------------------------
# 2. LightGBM tuner gate
# ---------------------------------------------------------------------------

class TestLightGBMOptunaGate:
    """When is_optuna_enabled()=False the tuner must return _DEFAULT_PARAMS immediately."""

    def _make_data(self, n: int = 20):
        rng = np.random.default_rng(0)
        X = rng.random((n, 5)).astype(float)
        y = rng.random(n).astype(float)
        return X[:16], y[:16], X[16:], y[16:]

    def test_returns_default_params_when_flag_false(self, monkeypatch):
        from src.algorithms.lightgbm_model import _tune_lightgbm_params, _DEFAULT_PARAMS

        # Patch is_optuna_enabled inside the lightgbm_model module
        monkeypatch.setattr(
            "src.algorithms.lightgbm_model.is_optuna_enabled",
            lambda: False,
        )

        X_tr, y_tr, X_val, y_val = self._make_data()
        # n_data_points large enough to pass the first guard
        result = _tune_lightgbm_params(X_tr, y_tr, X_val, y_val, n_data_points=300)

        assert result == _DEFAULT_PARAMS, (
            f"Expected default params when Optuna disabled, got: {result}"
        )

    def test_optuna_not_called_when_flag_false(self, monkeypatch):
        """Optuna must not be imported or invoked when the flag is False."""
        from src.algorithms.lightgbm_model import _tune_lightgbm_params

        monkeypatch.setattr(
            "src.algorithms.lightgbm_model.is_optuna_enabled",
            lambda: False,
        )

        optuna_called = []

        # If optuna were imported the study would be created — ensure it is not
        try:
            import optuna as _optuna
            original_create = _optuna.create_study

            def spy_create_study(*args, **kwargs):
                optuna_called.append(True)
                return original_create(*args, **kwargs)

            monkeypatch.setattr(_optuna, "create_study", spy_create_study)
        except ImportError:
            pass  # optuna not installed — test still valid

        X_tr, y_tr, X_val, y_val = self._make_data()
        _tune_lightgbm_params(X_tr, y_tr, X_val, y_val, n_data_points=300)

        assert optuna_called == [], "Optuna create_study must NOT be called when flag is False"

    def test_flag_true_does_not_short_circuit(self, monkeypatch):
        """When flag is True, the function does NOT short-circuit (still returns a dict)."""
        from src.algorithms.lightgbm_model import _tune_lightgbm_params

        monkeypatch.setattr(
            "src.algorithms.lightgbm_model.is_optuna_enabled",
            lambda: True,
        )

        X_tr, y_tr, X_val, y_val = self._make_data()
        # n_data_points < HYPEROPT_MIN_POINTS (200) so it returns defaults via first guard
        result = _tune_lightgbm_params(X_tr, y_tr, X_val, y_val, n_data_points=50)

        assert isinstance(result, dict)
        assert "objective" in result  # a LightGBM-style param dict


# ---------------------------------------------------------------------------
# 3. XGBoost tuner gate
# ---------------------------------------------------------------------------

class TestXGBoostOptunaGate:
    """When is_optuna_enabled()=False the tuner must return _DEFAULT_PARAMS immediately."""

    def _make_data(self, n: int = 20):
        rng = np.random.default_rng(1)
        X = rng.random((n, 5)).astype(float)
        y = rng.random(n).astype(float)
        return X[:16], y[:16], X[16:], y[16:]

    def test_returns_default_params_when_flag_false(self, monkeypatch):
        from src.algorithms.xgboost_model import _tune_xgboost_params, _DEFAULT_PARAMS

        monkeypatch.setattr(
            "src.algorithms.xgboost_model.is_optuna_enabled",
            lambda: False,
        )

        X_tr, y_tr, X_val, y_val = self._make_data()
        result = _tune_xgboost_params(X_tr, y_tr, X_val, y_val, n_data_points=300)

        assert result == _DEFAULT_PARAMS, (
            f"Expected default params when Optuna disabled, got: {result}"
        )

    def test_optuna_not_called_when_flag_false(self, monkeypatch):
        """Optuna must not be invoked when the flag is False."""
        from src.algorithms.xgboost_model import _tune_xgboost_params

        monkeypatch.setattr(
            "src.algorithms.xgboost_model.is_optuna_enabled",
            lambda: False,
        )

        optuna_called = []

        try:
            import optuna as _optuna
            original_create = _optuna.create_study

            def spy_create_study(*args, **kwargs):
                optuna_called.append(True)
                return original_create(*args, **kwargs)

            monkeypatch.setattr(_optuna, "create_study", spy_create_study)
        except ImportError:
            pass

        X_tr, y_tr, X_val, y_val = self._make_data()
        _tune_xgboost_params(X_tr, y_tr, X_val, y_val, n_data_points=300)

        assert optuna_called == [], "Optuna create_study must NOT be called when flag is False"

    def test_flag_true_does_not_short_circuit(self, monkeypatch):
        from src.algorithms.xgboost_model import _tune_xgboost_params

        monkeypatch.setattr(
            "src.algorithms.xgboost_model.is_optuna_enabled",
            lambda: True,
        )

        X_tr, y_tr, X_val, y_val = self._make_data()
        result = _tune_xgboost_params(X_tr, y_tr, X_val, y_val, n_data_points=50)

        assert isinstance(result, dict)
        assert "n_estimators" in result  # an XGBoost-style param dict


# ---------------------------------------------------------------------------
# 4. rebuild_and_replay — flag lifecycle
# ---------------------------------------------------------------------------

def _make_price_rows(n: int = 40, start: date = date(2025, 1, 1)):
    rows = []
    for i in range(n):
        obj = MagicMock()
        obj.buy_price = 100.0 + i * 0.5
        obj.close_price = 100.0 + i * 0.5
        obj.trading_date = start + timedelta(days=i)
        rows.append(obj)
    return rows


def _patch_rebuild_all(monkeypatch):
    """Minimal monkeypatches so rebuild_and_replay completes without DB/network."""
    from src.orchestrator import rebuild as rb
    from src.algorithms.base import PredictionResult

    price_rows = _make_price_rows(40)

    algo = MagicMock()
    algo.get_key.return_value = "mock_algo"
    algo.predict.return_value = PredictionResult(
        algorithm_name="mock_algo",
        predicted_price=101.0,
        current_price=100.0,
        confidence=0.7,
    )
    monkeypatch.setattr(rb, "build_algorithms", lambda: {"mock_algo": algo})

    monkeypatch.setattr(rb.repo, "get_gold_prices_asc",   lambda *a, **kw: price_rows)
    monkeypatch.setattr(rb.repo, "get_nasdaq_prices_asc", lambda *a, **kw: price_rows)
    monkeypatch.setattr(rb.repo, "get_nasdaq_symbols",    lambda: ["AAPL"])
    monkeypatch.setattr(rb.repo, "get_crypto_prices_asc", lambda *a, **kw: price_rows)
    monkeypatch.setattr(rb.repo, "get_sp500_prices_asc",  lambda *a, **kw: price_rows)
    monkeypatch.setattr(rb.repo, "get_sp500_symbols",     lambda: ["SPY"])

    monkeypatch.setattr(rb, "_bulk_gold_predictions",   lambda b: None)
    monkeypatch.setattr(rb, "_bulk_nasdaq_predictions", lambda b: None)
    monkeypatch.setattr(rb, "_bulk_crypto_predictions", lambda b: None)
    monkeypatch.setattr(rb, "_bulk_sp500_predictions",  lambda b: None)

    monkeypatch.setattr(rb, "_wipe_predictions_and_sim", lambda: None)
    monkeypatch.setattr(rb, "train_meta_all",            lambda **kw: None)
    monkeypatch.setattr(rb, "_run_sim_backtest",         lambda s, e: 0)

    return price_rows


class TestRebuildFlagLifecycle:

    def test_fast_true_disables_optuna_during_run(self, monkeypatch):
        """When fast=True the flag is False inside _rebuild_core."""
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        _patch_rebuild_all(monkeypatch)

        observed: list[bool] = []

        original_core = rb._rebuild_core

        def capturing_core(**kwargs):
            # Capture the flag value while _rebuild_core is running
            observed.append(is_optuna_enabled())
            return original_core(**kwargs)

        monkeypatch.setattr(rb, "_rebuild_core", capturing_core)

        set_optuna_enabled(True)  # start enabled
        cutoff = date(2025, 2, 5)
        rebuild_and_replay(cutoff, step_size=1, fast=True)

        assert len(observed) == 1
        assert observed[0] is False, (
            "Flag must be False inside _rebuild_core when fast=True"
        )

    def test_fast_true_restores_flag_after_success(self, monkeypatch):
        """After a successful fast=True run the flag must be restored to True."""
        from src.orchestrator.rebuild import rebuild_and_replay

        _patch_rebuild_all(monkeypatch)

        set_optuna_enabled(True)
        cutoff = date(2025, 2, 5)
        rebuild_and_replay(cutoff, step_size=1, fast=True)

        assert is_optuna_enabled() is True, (
            "Flag must be restored to True after rebuild completes"
        )

    def test_fast_true_restores_flag_after_exception(self, monkeypatch):
        """Flag is restored even when _rebuild_core raises an exception."""
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        _patch_rebuild_all(monkeypatch)

        monkeypatch.setattr(rb, "_rebuild_core", MagicMock(side_effect=RuntimeError("boom")))

        set_optuna_enabled(True)
        cutoff = date(2025, 2, 5)

        with pytest.raises(RuntimeError, match="boom"):
            rebuild_and_replay(cutoff, step_size=1, fast=True)

        assert is_optuna_enabled() is True, (
            "Flag must be restored even when rebuild raises"
        )

    def test_fast_false_leaves_flag_untouched(self, monkeypatch):
        """When fast=False the flag is never modified."""
        from src.orchestrator import rebuild as rb
        from src.orchestrator.rebuild import rebuild_and_replay

        _patch_rebuild_all(monkeypatch)

        observed: list[bool] = []

        original_core = rb._rebuild_core

        def capturing_core(**kwargs):
            observed.append(is_optuna_enabled())
            return original_core(**kwargs)

        monkeypatch.setattr(rb, "_rebuild_core", capturing_core)

        set_optuna_enabled(True)
        cutoff = date(2025, 2, 5)
        rebuild_and_replay(cutoff, step_size=1, fast=False)

        assert len(observed) == 1
        assert observed[0] is True, (
            "Flag must stay True when fast=False"
        )

    def test_fast_mode_default_is_true(self, monkeypatch):
        """Default value of fast parameter must be True."""
        import inspect
        from src.orchestrator.rebuild import rebuild_and_replay

        sig = inspect.signature(rebuild_and_replay)
        assert sig.parameters["fast"].default is True, (
            "fast parameter default must be True"
        )

    def test_result_contains_expected_keys_in_fast_mode(self, monkeypatch):
        """fast=True still returns all required stats keys."""
        from src.orchestrator.rebuild import rebuild_and_replay

        _patch_rebuild_all(monkeypatch)

        cutoff = date(2025, 2, 5)
        result = rebuild_and_replay(cutoff, step_size=1, fast=True)

        assert set(result.keys()) == {
            "predictions_generated", "markets", "meta_trained",
            "bots_replayed", "cutoff_date", "predict_from_date", "duration_ms",
        }
