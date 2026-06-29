"""Tests for FAIL LOUD, NEVER FAKE policy.

Verifies:
(a) When an algorithm's real computation fails, predict() re-raises — no fake
    EMA row is written under the algorithm's name.
(b) Orchestrator skips the failing algorithm's DB write; other algorithms still
    run and produce real predictions.

No network, no DB, no Docker required.
"""
from __future__ import annotations

from decimal import Decimal
from unittest.mock import MagicMock, patch

import pytest

from src.algorithms.base import PredictionResult


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_prices(n: int = 150, base: float = 100.0) -> list[float]:
    """Simple monotone price series with noise, oldest → newest."""
    import random
    rng = random.Random(42)
    prices = [base]
    for _ in range(n - 1):
        prices.append(max(1.0, prices[-1] * (1 + rng.uniform(-0.01, 0.01))))
    return prices


def _make_result(algo_name: str = "test", price: float = 101.0) -> PredictionResult:
    return PredictionResult(
        predicted_price=price,
        confidence=0.7,
        current_price=100.0,
        algorithm_name=algo_name,
    )


def _set_market(algo, market_key: str = "GOLD") -> None:
    algo._market_key = market_key


# ---------------------------------------------------------------------------
# 1. ARIMA-GARCH — predict() must raise when _fit_and_predict fails
# ---------------------------------------------------------------------------

def test_arima_garch_raises_on_fit_failure(monkeypatch):
    """predict() must propagate the exception, not silently return EMA."""
    from src.algorithms.arima_garch import ARIMAGARCHPredictor

    algo = ARIMAGARCHPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_fit_and_predict", MagicMock(side_effect=RuntimeError("fit boom")))

    with pytest.raises(RuntimeError, match="fit boom"):
        algo.predict(_make_prices(100))


def test_arima_garch_no_fake_ema_result_on_failure(monkeypatch):
    """predict() must not return any PredictionResult when computation fails."""
    from src.algorithms.arima_garch import ARIMAGARCHPredictor

    algo = ARIMAGARCHPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_fit_and_predict", MagicMock(side_effect=ValueError("arima broken")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None, "predict() must not return a result when computation fails"


# ---------------------------------------------------------------------------
# 2. EGARCH — predict() must raise when _fit_and_predict fails
# ---------------------------------------------------------------------------

def test_egarch_raises_on_fit_failure(monkeypatch):
    from src.algorithms.egarch import EGARCHPredictor

    algo = EGARCHPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_fit_and_predict", MagicMock(side_effect=RuntimeError("egarch boom")))

    with pytest.raises(RuntimeError, match="egarch boom"):
        algo.predict(_make_prices(100))


def test_egarch_no_fake_ema_result_on_failure(monkeypatch):
    from src.algorithms.egarch import EGARCHPredictor

    algo = EGARCHPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_fit_and_predict", MagicMock(side_effect=OSError("libgomp missing")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 3. SARIMA — predict() must raise when _fit_and_predict fails
# ---------------------------------------------------------------------------

def test_sarima_raises_on_fit_failure(monkeypatch):
    from src.algorithms.sarima import SARIMAPredictor

    algo = SARIMAPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_fit_and_predict", MagicMock(side_effect=RuntimeError("sarima convergence")))

    with pytest.raises(RuntimeError, match="sarima convergence"):
        algo.predict(_make_prices(100))


def test_sarima_no_fake_ema_on_failure(monkeypatch):
    from src.algorithms.sarima import SARIMAPredictor

    algo = SARIMAPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_fit_and_predict", MagicMock(side_effect=RuntimeError("sarima broken")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 4. LSTM — predict() must raise when both inference and train-and-predict fail
# ---------------------------------------------------------------------------

def test_lstm_raises_when_train_fails(monkeypatch):
    """When _train_and_predict also fails, predict() must raise (no EMA)."""
    pytest.importorskip("torch", reason="PyTorch not installed")
    from src.algorithms.lstm import LSTMPredictor

    algo = LSTMPredictor()
    _set_market(algo)
    # No cached model, so falls through directly to _train_and_predict
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("torch broken")))

    with pytest.raises(RuntimeError, match="torch broken"):
        algo.predict(_make_prices(80))


def test_lstm_no_fake_ema_on_train_failure(monkeypatch):
    pytest.importorskip("torch", reason="PyTorch not installed")
    from src.algorithms.lstm import LSTMPredictor

    algo = LSTMPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("no torch")))

    result = None
    try:
        result = algo.predict(_make_prices(80))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 5. GRU — predict() must raise when _train_and_predict fails
# ---------------------------------------------------------------------------

