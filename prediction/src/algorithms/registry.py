"""Algorithm registry — builds and returns all algorithm instances.

Supports per-market instance caching: each market gets its own set of algorithm
instances so that trained models are isolated and not shared across markets.
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
from src.algorithms.sarima import SARIMAPredictor
from src.algorithms.xgboost_model import XGBoostPredictor

# Global per-market algorithm cache: market_key → {algo_key → algo_instance}
_market_algos: dict[str, dict[str, PredictionAlgorithm]] = {}
_registry_lock = threading.Lock()


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
    }

    if market_key:
        with _registry_lock:
            _market_algos[market_key.upper()] = instances

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
