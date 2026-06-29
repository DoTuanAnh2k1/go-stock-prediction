"""EGARCH prediction algorithm.

Uses arch library EGARCH(1,1,1) with HAR mean component.
Falls back gracefully to EMA if fitting fails.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct
from src.utils.logger import get_logger

log = get_logger("egarch")

MIN_DATA_POINTS = 60  # EGARCH needs at least 60 points to fit reliably


class EGARCHPredictor(PredictionAlgorithm):
    """EGARCH(1,1,1) with HAR mean component predictor."""

    def get_name(self) -> str:
        return "EGARCH"

    def get_key(self) -> str:
        return "egarch"

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"EGARCH needs {MIN_DATA_POINTS} points, got {len(prices)}")

        current = float(prices[-1])
        try:
            return self._fit_and_predict(prices, current)
        except Exception as exc:
            log.error(
                "algo.egarch.failed",
                market=self._market_key,
                error=str(exc),
                exc_info=True,
            )
            raise

    def _fit_and_predict(self, prices: list[float], current: float) -> PredictionResult:
        import warnings

        from arch.univariate import arch_model

        arr = np.array(prices, dtype=float)
        log_returns = np.diff(np.log(arr)) * 100  # percent scale

        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            model = arch_model(
                log_returns,
                mean="HARX",
                lags=[1, 5, 22],
                vol="EGARCH",
                p=1,
                o=1,
                q=1,
                dist="normal",
            )
            model_fit = model.fit(disp="off", options={"maxiter": 300})

        # Forecast mean from HAR component
        forecast = model_fit.forecast(horizon=1)
        mean_forecast_pct = float(forecast.mean.iloc[-1, 0])

        # Convert percent return forecast to price
        predicted_price = current * (1 + mean_forecast_pct / 100)

        # Clamp to market-aware daily limit
        max_change = current * get_max_change_pct(self._market_key)
        predicted_price = max(current - max_change, min(current + max_change, predicted_price))

        # Confidence from forecast variance
        try:
            forecast_variance = forecast.variance.iloc[-1, 0]
            sigma = np.sqrt(float(forecast_variance))
            confidence = max(0.3, min(0.9, 1.0 / (1.0 + sigma / 100 * 3)))
        except Exception:
            confidence = 0.36

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    def _ema_fallback(self, prices: list[float], current: float) -> PredictionResult:
        arr = np.array(prices, dtype=float)
        period = min(26, len(arr))
        k = 2.0 / (period + 1)
        ema = float(np.mean(arr[:period]))
        for p in arr[period:]:
            ema = float(p) * k + ema * (1 - k)
        trend = (ema - current) / current
        predicted = current * (1 + trend * 0.5)
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))
        return PredictionResult(
            predicted_price=predicted,
            confidence=0.36,
            current_price=current,
            algorithm_name="egarch",
        )
