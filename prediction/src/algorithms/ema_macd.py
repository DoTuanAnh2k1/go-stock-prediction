"""EMA/MACD prediction algorithm.

Port of the Go EMAPredictor — MACD(12, 26, 9) with Vietnamese session adjustment.
"""
from __future__ import annotations

import random
import time
from typing import Optional

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult


class EMAMACDPredictor(PredictionAlgorithm):
    """MACD-based predictor using EMA(12), EMA(26), Signal(9)."""

    SHORT_PERIOD = 12
    LONG_PERIOD = 26
    SIGNAL_PERIOD = 9
    MAX_DAILY_CHANGE = 0.07

    def get_name(self) -> str:
        return "Exponential Moving Average"

    def get_key(self) -> str:
        return "ema"

    def predict(self, prices: list[float], volumes: Optional[list[float]] = None) -> PredictionResult:
        if len(prices) < self.LONG_PERIOD:
            raise ValueError(f"Need at least {self.LONG_PERIOD} price points, got {len(prices)}")

        arr = np.array(prices, dtype=float)
        current = float(arr[-1])

        short_ema = self._ema_scalar(arr, self.SHORT_PERIOD)
        long_ema = self._ema_scalar(arr, self.LONG_PERIOD)

        short_series = self._ema_series(arr, self.SHORT_PERIOD)
        long_series = self._ema_series(arr, self.LONG_PERIOD)

        overlap = len(long_series)
        short_offset = len(short_series) - overlap
        macd_series = short_series[short_offset:] - long_series

        macd_line = short_ema - long_ema

        if len(macd_series) >= self.SIGNAL_PERIOD:
            signal_line = self._ema_scalar(macd_series, self.SIGNAL_PERIOD)
        else:
            signal_line = float(np.mean(macd_series)) if len(macd_series) > 0 else 0.0

        histogram = macd_line - signal_line

        signal = self._macd_signal(macd_line, signal_line, histogram)
        confidence = self._adjust_for_vn_session(signal)
        predicted = self._calc_price(current, signal, confidence)

        return PredictionResult(
            predicted_price=predicted,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    # ------------------------------------------------------------------

    @staticmethod
    def _ema_scalar(prices: np.ndarray, period: int) -> float:
        if len(prices) < period:
            return 0.0
        k = 2.0 / (period + 1)
        ema = float(np.mean(prices[:period]))
        for p in prices[period:]:
            ema = float(p) * k + ema * (1 - k)
        return ema

    @staticmethod
    def _ema_series(prices: np.ndarray, period: int) -> np.ndarray:
        if len(prices) < period:
            return np.array([])
        k = 2.0 / (period + 1)
        result_len = len(prices) - period + 1
        result = np.empty(result_len)
        result[0] = np.mean(prices[:period])
        for i in range(1, result_len):
            result[i] = prices[period - 1 + i] * k + result[i - 1] * (1 - k)
        return result

    @staticmethod
    def _macd_signal(macd_line: float, signal_line: float, histogram: float) -> dict:
        direction = "HOLD"
        if macd_line > signal_line and histogram > 0:
            direction = "BUY"
        elif macd_line < signal_line and histogram < 0:
            direction = "SELL"

        if macd_line != 0:
            strength = min(1.0, abs(histogram) / abs(macd_line) * 2 + 0.5)
        else:
            strength = 0.5

        strength = max(0.1, min(1.0, strength))
        return {"direction": direction, "strength": strength}

    @staticmethod
    def _adjust_for_vn_session(signal: dict) -> float:
        base = signal["strength"] * 0.75
        hour = time.localtime().tm_hour

        if 9 <= hour <= 11:
            adj = 0.10
        elif 13 <= hour <= 14:
            adj = 0.05
        else:
            adj = -0.05

        return max(0.3, min(0.9, base + adj))

    def _calc_price(self, current: float, signal: dict, confidence: float) -> float:
        direction = signal["direction"]
        strength = signal["strength"]

        if direction == "BUY":
            movement = current * strength * 0.025 * confidence
        elif direction == "SELL":
            movement = -current * strength * 0.025 * confidence
        else:
            movement = current * (strength - 0.5) * 0.005

        noise = (random.random() - 0.5) * 2 * current * 0.001 * (1 - confidence)
        predicted = current + movement + noise

        max_change = current * self.MAX_DAILY_CHANGE
        predicted = max(current - max_change, min(current + max_change, predicted))
        if predicted <= 0:
            predicted = current * 0.01
        return predicted
