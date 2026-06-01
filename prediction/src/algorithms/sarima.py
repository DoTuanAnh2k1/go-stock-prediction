"""SARIMA prediction algorithm.

Uses statsmodels SARIMAX with order=(1,1,1) and seasonal_order=(1,0,1,5).
Falls back gracefully to EMA if fitting fails.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("sarima")

MIN_DATA_POINTS = 60  # SARIMA needs at least 60 points to fit reliably


class SARIMAPredictor(PredictionAlgorithm):
    """SARIMAX(1,1,1)(1,0,1,5) predictor."""

    def get_name(self) -> str:
        return "SARIMA"

    def get_key(self) -> str:
        return "sarima"

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"SARIMA needs {MIN_DATA_POINTS} points, got {len(prices)}")

        current = float(prices[-1])
        try:
            return self._fit_and_predict(prices, current)
        except Exception as exc:
            log.warning("sarima.fallback", error=str(exc))
            return self._ema_fallback(prices, current)

    def _fit_and_predict(self, prices: list[float], current: float) -> PredictionResult:
        import warnings

        import pandas as pd
        from statsmodels.tsa.statespace.sarimax import SARIMAX

        arr = np.array(prices, dtype=float)
        series = pd.Series(arr)

        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            model = SARIMAX(
                series,
                order=(1, 1, 1),
                seasonal_order=(1, 0, 1, 5),
                enforce_stationarity=False,
                enforce_invertibility=False,
            )
            model_fit = model.fit(disp=False, method="lbfgs", maxiter=200)

        forecast_summary = model_fit.get_forecast(steps=1)
        forecast = float(forecast_summary.predicted_mean.iloc[0])

        # Clamp to ±7% daily limit
        max_change = current * 0.07
        forecast = max(current - max_change, min(current + max_change, forecast))

        # Confidence from forecast standard error
        try:
            stderr = forecast_summary.summary_frame()["mean_se"].iloc[0]
            confidence = max(0.3, min(0.9, 1.0 / (1.0 + stderr / current * 10)))
        except Exception:
            confidence = 0.37

        return PredictionResult(
            predicted_price=forecast,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    @staticmethod
    def _ema_fallback(prices: list[float], current: float) -> PredictionResult:
        arr = np.array(prices, dtype=float)
        period = min(26, len(arr))
        k = 2.0 / (period + 1)
        ema = float(np.mean(arr[:period]))
        for p in arr[period:]:
            ema = float(p) * k + ema * (1 - k)
        trend = (ema - current) / current
        predicted = current * (1 + trend * 0.5)
        max_change = current * 0.07
        predicted = max(current - max_change, min(current + max_change, predicted))
        return PredictionResult(
            predicted_price=predicted,
            confidence=0.37,
            current_price=current,
            algorithm_name="sarima",
        )
