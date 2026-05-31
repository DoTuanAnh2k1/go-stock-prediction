"""Algorithm registry — builds and returns all algorithm instances."""
from __future__ import annotations

from src.algorithms.arima_garch import ARIMAGARCHPredictor
from src.algorithms.base import PredictionAlgorithm
from src.algorithms.ema_macd import EMAMACDPredictor
from src.algorithms.ensemble import EnsemblePredictor
from src.algorithms.lightgbm_model import LightGBMPredictor
from src.algorithms.lstm import LSTMPredictor
from src.algorithms.moving_average import MovingAveragePredictor


def build_algorithms() -> dict[str, PredictionAlgorithm]:
    """Build and return all algorithms keyed by their DB key.

    Two-pass: build base algorithms first, then composites (Ensemble).
    """
    ma = MovingAveragePredictor()
    ema = EMAMACDPredictor()
    lstm = LSTMPredictor()
    arima = ARIMAGARCHPredictor()
    lgbm = LightGBMPredictor()
    ensemble = EnsemblePredictor([ma, ema, lstm, arima, lgbm])

    return {
        ma.get_key(): ma,
        ema.get_key(): ema,
        lstm.get_key(): lstm,
        arima.get_key(): arima,
        lgbm.get_key(): lgbm,
        ensemble.get_key(): ensemble,
    }
