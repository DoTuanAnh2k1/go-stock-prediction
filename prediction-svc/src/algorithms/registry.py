"""Algorithm registry — builds and returns all algorithm instances.

Supports two caching layers:
1. Per-market (pooled): each market gets one set of algorithm instances trained
   on ALL symbols for that market combined.  Original behaviour, always active.
2. Per-symbol: each (market, symbol) pair gets its own set of algorithm instances
   trained on that symbol's prices only.  Activated when
   ``settings.per_symbol_enabled`` is True.  Predictions are written to DB with
   the ``__ps`` suffix appended to the algorithm name so pooled and per-symbol
   rows coexist in the same table and are distinguished by consumers.
"""
from __future__ import annotations

import threading

from src.algorithms.arima_garch import ARIMAGARCHPredictor
from src.algorithms.base import PredictionAlgorithm
from src.algorithms.egarch import EGARCHPredictor
from src.algorithms.ema_macd import EMAMACDPredictor
from src.algorithms.ensemble import EnsemblePredictor
from src.algorithms.gru import GRUPredictor
from src.algorithms.lightgbm_model import LightGBMPredictor
from src.algorithms.lstm import LSTMPredictor
from src.algorithms.moving_average import MovingAveragePredictor
from src.algorithms.random_forest import RandomForestPredictor
from src.algorithms.transformer_model import TransformerPredictor
from src.algorithms.rl_dqn import RLDQNPredictor
from src.algorithms.sarima import SARIMAPredictor
from src.algorithms.xgboost_model import XGBoostPredictor

# ---------------------------------------------------------------------------
# Per-symbol suffix — appended to algorithm_name for per-symbol predictions
# ---------------------------------------------------------------------------

PS_SUFFIX = "__ps"


def is_ps(name: str) -> bool:
    """Return True if *name* is a per-symbol algorithm name (has PS_SUFFIX)."""
    return name.endswith(PS_SUFFIX)


def base_key(name: str) -> str:
    """Strip PS_SUFFIX from *name* if present, otherwise return unchanged."""
    return name[: -len(PS_SUFFIX)] if is_ps(name) else name


# ---------------------------------------------------------------------------
# Per-algo minimum data points for per-symbol training
# ---------------------------------------------------------------------------

PER_SYMBOL_ALGO_MIN_POINTS: dict[str, int] = {
    "moving_average": 30,
    "ema": 35,
    "arima_garch": 60,
    "sarima": 80,
    "egarch": 80,
    "lightgbm": 80,
    "xgboost": 80,
    "random_forest": 80,
    "lstm_nn": 150,
    "gru_nn": 150,
    "ensemble": 150,
    "rl_dqn": 150,
}
_DEFAULT_PS_MIN_POINTS = 80

# ---------------------------------------------------------------------------
# Module-level caches and lock
# ---------------------------------------------------------------------------

# Global per-market algorithm cache: market_key → {algo_key → algo_instance}
_market_algos: dict[str, dict[str, PredictionAlgorithm]] = {}

# Global per-symbol algorithm cache: market_key → symbol_key → {algo_key → algo_instance}
_symbol_algos: dict[str, dict[str, dict[str, PredictionAlgorithm]]] = {}

_registry_lock = threading.Lock()


# ---------------------------------------------------------------------------
# Pooled (per-market) functions — UNCHANGED public contract
# ---------------------------------------------------------------------------

def build_algorithms(market_key: str | None = None) -> dict[str, PredictionAlgorithm]:
    """Build fresh algorithm instances.

    Two-pass: build base algorithms first, then composites (Ensemble).
    If market_key is provided, the fresh instances are cached for that market.
    """
    ma = MovingAveragePredictor()
    ema = EMAMACDPredictor()
    lstm = LSTMPredictor()
    arima = ARIMAGARCHPredictor()
    lgbm = LightGBMPredictor()
    sarima = SARIMAPredictor()
    egarch = EGARCHPredictor()
    gru = GRUPredictor()
    rf = RandomForestPredictor()
    xgb = XGBoostPredictor()
    # RL DQN: algorithm #12 — NOT added to Ensemble (v1 exclusion per spec)
    rl = RLDQNPredictor()
    # Transformer: algorithm #13 — NOT in Ensemble yet (same staged rollout as rl_dqn)
    tf = TransformerPredictor()
    ensemble = EnsemblePredictor([ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb])

    instances = {
        ma.get_key(): ma,
        ema.get_key(): ema,
        lstm.get_key(): lstm,
        arima.get_key(): arima,
        lgbm.get_key(): lgbm,
        sarima.get_key(): sarima,
        egarch.get_key(): egarch,
        gru.get_key(): gru,
        rf.get_key(): rf,
        xgb.get_key(): xgb,
        ensemble.get_key(): ensemble,
        rl.get_key(): rl,
        tf.get_key(): tf,
    }

    if market_key:
        mk_upper = market_key.upper()
        for algo in instances.values():
            algo._market_key = mk_upper
        with _registry_lock:
            _market_algos[mk_upper] = instances

    return instances