def test_gru_raises_when_train_fails(monkeypatch):
    pytest.importorskip("torch", reason="PyTorch not installed")
    from src.algorithms.gru import GRUPredictor

    algo = GRUPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("gru broken")))

    with pytest.raises(RuntimeError, match="gru broken"):
        algo.predict(_make_prices(80))


def test_gru_no_fake_ema_on_train_failure(monkeypatch):
    pytest.importorskip("torch", reason="PyTorch not installed")
    from src.algorithms.gru import GRUPredictor

    algo = GRUPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("gru broken")))

    result = None
    try:
        result = algo.predict(_make_prices(80))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 6. LightGBM — predict() must raise when _train_and_predict fails
# ---------------------------------------------------------------------------

def test_lightgbm_raises_when_train_fails(monkeypatch):
    pytest.importorskip("lightgbm", reason="lightgbm not installed")
    from src.algorithms.lightgbm_model import LightGBMPredictor

    algo = LightGBMPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=OSError("libgomp1 missing")))

    with pytest.raises(OSError, match="libgomp1 missing"):
        algo.predict(_make_prices(100))


def test_lightgbm_no_fake_ema_on_train_failure(monkeypatch):
    pytest.importorskip("lightgbm", reason="lightgbm not installed")
    from src.algorithms.lightgbm_model import LightGBMPredictor

    algo = LightGBMPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=OSError("libgomp1 missing")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None


def test_lightgbm_no_ema_under_lightgbm_name_on_failure(monkeypatch):
    """The result, if any, must not be an EMA prediction disguised as lightgbm."""
    pytest.importorskip("lightgbm", reason="lightgbm not installed")
    from src.algorithms.lightgbm_model import LightGBMPredictor

    algo = LightGBMPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("broken")))
    monkeypatch.setattr(algo, "_inference", MagicMock(side_effect=RuntimeError("inference broken")))

    result = None
    raised = False
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        raised = True
    # Either raised (correct) or returned None — must NOT return a fake EMA result
    assert raised or result is None, "predict() must not silently return EMA on failure"


# ---------------------------------------------------------------------------
# 7. XGBoost — predict() must raise when _train_and_predict fails
# ---------------------------------------------------------------------------

def test_xgboost_raises_when_train_fails(monkeypatch):
    pytest.importorskip("xgboost", reason="xgboost not installed")
    from src.algorithms.xgboost_model import XGBoostPredictor

    algo = XGBoostPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=OSError("libgomp1 missing")))

    with pytest.raises(OSError, match="libgomp1 missing"):
        algo.predict(_make_prices(100))


def test_xgboost_no_fake_ema_on_train_failure(monkeypatch):
    pytest.importorskip("xgboost", reason="xgboost not installed")
    from src.algorithms.xgboost_model import XGBoostPredictor

    algo = XGBoostPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=OSError("libgomp1 missing")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 8. Random Forest — predict() must raise when _train_and_predict fails
# ---------------------------------------------------------------------------

def test_random_forest_raises_when_train_fails(monkeypatch):
    pytest.importorskip("sklearn", reason="scikit-learn not installed")
    from src.algorithms.random_forest import RandomForestPredictor

    algo = RandomForestPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("sklearn boom")))

    with pytest.raises(RuntimeError, match="sklearn boom"):
        algo.predict(_make_prices(100))


def test_random_forest_no_fake_ema_on_failure(monkeypatch):
    pytest.importorskip("sklearn", reason="scikit-learn not installed")
    from src.algorithms.random_forest import RandomForestPredictor

    algo = RandomForestPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_train_and_predict", MagicMock(side_effect=RuntimeError("sklearn boom")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 9. RL DQN — predict() must raise when _inference fails
# ---------------------------------------------------------------------------

def test_rl_dqn_raises_on_inference_failure(monkeypatch):
    pytest.importorskip("torch", reason="PyTorch not installed")
    from src.algorithms.rl_dqn import RLDQNPredictor

    algo = RLDQNPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_inference", MagicMock(side_effect=RuntimeError("dqn broken")))

    with pytest.raises(RuntimeError, match="dqn broken"):
        algo.predict(_make_prices(100))


def test_rl_dqn_no_fake_ema_on_failure(monkeypatch):
    pytest.importorskip("torch", reason="PyTorch not installed")
    from src.algorithms.rl_dqn import RLDQNPredictor

    algo = RLDQNPredictor()
    _set_market(algo)
    monkeypatch.setattr(algo, "_inference", MagicMock(side_effect=RuntimeError("dqn broken")))

    result = None
    try:
        result = algo.predict(_make_prices(100))
    except Exception:
        pass
    assert result is None


