"""VWMA (Volume-Weighted Moving Average) prediction algorithm.

Port of the Go MovingAveragePredictor, translated to NumPy-vectorized operations.
Step 2 improvement: predict actual price via VWMA trend slope projection +
RSI momentum scaling instead of the old BUY/SELL signal → tiny %.
Step 3 improvement: Stochastic RSI overlay for overbought/oversold adjustment.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct

# Try to import pandas_ta for StochRSI; graceful fallback if absent
try:
    import pandas as pd
    import pandas_ta as ta  # noqa: F401
    _PANDAS_TA_AVAILABLE = True
except ImportError:
    _PANDAS_TA_AVAILABLE = False


class MovingAveragePredictor(PredictionAlgorithm):
    """Volume-Weighted Moving Average predictor with RSI, Bollinger Bands and Momentum."""

    SHORT_PERIOD = 5   # 1 trading week
    LONG_PERIOD = 20   # 1 trading month

    def get_name(self) -> str:
        return "Volume-Weighted Moving Average"

    def get_key(self) -> str:
        return "moving_average"

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < self.LONG_PERIOD:
            raise ValueError(f"Need at least {self.LONG_PERIOD} price points, got {len(prices)}")

        arr = np.array(prices, dtype=float)
        vol_arr = np.array(volumes, dtype=float) if volumes and len(volumes) == len(prices) else None

        current = float(arr[-1])

        # Compute VWMA series for trend slope extraction
        short_ma = self._vwma(arr, vol_arr, self.SHORT_PERIOD)
        long_ma = self._vwma(arr, vol_arr, self.LONG_PERIOD)

        # RSI for momentum scaling
        rsi = self.calc_rsi(arr)

        confidence = self._calc_confidence(short_ma, long_ma)
        predicted_price = self._calc_predicted_price(arr, vol_arr, current, rsi)

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

    def _vwma_series(self, prices: np.ndarray, volumes: np.ndarray | None, period: int) -> np.ndarray:
        """Compute a rolling VWMA series (length = len(prices) - period + 1)."""
        result = []
        for i in range(period - 1, len(prices)):
            wp = prices[i - period + 1 : i + 1]
            if volumes is not None and len(volumes) > i:
                wv = volumes[i - period + 1 : i + 1]
                wv = np.where(wv > 0, wv, 1.0)
                result.append(float(np.sum(wp * wv) / np.sum(wv)))
            else:
                result.append(float(np.mean(wp)))
        return np.array(result, dtype=float)

    def _calc_confidence(self, short_ma: float, long_ma: float) -> float:
        """Derive confidence from MA separation magnitude."""
        if long_ma == 0:
            return 0.50
        ma_diff_pct = abs(short_ma - long_ma) / long_ma
        # Stronger divergence → higher confidence, capped at 0.85
        confidence = 0.40 + min(ma_diff_pct * 10, 0.45)
        return round(max(0.30, min(0.85, confidence)), 4)

    def _calc_predicted_price(
        self,
        arr: np.ndarray,
        vol_arr: np.ndarray | None,
        current: float,
        rsi: float,
    ) -> float:
        """Predict next price using VWMA trend slope + RSI momentum scaling.

        Strategy:
          1. Build a rolling VWMA series and measure its recent slope (last N steps).
          2. Extrapolate 1 period forward: predicted = current + slope.
          3. Scale by RSI momentum:
               RSI > 60 → bullish boost (0.8 … 1.4 multiplier on slope)
               RSI < 40 → bearish (0.8 … 1.4 multiplier on slope in negative direction)
               40-60    → neutral (slope unmodified, slight dampen)
          4. Apply StochRSI overlay for overbought/oversold mean-reversion bias:
               StochRSI %K > 80 (overbought) → reduce bullish / increase bearish bias
               StochRSI %K < 20 (oversold)   → increase bullish / reduce bearish bias
          5. Apply market-aware clamp.
        """
        slope_window = min(5, self.SHORT_PERIOD)
        vwma_series = self._vwma_series(arr, vol_arr, self.SHORT_PERIOD)

        if len(vwma_series) >= slope_window:
            recent = vwma_series[-slope_window:]
            # Least-squares slope (per period)
            x = np.arange(len(recent), dtype=float)
            slope = float(np.polyfit(x, recent, 1)[0])
        else:
            # Fallback: simple difference
            slope = float(vwma_series[-1] - vwma_series[-2]) if len(vwma_series) >= 2 else 0.0

        # RSI momentum multiplier
        if rsi > 60:
            # Bullish zone: amplify upward slope, dampen downward
            momentum_mult = 0.8 + (rsi - 60) / 40 * 0.8   # 0.8 … 1.6 range → clamp
            momentum_mult = min(1.4, momentum_mult)
        elif rsi < 40:
            # Bearish zone: amplify downward slope, dampen upward
            momentum_mult = 0.8 + (40 - rsi) / 40 * 0.8
            momentum_mult = min(1.4, momentum_mult)
            slope = -abs(slope) if slope > 0 else slope   # bias towards down
        else:
            # Neutral: slight dampening
            momentum_mult = 0.6

        predicted = current + slope * momentum_mult

        # StochRSI overlay — mean-reversion adjustment
        stoch_k = self._calc_stochrsi_k(arr)
        if stoch_k is not None:
            if stoch_k > 80:
                # Overbought: nudge prediction toward current (dampen bullish overshoot)
                overbought_factor = (stoch_k - 80) / 20   # 0 … 1
                predicted = predicted - (predicted - current) * overbought_factor * 0.3
            elif stoch_k < 20:
                # Oversold: nudge prediction upward toward current (dampen bearish overshoot)
                oversold_factor = (20 - stoch_k) / 20   # 0 … 1
                predicted = predicted + (current - predicted) * oversold_factor * 0.3

        # Market-aware clamp
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))

        if predicted <= 0:
            predicted = current * 0.01

        return predicted

    @staticmethod
    def _calc_stochrsi_k(arr: np.ndarray, length: int = 14) -> float | None:
        """Return StochRSI %K using pandas_ta, or None if unavailable/insufficient data."""
        if not _PANDAS_TA_AVAILABLE or len(arr) < length * 2 + 5:
            return None
        try:
            import pandas as pd
            import pandas_ta as ta  # noqa: F811

            close = pd.Series(arr, dtype=float)
            df = ta.stochrsi(close, length=length, rsi_length=length, k=3, d=3)
            if df is None or df.empty:
                return None
            k_col = df.iloc[:, 0]
            last_k = k_col.dropna().iloc[-1] if not k_col.dropna().empty else None
            if last_k is None or (hasattr(last_k, "__float__") and float(last_k) != float(last_k)):
                return None
            return float(last_k)
        except Exception:
            return None

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