def get_algos_for_market(market_key: str) -> dict[str, PredictionAlgorithm]:
    """Return cached algorithm instances for a market.

    If no cached instances exist for this market, build fresh ones and cache them.
    """
    mk = market_key.upper()
    with _registry_lock:
        if mk in _market_algos:
            return _market_algos[mk]
    # Build fresh and cache
    return build_algorithms(market_key=mk)


def set_algos_for_market(market_key: str, algos: dict[str, PredictionAlgorithm]) -> None:
    """Store a set of (trained) algorithm instances for a market."""
    with _registry_lock:
        _market_algos[market_key.upper()] = algos


# ---------------------------------------------------------------------------
# Per-symbol functions — ADDITIVE, only activated when per_symbol_enabled=True
# ---------------------------------------------------------------------------

def build_algorithms_for_symbol(
    market_key: str,
    symbol_key: str,
) -> dict[str, PredictionAlgorithm]:
    """Build fresh algorithm instances for a specific (market, symbol) pair.

    Uses the same construction as :func:`build_algorithms` but sets both
    ``_market_key`` and ``_symbol_key`` on every instance, so stateful algos
    (LSTM/GRU/LightGBM/XGBoost/RF/ARIMA/SARIMA/EGARCH/RL-DQN) can isolate
    their trained state from the pooled instances.

    The freshly built dict is cached into ``_symbol_algos[market][symbol]``
    and also returned to the caller.
    """
    mk_upper = market_key.upper()

    ma = MovingAveragePredictor()
    ema = EMAMACDPredictor()
    lstm = LSTMPredictor()
    arima = ARIMAGARCHPredictor()
    lgbm = LightGBMPredictor()
    sarima = SARIMAPredictor()
    egarch = EGARCHPredictor()
    gru = GRUPredictor()
    rf = RandomForestPredictor()
    xgb = XGBoostPredictor()
    rl = RLDQNPredictor()
    tf = TransformerPredictor()
    ensemble = EnsemblePredictor([ma, ema, lstm, arima, lgbm, sarima, egarch, gru, rf, xgb])

    instances: dict[str, PredictionAlgorithm] = {
        ma.get_key(): ma,
        ema.get_key(): ema,
        lstm.get_key(): lstm,
        arima.get_key(): arima,
        lgbm.get_key(): lgbm,
        sarima.get_key(): sarima,
        egarch.get_key(): egarch,
        gru.get_key(): gru,
        rf.get_key(): rf,
        xgb.get_key(): xgb,
        ensemble.get_key(): ensemble,
        rl.get_key(): rl,
        tf.get_key(): tf,
    }

    for algo in instances.values():
        algo._market_key = mk_upper
        algo._symbol_key = symbol_key

    with _registry_lock:
        if mk_upper not in _symbol_algos:
            _symbol_algos[mk_upper] = {}
        _symbol_algos[mk_upper][symbol_key] = instances

    return instances


def get_algos_for_symbol(
    market_key: str,
    symbol_key: str,
) -> dict[str, PredictionAlgorithm]:
    """Return cached algorithm instances for a (market, symbol) pair.

    If no cached instances exist, build fresh ones and cache them.
    Note: the returned instances are NOT yet trained — callers that want
    trained-only instances should use :func:`get_trained_algos_for_symbol`.
    """
    mk = market_key.upper()
    with _registry_lock:
        market_cache = _symbol_algos.get(mk, {})
        if symbol_key in market_cache:
            return market_cache[symbol_key]
    # Build fresh and cache
    return build_algorithms_for_symbol(mk, symbol_key)


def set_algos_for_symbol(
    market_key: str,
    symbol_key: str,
    algos: dict[str, PredictionAlgorithm],
) -> None:
    """Store a set of (trained) algorithm instances for a (market, symbol) pair."""
    mk = market_key.upper()
    with _registry_lock:
        if mk not in _symbol_algos:
            _symbol_algos[mk] = {}
        _symbol_algos[mk][symbol_key] = algos


def has_symbol_algos(market_key: str, symbol_key: str) -> bool:
    """Return True if cached algo instances exist for this (market, symbol) pair."""
    mk = market_key.upper()
    with _registry_lock:
        return symbol_key in _symbol_algos.get(mk, {})


def get_trained_algos_for_symbol(
    market_key: str,
    symbol_key: str,
) -> dict[str, PredictionAlgorithm] | None:
    """Return cached trained algo dict for a (market, symbol) pair, or None.

    Used by the hot prediction path.  This function NEVER triggers training;
    if no cached instances exist for this symbol, it returns None and the
    caller should skip that symbol (cold-start behaviour).

    The returned dict contains ONLY the algos that survived the training step
    (data-starved entries are removed by ``train_per_symbol_for_market``).
    """
    mk = market_key.upper()
    with _registry_lock:
        market_cache = _symbol_algos.get(mk, {})
        return market_cache.get(symbol_key, None)
