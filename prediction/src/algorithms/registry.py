"""Algorithm registry — builds and returns all algorithm instances."""
from __future__ import annotations

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


def build_algorithms() -> dict[str, PredictionAlgorithm]:
    """Build and return all algorithms keyed by their DB key.

    Two-pass: build base algorithms first, then composites (Ensemble).
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

    return {
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