# ---------------------------------------------------------------------------
# 10. Orchestrator — failing algo is skipped; other algos still produce rows
# ---------------------------------------------------------------------------

class _MockPriceRow:
    """Minimal price row object for GOLD (has buy_price)."""
    def __init__(self, price: float):
        self.buy_price = price


def _make_price_rows(n: int = 30) -> list[_MockPriceRow]:
    return [_MockPriceRow(100.0 + i * 0.5) for i in range(n)]


def test_orchestrator_gold_skips_failing_algo_writes_others(monkeypatch):
    """When one algo raises, its DB row is NOT written; the passing algo IS written."""
    from src.orchestrator.runner import _predict_gold, _noop, GOLD_INSTRUMENTS

    price_rows = _make_price_rows(30)

    success_algo = MagicMock()
    success_algo.predict.return_value = _make_result("success", 100.5)

    fail_algo = MagicMock()
    fail_algo.predict.side_effect = RuntimeError("real compute exploded")

    algos = {"success": success_algo, "fail_algo": fail_algo}

    created_rows = []

    monkeypatch.setattr(
        "src.orchestrator.runner.repo.get_gold_prices_asc",
        lambda *a, **kw: price_rows,
    )
    monkeypatch.setattr(
        "src.orchestrator.runner.repo.create_gold_prediction",
        lambda **kw: created_rows.append(kw),
    )
    monkeypatch.setattr(
        "src.orchestrator.runner.repo.get_direction_accuracy",
        lambda market: {},
    )
    # Disable per-symbol pass
    settings_mock = MagicMock()
    settings_mock.per_symbol_enabled = False
    monkeypatch.setattr("src.orchestrator.runner.get_settings", lambda: settings_mock)

    count = _predict_gold(algos, _noop)

    n_instruments = len(GOLD_INSTRUMENTS)
    # Only success algo writes — n_instruments rows
    assert count == n_instruments
    assert len(created_rows) == n_instruments
    # Verify no row belongs to the failing algo
    algo_names_written = {r["algorithm_name"] for r in created_rows}
    assert "fail_algo" not in algo_names_written
    assert "success" in algo_names_written


def test_orchestrator_gold_all_algos_fail_returns_zero(monkeypatch):
    """When ALL algos fail, count == 0 and no rows are written."""
    from src.orchestrator.runner import _predict_gold, _noop

    price_rows = _make_price_rows(30)

    fail_a = MagicMock()
    fail_a.predict.side_effect = RuntimeError("fail a")
    fail_b = MagicMock()
    fail_b.predict.side_effect = RuntimeError("fail b")
    algos = {"fail_a": fail_a, "fail_b": fail_b}

    created_rows = []

    monkeypatch.setattr(
        "src.orchestrator.runner.repo.get_gold_prices_asc",
        lambda *a, **kw: price_rows,
    )
    monkeypatch.setattr(
        "src.orchestrator.runner.repo.create_gold_prediction",
        lambda **kw: created_rows.append(kw),
    )
    monkeypatch.setattr(
        "src.orchestrator.runner.repo.get_direction_accuracy",
        lambda market: {},
    )
    settings_mock = MagicMock()
    settings_mock.per_symbol_enabled = False
    monkeypatch.setattr("src.orchestrator.runner.get_settings", lambda: settings_mock)

    count = _predict_gold(algos, _noop)

    assert count == 0
    assert created_rows == []


def test_orchestrator_gold_insufficient_data_skips_instrument(monkeypatch):
    """When an instrument has <20 price points, all algos are skipped for it."""
    from src.orchestrator.runner import _predict_gold, _noop

    short_rows = _make_price_rows(5)  # < 20 points

    success_algo = MagicMock()
    success_algo.predict.return_value = _make_result("success")
    algos = {"success": success_algo}

    created_rows = []

    monkeypatch.setattr(
        "src.orchestrator.runner.repo.get_gold_prices_asc",
        lambda *a, **kw: short_rows,
    )
    monkeypatch.setattr(
        "src.orchestrator.runner.repo.create_gold_prediction",
        lambda **kw: created_rows.append(kw),
    )
    monkeypatch.setattr(
        "src.orchestrator.runner.repo.get_direction_accuracy",
        lambda market: {},
    )
    settings_mock = MagicMock()
    settings_mock.per_symbol_enabled = False
    monkeypatch.setattr("src.orchestrator.runner.get_settings", lambda: settings_mock)

    count = _predict_gold(algos, _noop)

    assert count == 0
    assert created_rows == []
    # predict was never called (data guard fired first)
    assert success_algo.predict.call_count == 0
