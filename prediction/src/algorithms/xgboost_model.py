"""XGBoost prediction algorithm.

Features: enhanced ~30-feature set via features.build_enhanced_features()
  (lag returns 1-10, RSI, StochRSI, Bollinger %B, MACD, volatility, ROC,
  momentum, MA ratios 5/10/20/50, multi-timeframe returns, volume ratio).
Target: next-day log return (regression).

Supports:
  - Per-market model caching via train() / train_batch().
  - Optuna hyperparameter optimisation when data >= 200 points and optuna
    is installed (falls back to default params gracefully).
  - EMA fallback when XGBoost is unavailable.
"""
from __future__ import annotations

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct
from src.algorithms.features import build_enhanced_features, build_basic_features
from src.utils.logger import get_logger

log = get_logger("xgboost")

MIN_DATA_POINTS = 80        # minimum to build lag features
HYPEROPT_MIN_POINTS = 200   # minimum to bother with Optuna search
HYPEROPT_TRIALS = 30
HYPEROPT_TIMEOUT = 120      # seconds

_DEFAULT_PARAMS = {
    "n_estimators": 200,
    "learning_rate": 0.05,
    "max_depth": 5,
    "subsample": 0.8,
    "colsample_bytree": 0.8,
    "min_child_weight": 5,
    "random_state": 42,
    "verbosity": 0,
}


class XGBoostPredictor(PredictionAlgorithm):
    """XGBoost-based return prediction with enhanced technical feature engineering."""

    def __init__(self) -> None:
        self._model = None  # cached XGBRegressor

    def get_name(self) -> str:
        return "XGBoost"

    def get_key(self) -> str:
        return "xgboost"

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
            from xgboost import XGBRegressor

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

            params = _tune_xgboost_params(X_tr, y_tr, X_val, y_val, len(prices))
            model = XGBRegressor(**params)

            eval_set = [(X_val, y_val)] if len(X_val) > 0 else None
            if eval_set:
                model.set_params(early_stopping_rounds=10)
                model.fit(X_tr, y_tr, eval_set=eval_set, verbose=False)
            else:
                model.fit(X_tr, y_tr, verbose=False)

            self._model = model
            log.info("xgboost.trained", data_points=len(prices))
        except Exception as exc:
            log.warning("xgboost.train_failed", error=str(exc))

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Train a single XGBoost model on all price series combined."""
        try:
            from xgboost import XGBRegressor

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
                    "xgboost.batch_train_skip",
                    reason="insufficient rows",
                    rows=len(all_features),
                )
                return

            X = np.array(all_features)
            y = np.array(all_targets)
            split = max(1, int(len(X) * 0.8))
            X_tr, X_val = X[:split], X[split:]
            y_tr, y_val = y[:split], y[split:]

            total_points = sum(len(p) for p, _ in series)
            params = _tune_xgboost_params(X_tr, y_tr, X_val, y_val, total_points)
            model = XGBRegressor(**params)

            eval_set = [(X_val, y_val)] if len(X_val) > 0 else None
            if eval_set:
                model.set_params(early_stopping_rounds=10)
                model.fit(X_tr, y_tr, eval_set=eval_set, verbose=False)
            else:
                model.fit(X_tr, y_tr, verbose=False)

            self._model = model
            log.info("xgboost.batch_trained", series=len(series), rows=len(all_features))
        except Exception as exc:
            log.warning("xgboost.batch_train_failed", error=str(exc))

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"XGBoost needs {MIN_DATA_POINTS} points, got {len(prices)}")

        current = float(prices[-1])

        if self._model is not None:
            try:
                return self._inference(prices, volumes, current)
            except Exception as exc:
                log.warning("xgboost.inference_failed", error=str(exc))

        try:
            return self._train_and_predict(prices, volumes, current)
        except Exception as exc:
            log.warning("xgboost.fallback", error=str(exc))
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
        from xgboost import XGBRegressor

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

        split = max(1, int(len(X_train) * 0.8))
        X_tr, X_val = X_train[:split], X_train[split:]
        y_tr, y_val = y_train[:split], y_train[split:]

        params = _tune_xgboost_params(X_tr, y_tr, X_val, y_val, len(prices))
        model = XGBRegressor(**params)

        eval_set = [(X_val, y_val)] if len(X_val) > 0 else None
        if eval_set:
            model.set_params(early_stopping_rounds=10)
            model.fit(X_tr, y_tr, eval_set=eval_set, verbose=False)
        else:
            model.fit(X_tr, y_tr, verbose=False)

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
            algorithm_name="xgboost",
        )


# ---------------------------------------------------------------------------
# Optuna hyperparameter tuning helper
# ---------------------------------------------------------------------------

def _tune_xgboost_params(
    X_tr: np.ndarray,
    y_tr: np.ndarray,
    X_val: np.ndarray,
    y_val: np.ndarray,
    n_data_points: int,
) -> dict:
    """Return best XGBoost params via Optuna, or defaults if unavailable/insufficient data."""
    if n_data_points < HYPEROPT_MIN_POINTS or len(X_val) < 5:
        return dict(_DEFAULT_PARAMS)

    try:
        import optuna
        from xgboost import XGBRegressor
        from sklearn.metrics import mean_absolute_error

        optuna.logging.set_verbosity(optuna.logging.WARNING)

        def objective(trial: optuna.Trial) -> float:
            params = {
                "n_estimators": trial.suggest_int("n_estimators", 50, 300),
                "learning_rate": trial.suggest_float("learning_rate", 0.01, 0.2, log=True),
                "max_depth": trial.suggest_int("max_depth", 3, 10),
                "subsample": trial.suggest_float("subsample", 0.6, 1.0),
                "colsample_bytree": trial.suggest_float("colsample_bytree", 0.6, 1.0),
                "min_child_weight": trial.suggest_int("min_child_weight", 1, 10),
                "random_state": 42,
                "verbosity": 0,
                "early_stopping_rounds": 10,
            }
            model = XGBRegressor(**params)
            model.fit(X_tr, y_tr, eval_set=[(X_val, y_val)], verbose=False)
            preds = model.predict(X_val)
            return float(mean_absolute_error(y_val, preds))

        study = optuna.create_study(direction="minimize")
        study.optimize(objective, n_trials=HYPEROPT_TRIALS, timeout=HYPEROPT_TIMEOUT)

        best = study.best_params
        best_params = {
            "n_estimators": best["n_estimators"],
            "learning_rate": best["learning_rate"],
            "max_depth": best["max_depth"],
            "subsample": best["subsample"],
            "colsample_bytree": best["colsample_bytree"],
            "min_child_weight": best["min_child_weight"],
            "random_state": 42,
            "verbosity": 0,
        }
        log.info(
            "xgboost.hyperopt_done",
            best_mae=study.best_value,
            n_trials=len(study.trials),
            params=best_params,
        )
        return best_params

    except ImportError:
        log.debug("xgboost.hyperopt_skip", reason="optuna not installed")
        return dict(_DEFAULT_PARAMS)
    except Exception as exc:
        log.warning("xgboost.hyperopt_failed", error=str(exc))
        return dict(_DEFAULT_PARAMS)
