"""LightGBM prediction algorithm.

Features: lag returns (1-10 days), RSI, volume ratios, MA ratios.
Target: next-day log return (regression).
Supports per-market model caching via train(). When a cached model exists,
predict() uses it directly without refitting.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("lightgbm")

MIN_DATA_POINTS = 80  # need enough data to build lag features


class LightGBMPredictor(PredictionAlgorithm):
    """LightGBM-based return prediction with technical feature engineering."""

    def __init__(self) -> None:
        self._model = None  # cached lgb.LGBMRegressor

    def get_name(self) -> str:
        return "LightGBM"

    def get_key(self) -> str:
        return "lightgbm"

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
            import lightgbm as lgb

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

            split = max(1, int(len(X) * 0.8))
            X_tr, X_val = X[:split], X[split:]
            y_tr, y_val = y[:split], y[split:]

            params = {
                "objective": "regression",
                "metric": "mae",
                "learning_rate": 0.05,
                "num_leaves": 15,
                "min_data_in_leaf": 5,
                "verbose": -1,
                "n_estimators": 100,
            }
            model = lgb.LGBMRegressor(**params)
            eval_set = [(X_val, y_val)] if len(X_val) > 0 else None
            model.fit(
                X_tr,
                y_tr,
                eval_set=eval_set,
                callbacks=(
                    [lgb.early_stopping(10, verbose=False), lgb.log_evaluation(-1)]
                    if eval_set
                    else [lgb.log_evaluation(-1)]
                ),
            )
            self._model = model
            log.info("lightgbm.trained", data_points=len(prices))
        except Exception as exc:
            log.warning("lightgbm.train_failed", error=str(exc))

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Train a single LightGBM model on all price series combined."""
        try:
            import lightgbm as lgb

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
                log.warning("lightgbm.batch_train_skip", reason="insufficient rows", rows=len(all_features))
                return

            X = np.array(all_features)
            y = np.array(all_targets)
            split = max(1, int(len(X) * 0.8))
            X_tr, X_val = X[:split], X[split:]
            y_tr, y_val = y[:split], y[split:]

            params = {
                "objective": "regression",
                "metric": "mae",
                "learning_rate": 0.05,
                "num_leaves": 15,
                "min_data_in_leaf": 5,
                "verbose": -1,
                "n_estimators": 100,
            }
            model = lgb.LGBMRegressor(**params)
            eval_set = [(X_val, y_val)] if len(X_val) > 0 else None
            model.fit(
                X_tr,
                y_tr,
                eval_set=eval_set,
                callbacks=(
                    [lgb.early_stopping(10, verbose=False), lgb.log_evaluation(-1)]
                    if eval_set
                    else [lgb.log_evaluation(-1)]
                ),
            )
            self._model = model
            log.info("lightgbm.batch_trained", series=len(series), rows=len(all_features))
        except Exception as exc:
            log.warning("lightgbm.batch_train_failed", error=str(exc))

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"LightGBM needs {MIN_DATA_POINTS} points, got {len(prices)}")

        current = float(prices[-1])

        if self._model is not None:
            try:
                return self._inference(prices, volumes, current)
            except Exception as exc:
                log.warning("lightgbm.inference_failed", error=str(exc))
                # fall through to fresh train-and-predict

        try:
            return self._train_and_predict(prices, volumes, current)
        except Exception as exc:
            log.warning("lightgbm.fallback", error=str(exc))
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
        max_change = current * 0.07
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
        import lightgbm as lgb

        arr = np.array(prices, dtype=float)
        vol_arr = np.array(volumes, dtype=float) if volumes and len(volumes) == len(prices) else None

        features, targets = self._build_features(arr, vol_arr)
        if len(features) < 20:
            raise ValueError("Not enough feature rows after engineering")

        X_train = np.array(features[:-1])
        y_train = np.array(targets[:-1])
        X_pred = np.array([features[-1]])

        split = max(1, int(len(X_train) * 0.8))
        X_tr, X_val = X_train[:split], X_train[split:]
        y_tr, y_val = y_train[:split], y_train[split:]

        params = {
            "objective": "regression",
            "metric": "mae",
            "learning_rate": 0.05,
            "num_leaves": 15,
            "min_data_in_leaf": 5,
            "verbose": -1,
            "n_estimators": 100,
        }
        model = lgb.LGBMRegressor(**params)

        eval_set = [(X_val, y_val)] if len(X_val) > 0 else None
        model.fit(
            X_tr, y_tr,
            eval_set=eval_set,
            callbacks=[lgb.early_stopping(10, verbose=False), lgb.log_evaluation(-1)]
            if eval_set
            else [lgb.log_evaluation(-1)],
        )

        pred_return = float(model.predict(X_pred)[0])

        predicted_price = current * (1 + pred_return)
        max_change = current * 0.07
        predicted_price = max(current - max_change, min(current + max_change, predicted_price))

        # Confidence based on abs of predicted return (smaller = more uncertain)
        confidence = max(0.3, min(0.9, 0.5 + abs(pred_return) * 5))

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    @staticmethod
    def _build_features(arr: np.ndarray, vol_arr: np.ndarray | None) -> tuple[list, list]:
        log_returns = np.diff(np.log(arr))
        features, targets = [], []

        for i in range(10, len(log_returns)):
            row = []

            # Lag returns: 1..10
            for lag in range(1, 11):
                row.append(float(log_returns[i - lag]))

            # RSI(14)
            if i >= 14:
                subset = arr[i - 14 : i + 1]
                deltas = np.diff(subset)
                gains = np.where(deltas > 0, deltas, 0.0)
                losses = np.where(deltas < 0, -deltas, 0.0)
                avg_g = np.mean(gains) if len(gains) > 0 else 1e-9
                avg_l = np.mean(losses) if len(losses) > 0 else 1e-9
                rsi = 100 - 100 / (1 + avg_g / (avg_l + 1e-9))
            else:
                rsi = 50.0
            row.append(float(rsi))

            # MA ratios: price / MA5, price / MA20
            price_now = float(arr[i + 1])
            ma5 = float(np.mean(arr[max(0, i - 4) : i + 1])) if i >= 4 else price_now
            ma20 = float(np.mean(arr[max(0, i - 19) : i + 1])) if i >= 19 else price_now
            row.append(price_now / ma5 if ma5 > 0 else 1.0)
            row.append(price_now / ma20 if ma20 > 0 else 1.0)

            # Volume ratio: current vol / avg vol (10)
            if vol_arr is not None and len(vol_arr) > i + 1:
                vol_now = float(vol_arr[i + 1])
                vol_avg = float(np.mean(vol_arr[max(0, i - 9) : i + 1]))
                row.append(vol_now / vol_avg if vol_avg > 0 else 1.0)
            else:
                row.append(1.0)

            features.append(row)
            targets.append(float(log_returns[i]))

        return features, targets

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
            confidence=0.35,
            current_price=current,
            algorithm_name="lightgbm",
        )
