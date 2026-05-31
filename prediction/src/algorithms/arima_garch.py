"""ARIMA-GARCH prediction algorithm.

Uses statsmodels ARIMA(2,1,2) for mean forecast and arch GARCH(1,1) for
volatility estimation. Falls back gracefully to EMA if fitting fails.
"""
from __future__ import annotations

import math
from typing import Optional

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("arima_garch")

MIN_DATA_POINTS = 50  # ARIMA needs at least 50 points to fit reliably


class ARIMAGARCHPredictor(PredictionAlgorithm):
    """ARIMA(2,1,2) + GARCH(1,1) predictor."""

    def get_name(self) -> str:
        return "ARIMA-GARCH"

    def get_key(self) -> str:
        return "arima_garch"

    def predict(self, prices: list[float], volumes: Optional[list[float]] = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"ARIMA-GARCH needs {MIN_DATA_POINTS} points, got {len(prices)}")

        current = float(prices[-1])
        try:
            return self._fit_and_predict(prices, current)
        except Exception as exc:
            log.warning("arima_garch.fallback", error=str(exc))
            return self._ema_fallback(prices, current)

    def _fit_and_predict(self, prices: list[float], current: float) -> PredictionResult:
        import warnings
        import pandas as pd
        from statsmodels.tsa.arima.model import ARIMA

        arr = np.array(prices, dtype=float)
        series = pd.Series(arr)

        # --- ARIMA(2,1,2) mean forecast ---
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            arima_model = ARIMA(series, order=(2, 1, 2))
            arima_fit = arima_model.fit(method_kwargs={"warn_convergence": False})

        forecast = float(arima_fit.forecast(steps=1).iloc[0])

        # Clamp to ±7% daily limit
        max_change = current * 0.07
        forecast = max(current - max_change, min(current + max_change, forecast))

        # --- GARCH(1,1) volatility ---
        volatility = 0.02  # default 2% volatility
        try:
            from arch import arch_model

            log_returns = np.diff(np.log(arr)) * 100  # in percent
            if len(log_returns) >= 30:
                with warnings.catch_warnings():
                    warnings.simplefilter("ignore")
                    garch = arch_model(log_returns, vol="Garch", p=1, q=1, rescale=False)
                    garch_fit = garch.fit(disp="off", show_warning=False)
                    vol_forecast = garch_fit.forecast(horizon=1)
                    variance = float(vol_forecast.variance.iloc[-1, 0])
                    if variance > 0:
                        volatility = math.sqrt(variance) / 100  # back to fraction
        except Exception as exc:
            log.debug("garch.failed", error=str(exc))

        confidence = max(0.3, min(0.9, 1.0 / (1.0 + volatility * 5)))

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
            confidence=0.38,
            current_price=current,
            algorithm_name="arima_garch",
        )
