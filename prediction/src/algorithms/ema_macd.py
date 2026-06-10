"""EMA/MACD prediction algorithm.

Port of the Go EMAPredictor — MACD(12, 26, 9) with Vietnamese session adjustment.
Step 2 improvement: predict actual price via EMA trend slope + MACD momentum
instead of the old BUY/SELL signal → tiny %.
Step 3 improvement: Bollinger %B overlay for mean-reversion bias near band extremes.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct

# Try to import pandas_ta for Bollinger %B; graceful fallback if absent
try:
    import pandas as pd
    import pandas_ta as ta  # noqa: F401
    _PANDAS_TA_AVAILABLE = True
except ImportError:
    _PANDAS_TA_AVAILABLE = False


class EMAMACDPredictor(PredictionAlgorithm):
    """MACD-based predictor using EMA(12), EMA(26), Signal(9)."""

    SHORT_PERIOD = 12
    LONG_PERIOD = 26
    SIGNAL_PERIOD = 9

    def get_name(self) -> str:
        return "Exponential Moving Average"

    def get_key(self) -> str:
        return "ema"

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < self.LONG_PERIOD:
            raise ValueError(f"Need at least {self.LONG_PERIOD} price points, got {len(prices)}")

        arr = np.array(prices, dtype=float)
        current = float(arr[-1])

        # Build full EMA series for slope calculation
        short_series = self._ema_series(arr, self.SHORT_PERIOD)
        long_series = self._ema_series(arr, self.LONG_PERIOD)

        # Scalar EMA values (last point of each series)
        short_ema = float(short_series[-1]) if len(short_series) > 0 else current
        long_ema = float(long_series[-1]) if len(long_series) > 0 else current

        # MACD line and signal line
        overlap = len(long_series)
        short_offset = len(short_series) - overlap
        macd_series = short_series[short_offset:] - long_series

        macd_line = short_ema - long_ema

        if len(macd_series) >= self.SIGNAL_PERIOD:
            signal_line = self._ema_scalar(macd_series, self.SIGNAL_PERIOD)
        else:
            signal_line = float(np.mean(macd_series)) if len(macd_series) > 0 else 0.0

        histogram = macd_line - signal_line

        confidence = self._calc_confidence(macd_line, signal_line, histogram)
        predicted = self._calc_price(arr, current, short_series, macd_line, signal_line)

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
    def _calc_confidence(macd_line: float, signal_line: float, histogram: float) -> float:
        """Confidence based on MACD histogram size relative to MACD magnitude."""
        if macd_line != 0:
            strength = min(1.0, abs(histogram) / abs(macd_line) * 2 + 0.5)
        else:
            strength = 0.5
        strength = max(0.1, min(1.0, strength))
        # Map to [0.30, 0.85] confidence range
        return round(max(0.30, min(0.85, strength * 0.75)), 4)

    def _calc_price(
        self,
        arr: np.ndarray,
        current: float,
        short_ema_series: np.ndarray,
        macd_line: float,
        signal_line: float,
    ) -> float:
        """Predict next price using EMA slope + MACD momentum boost + Bollinger %B bias.

        Strategy:
          1. Compute the EMA(12) slope from the last few values of the series.
          2. MACD momentum: if MACD > signal → upward boost proportional to
             (macd - signal) / current; if MACD < signal → downward.
          3. predicted = current + ema_slope + macd_boost
          4. Bollinger %B mean-reversion overlay:
               %B > 0.9 (near upper band) → slight bearish nudge
               %B < 0.1 (near lower band) → slight bullish nudge
          5. Apply market-aware clamp.
        """
        # EMA slope: average change over last min(3, available) steps
        slope_window = min(4, len(short_ema_series))
        if slope_window >= 2:
            recent = short_ema_series[-slope_window:]
            x = np.arange(len(recent), dtype=float)
            ema_slope = float(np.polyfit(x, recent, 1)[0])
        else:
            ema_slope = 0.0

        # MACD momentum: normalise by current price to get a price-unit contribution
        if current > 0:
            macd_momentum = (macd_line - signal_line) / current * current * 0.5
        else:
            macd_momentum = 0.0

        predicted = current + ema_slope + macd_momentum

        # Bollinger %B overlay — mean-reversion adjustment
        bb_pct = self._calc_bb_pct(arr)
        if bb_pct is not None:
            if bb_pct > 0.9:
                # Near upper band: bearish mean-reversion nudge
                extreme_factor = min(1.0, (bb_pct - 0.9) / 0.1)   # 0 … 1
                # Nudge predicted toward (or below) current
                predicted = predicted - (predicted - current) * extreme_factor * 0.2
            elif bb_pct < 0.1:
                # Near lower band: bullish mean-reversion nudge
                extreme_factor = min(1.0, (0.1 - bb_pct) / 0.1)   # 0 … 1
                # Nudge predicted toward (or above) current
                predicted = predicted + (current - predicted) * extreme_factor * 0.2

        # Market-aware clamp
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))
        if predicted <= 0:
            predicted = current * 0.01
        return predicted

    @staticmethod
    def _calc_bb_pct(arr: np.ndarray, length: int = 20) -> float | None:
        """Return Bollinger %B for the last price using pandas_ta, or None if unavailable."""
        if not _PANDAS_TA_AVAILABLE or len(arr) < length + 5:
            return None
        try:
            import pandas as pd
            import pandas_ta as ta  # noqa: F811

            close = pd.Series(arr, dtype=float)
            bb_df = ta.bbands(close, length=length, std=2.0)
            if bb_df is None or bb_df.empty:
                return None
            # %B column starts with "BBP"
            pct_b_cols = [c for c in bb_df.columns if c.startswith("BBP")]
            if not pct_b_cols:
                return None
            pct_b_series = bb_df[pct_b_cols[0]].dropna()
            if pct_b_series.empty:
                return None
            val = float(pct_b_series.iloc[-1])
            return val if not (val != val) else None   # NaN check
        except Exception:
            return None
