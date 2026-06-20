"""Random Forest prediction algorithm.

Features: enhanced ~30-feature set via features.build_enhanced_features()
  (lag returns 1-10, RSI, StochRSI, Bollinger %B, MACD, volatility, ROC,
  momentum, MA ratios 5/10/20/50, multi-timeframe returns, volume ratio).
Target: next-day log return (regression).
Supports per-market model caching via train(). When a cached model exists,
predict() uses it directly without refitting.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct
from src.algorithms.features import build_enhanced_features, build_basic_features
from src.utils.logger import get_logger

log = get_logger("random_forest")

MIN_DATA_POINTS = 80  # need enough data to build lag features


class RandomForestPredictor(PredictionAlgorithm):
    """scikit-learn RandomForestRegressor with enhanced technical feature engineering."""

    def __init__(self) -> None:
        self._model = None  # cached RandomForestRegressor

    def get_name(self) -> str:
        return "Random Forest"

    def get_key(self) -> str:
        return "random_forest"

    def is_trained(self) -> bool:
        return self._model is not None

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def train(self, prices: list[float], volumes: list[float] | None = None) -> None:
        """Pre-train model on training data and cache it."""
        if len(prices) < MIN_DATA_POINTS:
            return
        try:
            from sklearn.ensemble import RandomForestRegressor

            arr = np.array(prices, dtype=float)
            vol_arr = (
                np.array(volumes, dtype=float)
                if volumes and len(volumes) == len(prices)
                else None
            )

            features, targets = self._build_features(arr, vol_arr)
            if len(features) < 20:
                return

            X = np.array(features[:-1])
            y = np.array(targets[:-1])

            model = RandomForestRegressor(
                n_estimators=200,
                max_depth=8,
                min_samples_leaf=5,
                max_features="sqrt",
                n_jobs=-1,
                random_state=42,
            )
            model.fit(X, y)
            self._model = model
            log.info("random_forest.trained", data_points=len(prices))
        except Exception as exc:
            log.warning("random_forest.train_failed", error=str(exc))

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Train a single RandomForest model on all price series combined."""
        try:
            from sklearn.ensemble import RandomForestRegressor

            all_features: list = []
            all_targets: list = []

            for prices, volumes in series:
                if len(prices) < MIN_DATA_POINTS:
                    continue
                arr = np.array(prices, dtype=float)
                vol_arr = (
                    np.array(volumes, dtype=float)
                    if volumes and len(volumes) == len(prices)
                    else None
                )
                features, targets = self._build_features(arr, vol_arr)
                if len(features) > 1:
                    all_features.extend(features[:-1])
                    all_targets.extend(targets[:-1])

            if len(all_features) < 20:
                log.warning(
                    "random_forest.batch_train_skip",
                    reason="insufficient rows",
                    rows=len(all_features),
                )
                return

            X = np.array(all_features)
            y = np.array(all_targets)

            model = RandomForestRegressor(
                n_estimators=200,
                max_depth=8,
                min_samples_leaf=5,
                max_features="sqrt",
                n_jobs=-1,
                random_state=42,
            )
            model.fit(X, y)
            self._model = model
            log.info("random_forest.batch_trained", series=len(series), rows=len(all_features))
        except Exception as exc:
            log.warning("random_forest.batch_train_failed", error=str(exc))

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"RandomForest needs {MIN_DATA_POINTS} points, got {len(prices)}")

        current = float(prices[-1])

        if self._model is not None:
            try:
                return self._inference(prices, volumes, current)
            except Exception as exc:
                log.warning("random_forest.inference_failed", error=str(exc))

        try:
            return self._train_and_predict(prices, volumes, current)
        except Exception as exc:
            log.warning("random_forest.fallback", error=str(exc))
            return self._ema_fallback(prices, current)

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _inference(
        self, prices: list[float], volumes: list[float] | None, current: float
    ) -> PredictionResult:
        """Run prediction using cached model on last feature row."""
        arr = np.array(prices, dtype=float)
        vol_arr = (
            np.array(volumes, dtype=float)
            if volumes and len(volumes) == len(prices)
            else None
        )
        features, _ = self._build_features(arr, vol_arr)
        if not features:
            raise ValueError("No feature rows built")

        X_pred = np.array([features[-1]])
        pred_return = float(self._model.predict(X_pred)[0])

        predicted_price = current * (1 + pred_return)
        max_change = current * get_max_change_pct(self._market_key)
        predicted_price = max(current - max_change, min(current + max_change, predicted_price))
        confidence = max(0.3, min(0.9, 0.5 + abs(pred_return) * 5))

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    def _train_and_predict(
        self, prices: list[float], volumes: list[float] | None, current: float
    ) -> PredictionResult:
        from sklearn.ensemble import RandomForestRegressor

        arr = np.array(prices, dtype=float)
        vol_arr = (
            np.array(volumes, dtype=float)
            if volumes and len(volumes) == len(prices)
            else None
        )

        features, targets = self._build_features(arr, vol_arr)
        if len(features) < 20:
            raise ValueError("Not enough feature rows after engineering")

        X_train = np.array(features[:-1])
        y_train = np.array(targets[:-1])
        X_pred = np.array([features[-1]])

        model = RandomForestRegressor(
            n_estimators=200,
            max_depth=8,
            min_samples_leaf=5,
            max_features="sqrt",
            n_jobs=-1,
            random_state=42,
        )
        model.fit(X_train, y_train)

        pred_return = float(model.predict(X_pred)[0])

        predicted_price = current * (1 + pred_return)
        max_change = current * get_max_change_pct(self._market_key)
        predicted_price = max(current - max_change, min(current + max_change, predicted_price))
        confidence = max(0.3, min(0.9, 0.5 + abs(pred_return) * 5))

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    @staticmethod
    def _build_features(arr: np.ndarray, vol_arr: np.ndarray | None) -> tuple[list, list]:
        """Thin wrapper — use enhanced features, fall back to basic on error."""
        try:
            return build_enhanced_features(arr, vol_arr)
        except Exception:
            return build_basic_features(arr, vol_arr)

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
            confidence=0.35,
            current_price=current,
            algorithm_name="random_forest",
        )
