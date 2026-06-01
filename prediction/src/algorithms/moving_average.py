"""VWMA (Volume-Weighted Moving Average) prediction algorithm.

Port of the Go MovingAveragePredictor, translated to NumPy-vectorized operations.
"""
from __future__ import annotations

import random
import time

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult


class MovingAveragePredictor(PredictionAlgorithm):
    """Volume-Weighted Moving Average predictor with RSI, Bollinger Bands and Momentum."""

    SHORT_PERIOD = 5   # 1 trading week
    LONG_PERIOD = 20   # 1 trading month
    MAX_DAILY_CHANGE = 0.07  # 7% HOSE daily limit

    def get_name(self) -> str:
        return "Volume-Weighted Moving Average"

    def get_key(self) -> str:
        return "moving_average"

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < self.LONG_PERIOD:
            raise ValueError(f"Need at least {self.LONG_PERIOD} price points, got {len(prices)}")

        arr = np.array(prices, dtype=float)
        vol_arr = np.array(volumes, dtype=float) if volumes and len(volumes) == len(prices) else None

        short_ma = self._vwma(arr, vol_arr, self.SHORT_PERIOD)
        long_ma = self._vwma(arr, vol_arr, self.LONG_PERIOD)
        current = float(arr[-1])

        signal = self._generate_signal(short_ma, long_ma, current)
        confidence = self._adjust_for_vn_session(signal)
        predicted_price = self._calc_predicted_price(current, signal, confidence)

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _vwma(self, prices: np.ndarray, volumes: np.ndarray | None, period: int) -> float:
        if len(prices) < period:
            return float(np.mean(prices[-period:]))
        window_prices = prices[-period:]
        if volumes is not None and len(volumes) >= period:
            window_volumes = volumes[-period:]
            # Replace zero volumes with 1 to avoid division-by-zero
            window_volumes = np.where(window_volumes > 0, window_volumes, 1.0)
            return float(np.sum(window_prices * window_volumes) / np.sum(window_volumes))
        return float(np.mean(window_prices))

    def _generate_signal(self, short_ma: float, long_ma: float, current: float) -> dict:
        direction = "HOLD"
        strength = 0.5

        if short_ma == 0 or long_ma == 0:
            return {"direction": direction, "strength": strength}

        ma_diff_pct = (short_ma - long_ma) / long_ma * 100

        if short_ma > long_ma:
            direction = "BUY"
            strength = min(1.0, 0.5 + abs(ma_diff_pct) / 10)
        elif short_ma < long_ma:
            direction = "SELL"
            strength = min(1.0, 0.5 + abs(ma_diff_pct) / 10)

        # Adjust based on current price vs MAs
        if current > short_ma and current > long_ma:
            if direction == "BUY":
                strength *= 1.2
            else:
                strength *= 0.8
        elif current < short_ma and current < long_ma:
            if direction == "SELL":
                strength *= 1.2
            else:
                strength *= 0.8

        strength = max(0.1, min(1.0, strength))
        return {"direction": direction, "strength": strength}

    def _adjust_for_vn_session(self, signal: dict) -> float:
        base_confidence = signal["strength"] * 0.75
        hour = time.localtime().tm_hour

        if 9 <= hour <= 11:
            time_adj = 0.10
        elif 13 <= hour <= 14:
            time_adj = 0.05
        else:
            time_adj = -0.05

        confidence = base_confidence + time_adj
        return max(0.3, min(0.9, confidence))

    def _calc_predicted_price(self, current: float, signal: dict, confidence: float) -> float:
        direction = signal["direction"]
        strength = signal["strength"]

        if direction == "BUY":
            movement = current * strength * 0.02 * confidence
        elif direction == "SELL":
            movement = -current * strength * 0.02 * confidence
        else:
            movement = current * (strength - 0.5) * 0.005

        noise_range = current * 0.001 * (1 - confidence)
        noise = (random.random() - 0.5) * 2 * noise_range

        predicted = current + movement + noise

        max_change = current * self.MAX_DAILY_CHANGE
        predicted = max(current - max_change, min(current + max_change, predicted))

        if predicted <= 0:
            predicted = current * 0.01

        return predicted

    # Additional indicators used by enhanced_predict and ensemble
    @staticmethod
    def calc_rsi(prices: np.ndarray, period: int = 14) -> float:
        if len(prices) < period + 1:
            return 50.0
        deltas = np.diff(prices[-period - 1:])
        gains = np.where(deltas > 0, deltas, 0.0)
        losses = np.where(deltas < 0, -deltas, 0.0)
        avg_gain = np.mean(gains)
        avg_loss = np.mean(losses)
        if avg_loss == 0:
            return 100.0
        rs = avg_gain / avg_loss
        return float(100 - 100 / (1 + rs))

    @staticmethod
    def calc_bollinger(prices: np.ndarray, period: int = 20) -> tuple[float, float, float]:
        if len(prices) < period:
            return 0.0, 0.0, 0.0
        window = prices[-period:]
        sma = float(np.mean(window))
        std = float(np.std(window))
        return sma + 2 * std, sma, sma - 2 * std
