"""Meta-stacking supervised bot model: all-algo predictions → P(up next hour).

This module lives in the *simulation* layer, NOT in algorithms/.
It is a trading tactic, not a prediction algorithm: it does not implement
PredictionAlgorithm and is not registered in the predict registry.

Architecture:
  Feature vector x_t = [g_{t,1}..g_{t,A}, w_{t,1}..w_{t,A}, sigma_t, mom_t]
    g_{t,a} = (pred_{t,a} - p_t) / p_t          # relative prediction of algo a
    w_{t,a} = rolling direction accuracy K=40     # AS-OF t, no leakage
    sigma_t = std log-return window 20            # regime volatility
    mom_t   = (p_t - p_{t-5}) / p_{t-5}          # short momentum

  Model: LightGBM binary classifier (class_weight="balanced") + sigmoid
  (Platt) calibration → P(up).  Calibration is skipped when the held-out
  slice has < 30 rows or when the resulting p-distribution std < 0.02
  (still nearly flat), in which case the balanced raw model is used directly.
  Walk-forward split (NO shuffle): train 70%, calibrate 10%, test 20%.

Checkpoint:
  Pooled   : ${RL_MODEL_DIR}/meta_{MARKET}.pkl
  Per-symbol: ${RL_MODEL_DIR}/meta_{MARKET}_{symbol}.pkl

Fallback (no LightGBM or no checkpoint):
  Reliability-weighted vote: P(up) = 0.5 + sum(w_a * eff_sign_a) / (2*sum(w_a))
  where eff_sign_a = sign(g_a) if w_a >= 0.45 else -sign(g_a).
  Pipeline never crashes — fallback returns 0.5 when all signals are zero.
"""
from __future__ import annotations

import bisect
import math
import os
import pickle
from datetime import datetime, timedelta
from typing import Optional

import numpy as np

from src.storage.model_store import get_store
from src.utils.logger import get_logger

log = get_logger("simulation.meta_stack")

# ---------------------------------------------------------------------------
# Optional heavy deps
# ---------------------------------------------------------------------------
try:
    import lightgbm as lgb
    from sklearn.calibration import CalibratedClassifierCV
    HAS_LIGHTGBM = True
except ImportError:
    lgb = None  # type: ignore[assignment]
    CalibratedClassifierCV = None  # type: ignore[assignment]
    HAS_LIGHTGBM = False

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
FIXED_ALGO_KEYS: list[str] = [
    "moving_average", "ema", "lstm_nn", "gru_nn", "arima_garch",
    "egarch", "sarima", "lightgbm", "xgboost", "random_forest",
    "ensemble", "rl_dqn",
]
N_ALGOS = len(FIXED_ALGO_KEYS)
# Feature layout: [g_1..g_A | w_1..w_A | sigma | mom]
FEATURE_DIM = N_ALGOS * 2 + 2
ROLLING_K = 40          # Rolling direction-accuracy window
MIN_TRAIN_SAMPLES = 80  # Minimum aligned rows to attempt training


# ---------------------------------------------------------------------------
# Path helper
# ---------------------------------------------------------------------------

def meta_checkpoint_path(market: str, symbol: Optional[str] = None) -> str:
    """Return filesystem path for .pkl checkpoint."""
    model_dir = os.environ.get("RL_MODEL_DIR", "/models")
    market_slug = market.upper()
    if symbol is not None:
        sym_slug = symbol.replace("/", "_").replace(" ", "_")
        fname = f"meta_{market_slug}_{sym_slug}.pkl"
    else:
        fname = f"meta_{market_slug}.pkl"
    return os.path.join(model_dir, fname)


# ---------------------------------------------------------------------------
# In-process model cache (avoids reloading checkpoint on every bot step)
# ---------------------------------------------------------------------------
_model_cache: dict[str, "MetaStackModel"] = {}


def get_meta_model(market: str, symbol: Optional[str] = None) -> "MetaStackModel":
    """Return cached MetaStackModel, loading from checkpoint on first access."""
    cache_key = f"{market.upper()}_{symbol}" if symbol else market.upper()
    if cache_key not in _model_cache:
        model = MetaStackModel(market, symbol)
        model.load()  # silently no-ops if checkpoint absent → fallback vote
        _model_cache[cache_key] = model
    return _model_cache[cache_key]


def invalidate_cache(market: Optional[str] = None) -> None:
    """Invalidate cached models (call after train_meta_for_market to reload fresh checkpoint)."""
    global _model_cache
    if market is None:
        _model_cache.clear()
    else:
        prefix = market.upper()
        keys_to_remove = [k for k in _model_cache if k.startswith(prefix)]
        for k in keys_to_remove:
            del _model_cache[k]


# ---------------------------------------------------------------------------
# Feature helpers (stateless)
# ---------------------------------------------------------------------------

def _compute_sigma(prices: list[float], window: int = 20) -> float:
    """Standard deviation of log returns over last `window` periods."""
    if len(prices) < 2:
        return 0.0
    tail = prices[-(window + 1):]
    log_rets = []
    for i in range(1, len(tail)):
        if tail[i - 1] > 0 and tail[i] > 0:
            log_rets.append(math.log(tail[i] / tail[i - 1]))
    if len(log_rets) < 2:
        return 0.0
    return float(np.std(log_rets))


def _compute_momentum(prices: list[float], window: int = 5) -> float:
    """Short momentum: (prices[-1] - prices[-window-1]) / prices[-window-1]."""
    if len(prices) < window + 1:
        return 0.0
    older = prices[-(window + 1)]
    latest = prices[-1]
    if older <= 0:
        return 0.0
    return (latest - older) / older


def _rolling_accuracy_as_of(
    sorted_dc_rows: list[tuple],
    as_of_t: datetime,
    k: int = ROLLING_K,
) -> float:
    """Rolling direction accuracy using only rows where target_date < as_of_t.

    Args:
        sorted_dc_rows: List of (target_date, direction_correct) sorted by
                        target_date ASC. Elements may be datetime-naïve or aware
                        but must be consistently typed.
        as_of_t: Cut-off time; rows with target_date >= as_of_t are excluded.
        k: Window size.

    Returns:
        float in [0, 1]. Returns 0.5 (neutral) when no history is available.
    """
    if not sorted_dc_rows:
        return 0.5

    # Normalize as_of_t to match the type stored in the list
    sample_td = sorted_dc_rows[0][0]
    # If both are datetime objects, bisect works directly.
    targets = [r[0] for r in sorted_dc_rows]

    try:
        idx = bisect.bisect_left(targets, as_of_t)
    except TypeError:
        # Mixed date/datetime types — convert all to datetime
        from datetime import date
        def _to_dt(v):
            if isinstance(v, datetime):
                return v
            if isinstance(v, date):
                return datetime(v.year, v.month, v.day)
            return v
        targets_conv = [_to_dt(t) for t in targets]
        as_of_conv = _to_dt(as_of_t)
        idx = bisect.bisect_left(targets_conv, as_of_conv)

    available = sorted_dc_rows[:idx]
    if not available:
        return 0.5
    last_k = available[-k:]
    correct = sum(1 for _, dc in last_k if dc)
    return correct / len(last_k)


# ---------------------------------------------------------------------------
# MetaStackModel
# ---------------------------------------------------------------------------

class MetaStackModel:
    """Supervised meta-stacking model for one (market, optional symbol).

    Usage:
        model = MetaStackModel("GOLD")
        model.load()            # load checkpoint; if missing, fallback vote
        x = model.build_features_for_inference(symbol, as_of_dt, price_history)
        p = model.predict_proba(x)  # P(up)

        metrics = model.train()  # walk-forward train; saves checkpoint
    """

    def __init__(self, market: str, symbol: Optional[str] = None):
        self.market = market.upper()
        self.symbol = symbol
        self._calibrated_model = None   # CalibratedClassifierCV wrapping LGBMClassifier
        self._is_trained = False

    # ------------------------------------------------------------------
    # Checkpoint persistence
    # ------------------------------------------------------------------

    def checkpoint_path(self) -> str:
        return meta_checkpoint_path(self.market, self.symbol)

    def save(self) -> bool:
        """Pickle checkpoint to disk (and upload to S3 if backend=s3). Returns True on success."""
        if self._calibrated_model is None:
            return False
        path = self.checkpoint_path()
        try:
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "wb") as fh:
                pickle.dump({"calibrated_model": self._calibrated_model}, fh)
            log.info("meta.checkpoint.saved", path=path, market=self.market, symbol=self.symbol)
            # Upload to S3/MinIO (no-op for local backend).
            get_store().upload_if_remote(path)
            return True
        except Exception as exc:
            log.warning("meta.checkpoint.save.failed", path=path, error=str(exc))
            return False

    def load(self) -> bool:
        """Load checkpoint from disk (downloading from S3 if backend=s3 and file missing). Returns True on success, False if absent."""
        path = self.checkpoint_path()
        # Ensure local (downloads from S3 if backend=s3 and file missing locally).
        path = get_store().ensure_local(path)
        if not os.path.exists(path):
            return False
        try:
            with open(path, "rb") as fh:
                data = pickle.load(fh)
            self._calibrated_model = data["calibrated_model"]
            self._is_trained = True
            log.info("meta.checkpoint.loaded", path=path, market=self.market, symbol=self.symbol)
            return True
        except Exception as exc:
            log.warning("meta.checkpoint.load.failed", path=path, error=str(exc))
            self._calibrated_model = None
            self._is_trained = False
            return False

    def is_trained(self) -> bool:
        return self._is_trained

    # ------------------------------------------------------------------
    # Inference feature construction
    # ------------------------------------------------------------------

    def build_features_for_inference(
        self,
        symbol: str,
        as_of_dt: datetime,
        price_history: list[float],
    ) -> Optional[np.ndarray]:
        """Build feature vector x_t for bot step (AS-OF as_of_dt, no leakage).

        Args:
            symbol: Trading symbol (e.g. "XAU_spot", "AAPL", "BTC").
            as_of_dt: Current step datetime. Only predictions with
                      prediction_date <= as_of_dt and direction_correct data
                      with target_date < as_of_dt are used.
            price_history: ASC price list for sigma/momentum features.

        Returns:
            np.ndarray of shape (FEATURE_DIM,) or None on fatal error.
        """
        from src.database.connection import session_scope

        g_vec: list[float] = []
        w_vec: list[float] = []

        try:
            with session_scope() as session:
                for ak in FIXED_ALGO_KEYS:
                    row = self._query_latest_prediction(session, symbol, ak, as_of_dt)
                    if row is not None:
                        pred = float(row[0])
                        cur = float(row[1])
                        g = (pred - cur) / cur if cur != 0 else 0.0
                    else:
                        g = 0.0
                    g_vec.append(g)

                    w = self._query_rolling_accuracy(session, symbol, ak, as_of_dt)
                    w_vec.append(w)
        except Exception as exc:
            log.warning("meta.features.inference.error",
                        market=self.market, symbol=symbol, error=str(exc))
            return None

        sigma = _compute_sigma(price_history, window=20)
        mom = _compute_momentum(price_history, window=5)

        return np.array(g_vec + w_vec + [sigma, mom], dtype=np.float32)

    def _query_latest_prediction(
        self,
        session,
        symbol: str,
        algo: str,
        as_of_dt: datetime,
    ) -> Optional[tuple]:
        """Return (predicted_price, current_price) for the most recent prediction <= as_of_dt."""
        import sqlalchemy

        lookback = as_of_dt - timedelta(days=3)

        if self.market == "GOLD":
            parts = symbol.split("_", 1)
            if len(parts) != 2:
                return None
            src, ptype = parts
            row = session.execute(
                sqlalchemy.text("""
                    SELECT predicted_price, current_price
                    FROM gold_predictions
                    WHERE source = :src AND product_type = :ptype
                      AND algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                    ORDER BY prediction_date DESC LIMIT 1
                """),
                {"src": src, "ptype": ptype, "algo": algo,
                 "d_start": lookback, "d_end": as_of_dt},
            ).fetchone()

        elif self.market == "NASDAQ":
            row = session.execute(
                sqlalchemy.text("""
                    SELECT predicted_price, current_price
                    FROM nasdaq_predictions
                    WHERE symbol = :sym AND algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                    ORDER BY prediction_date DESC LIMIT 1
                """),
                {"sym": symbol, "algo": algo,
                 "d_start": lookback, "d_end": as_of_dt},
            ).fetchone()

        elif self.market == "SP500":
            row = session.execute(
                sqlalchemy.text("""
                    SELECT predicted_price, current_price
                    FROM sp500_predictions
                    WHERE symbol = :sym AND algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                    ORDER BY prediction_date DESC LIMIT 1
                """),
                {"sym": symbol, "algo": algo,
                 "d_start": lookback, "d_end": as_of_dt},
            ).fetchone()

        elif self.market == "CRYPTO":
            row = session.execute(
                sqlalchemy.text("""
                    SELECT predicted_price, current_price
                    FROM crypto_predictions
                    WHERE symbol = :sym AND algorithm_name = :algo
                      AND prediction_date BETWEEN :d_start AND :d_end
                      AND deleted_at IS NULL
                    ORDER BY prediction_date DESC LIMIT 1
                """),
                {"sym": symbol, "algo": algo,
                 "d_start": lookback, "d_end": as_of_dt},
            ).fetchone()

        else:
            return None

        return row

    def _query_rolling_accuracy(
        self,
        session,
        symbol: str,
        algo: str,
        as_of_dt: datetime,
        k: int = ROLLING_K,
    ) -> float:
        """Return rolling direction accuracy for algo AS-OF as_of_dt (no leakage).

        Only rows where target_date < as_of_dt are considered, ensuring we do
        not use outcomes that have not yet been revealed.
        """
        import sqlalchemy

        if self.market == "GOLD":
            parts = symbol.split("_", 1)
            if len(parts) != 2:
                return 0.5
            src, ptype = parts
            rows = session.execute(
                sqlalchemy.text("""
                    SELECT direction_correct FROM gold_predictions
                    WHERE source = :src AND product_type = :ptype
                      AND algorithm_name = :algo
                      AND target_date < :t
                      AND direction_correct IS NOT NULL
                      AND deleted_at IS NULL
                    ORDER BY target_date DESC LIMIT :k
                """),
                {"src": src, "ptype": ptype, "algo": algo, "t": as_of_dt, "k": k},
            ).fetchall()

        elif self.market == "NASDAQ":
            rows = session.execute(
                sqlalchemy.text("""
                    SELECT direction_correct FROM nasdaq_predictions
                    WHERE symbol = :sym AND algorithm_name = :algo
                      AND target_date < :t
                      AND direction_correct IS NOT NULL
                      AND deleted_at IS NULL
                    ORDER BY target_date DESC LIMIT :k
                """),
                {"sym": symbol, "algo": algo, "t": as_of_dt, "k": k},
            ).fetchall()

        elif self.market == "SP500":
            rows = session.execute(
                sqlalchemy.text("""
                    SELECT direction_correct FROM sp500_predictions
                    WHERE symbol = :sym AND algorithm_name = :algo
                      AND target_date < :t
                      AND direction_correct IS NOT NULL
                      AND deleted_at IS NULL
                    ORDER BY target_date DESC LIMIT :k
                """),
                {"sym": symbol, "algo": algo, "t": as_of_dt, "k": k},
            ).fetchall()

        elif self.market == "CRYPTO":
            rows = session.execute(
                sqlalchemy.text("""
                    SELECT direction_correct FROM crypto_predictions
                    WHERE symbol = :sym AND algorithm_name = :algo
                      AND target_date < :t
                      AND direction_correct IS NOT NULL
                      AND deleted_at IS NULL
                    ORDER BY target_date DESC LIMIT :k
                """),
                {"sym": symbol, "algo": algo, "t": as_of_dt, "k": k},
            ).fetchall()

        else:
            return 0.5

        if not rows:
            return 0.5
        correct = sum(1 for r in rows if r[0])
        return correct / len(rows)

    # ------------------------------------------------------------------
    # Fallback vote (used when no model is loaded)
    # ------------------------------------------------------------------

    @staticmethod
    def _fallback_vote(g_vec: list[float], w_vec: list[float]) -> float:
        """Reliability-weighted vote: P(up) without a trained model.

        For each algo a:
          - effective_signal_a = sign(g_a)        if w_a >= 0.45
          - effective_signal_a = -sign(g_a)       if w_a < 0.45  (signal inversion)

        P(up) = 0.5 + sum(w_a * eff_signal_a) / (2 * sum(w_a))

        Returns 0.5 when all weights or signals are zero.
        """
        total_w = 0.0
        total_signal = 0.0
        for g, w in zip(g_vec, w_vec):
            if w <= 0:
                continue
            sign_g = 1.0 if g > 0 else (-1.0 if g < 0 else 0.0)
            if w < 0.45:
                sign_g = -sign_g  # invert: systematically-wrong algos
            total_w += w
            total_signal += w * sign_g
        if total_w == 0:
            return 0.5
        p = 0.5 + total_signal / (2.0 * total_w)
        return max(0.0, min(1.0, p))

    # ------------------------------------------------------------------
    # Inference
    # ------------------------------------------------------------------

    def predict_proba(self, x: np.ndarray) -> float:
        """Return P(up | x_t) in [0, 1].

        Uses the trained/calibrated LightGBM model when available; falls back
        to reliability-weighted vote otherwise. Never raises.
        """
        if self._calibrated_model is not None:
            try:
                proba = float(self._calibrated_model.predict_proba([x])[0][1])
                return max(0.0, min(1.0, proba))
            except Exception as exc:
                log.warning("meta.predict_proba.model.error", error=str(exc))

        # Fallback: extract g and w from feature vector and do weighted vote
        g_vec = x[:N_ALGOS].tolist()
        w_vec = x[N_ALGOS:2 * N_ALGOS].tolist()
        return self._fallback_vote(g_vec, w_vec)

    # ------------------------------------------------------------------
    # Training
    # ------------------------------------------------------------------

    def train(
        self,
        min_samples: int = MIN_TRAIN_SAMPLES,
        max_date=None,
    ) -> dict:
        """Walk-forward supervised training.

        Splits data chronologically (NO shuffle):
          Train  : rows  0 .. 70%
          Calibrate: rows 70% .. 80%   (Platt sigmoid calibration, held-out)
          Test   : rows 80% .. 100%   (out-of-sample evaluation)

        Class imbalance (e.g. recent data skewed toward "down") is handled via
        class_weight="balanced" in LGBMClassifier so that P outputs are centred
        near 0.5 rather than collapsing to the training base-rate.

        Calibration uses sigmoid (Platt) instead of isotonic: Platt is monotonic
        and preserves the spread of the probability distribution, whereas isotonic
        tends to clip probabilities back to the training base-rate.  Calibration
        is skipped when the held-out slice has < 30 samples (too few for reliable
        fit) or when the post-calibration p-std on the test set is < 0.02 (still
        flat — use balanced raw model instead).

        Reports out-of-sample direction_accuracy and brier_score via log and
        return dict.

        Args:
            min_samples: Minimum aligned rows to attempt training.
            max_date: When set (date or datetime), only prediction rows with
                      prediction_date <= max_date are used.  None = no filter
                      (normal cron behaviour, backward-compatible).

        Returns dict with keys: status, direction_accuracy, brier_score, n_test.
        """
        if not HAS_LIGHTGBM:
            log.warning("meta.train.no_lightgbm", market=self.market, symbol=self.symbol)
            return {"status": "skipped", "reason": "lightgbm not available — fallback vote will be used"}

        log.info("meta.train.start", market=self.market, symbol=self.symbol,
                 max_date=str(max_date) if max_date is not None else None)

        rows = self._fetch_all_predictions(max_date=max_date)
        if not rows:
            log.warning("meta.train.no_data", market=self.market, symbol=self.symbol)
            return {"status": "skipped", "reason": "no reconciled predictions found"}

        records = self._build_training_records(rows)
        n_records = len(records)
        if n_records < min_samples:
            log.warning("meta.train.insufficient",
                        market=self.market, symbol=self.symbol,
                        n_records=n_records, min_required=min_samples)
            return {"status": "skipped",
                    "reason": f"only {n_records} aligned records (need {min_samples})"}

        # Sort chronologically — walk-forward, no shuffle
        records.sort(key=lambda r: r["t"])

        n = len(records)
        train_end = int(n * 0.70)
        calib_end = int(n * 0.80)

        X_train = np.array([r["x"] for r in records[:train_end]], dtype=np.float32)
        y_train = np.array([r["y"] for r in records[:train_end]], dtype=int)
        X_calib = np.array([r["x"] for r in records[train_end:calib_end]], dtype=np.float32)
        y_calib = np.array([r["y"] for r in records[train_end:calib_end]], dtype=int)
        X_test = np.array([r["x"] for r in records[calib_end:]], dtype=np.float32)
        y_test = np.array([r["y"] for r in records[calib_end:]], dtype=int)

        if len(X_train) < 10 or len(X_calib) < 5 or len(X_test) < 5:
            log.warning("meta.train.split_too_small",
                        market=self.market, symbol=self.symbol,
                        train=len(X_train), calib=len(X_calib), test=len(X_test))
            return {"status": "skipped", "reason": "walk-forward splits too small after 70/10/20 split"}

        # Train LightGBM binary classifier with class balancing (train slice).
        # class_weight="balanced" weights minority class up so the model does not
        # trivially predict the majority class — keeps P outputs centred near 0.5.
        try:
            base_model = lgb.LGBMClassifier(
                n_estimators=100,
                learning_rate=0.05,
                num_leaves=31,
                max_depth=5,
                subsample=0.8,
                colsample_bytree=0.8,
                class_weight="balanced",  # counteract base-rate skew toward "down"
                random_state=42,
                verbose=-1,
            )
            base_model.fit(X_train, y_train)
        except Exception as exc:
            log.error("meta.train.lgbm.failed", market=self.market, error=str(exc))
            return {"status": "failed", "reason": str(exc)}

        # Sigmoid (Platt) calibration on held-out slice.
        # Platt is monotonic and preserves the probability spread; isotonic can
        # collapse probabilities back to the training base-rate.
        # Skip calibration when the held-out slice is too small (< 30 rows) or
        # when the calibrated model still produces a nearly-flat distribution
        # (p_std < 0.02 on the test set) — in both cases use the balanced raw model.
        final_model = base_model  # default: raw balanced model
        if len(X_calib) < 30:
            log.info("meta.train.calibration.skipped",
                     market=self.market, symbol=self.symbol,
                     n_calib=len(X_calib), reason="calib_set_lt_30")
        else:
            try:
                calibrated = CalibratedClassifierCV(base_model, method="sigmoid", cv="prefit")
                calibrated.fit(X_calib, y_calib)
                # Sanity-check: verify calibration did not collapse the p spread
                if len(X_test) > 0:
                    calib_probas = calibrated.predict_proba(X_test)[:, 1]
                    p_std_calib = float(np.std(calib_probas))
                else:
                    p_std_calib = 1.0  # cannot check — assume OK
                if p_std_calib < 0.02:
                    log.warning("meta.train.calibration.flat",
                                market=self.market, symbol=self.symbol,
                                p_std=round(p_std_calib, 4),
                                reason="using_balanced_raw_model")
                    # final_model stays as base_model
                else:
                    final_model = calibrated
            except Exception as exc:
                log.warning("meta.train.calibration.failed",
                            market=self.market, symbol=self.symbol, error=str(exc))
                # final_model stays as base_model

        self._calibrated_model = final_model
        self._is_trained = True

        # Out-of-sample evaluation (test slice)
        metrics = self._evaluate(X_test, y_test)

        # Log p distribution on test set — confirms spread is centred near 0.5
        if len(X_test) > 0:
            try:
                test_probas = final_model.predict_proba(X_test)[:, 1]
                log.info("meta.train.pdist",
                         market=self.market, symbol=self.symbol,
                         p_min=round(float(np.min(test_probas)), 4),
                         p_med=round(float(np.median(test_probas)), 4),
                         p_max=round(float(np.max(test_probas)), 4),
                         p_std=round(float(np.std(test_probas)), 4))
            except Exception:
                pass

        log.info(
            "meta.train.done",
            market=self.market, symbol=self.symbol,
            n_train=len(X_train), n_calib=len(X_calib), n_test=len(X_test),
            direction_accuracy=metrics.get("direction_accuracy"),
            brier_score=metrics.get("brier_score"),
        )

        self.save()
        return {"status": "success", **metrics}

    def _evaluate(self, X_test: np.ndarray, y_test: np.ndarray) -> dict:
        """Compute out-of-sample direction accuracy and Brier score."""
        if len(X_test) == 0 or self._calibrated_model is None:
            return {}
        try:
            probas = self._calibrated_model.predict_proba(X_test)[:, 1]
            preds = (probas > 0.5).astype(int)
            da = float(np.mean(preds == y_test))
            brier = float(np.mean((probas - y_test.astype(float)) ** 2))
            return {
                "direction_accuracy": round(da, 4),
                "brier_score": round(brier, 4),
                "n_test": int(len(X_test)),
            }
        except Exception as exc:
            log.warning("meta.evaluate.error", error=str(exc))
            return {}

    # ------------------------------------------------------------------
    # Training data extraction
    # ------------------------------------------------------------------

    def _fetch_all_predictions(self, max_date=None) -> list[dict]:
        """Fetch reconciled prediction rows for this market (and optional symbol).

        Args:
            max_date: When set (date or datetime), only rows with
                      prediction_date <= max_date are returned.  None = no
                      filter (normal behaviour).

        Returns list of dicts:
          algorithm_name, prediction_date, predicted_price, current_price,
          actual_price, direction_correct, target_date, symbol.
        """
        from src.database.connection import session_scope
        import sqlalchemy

        sym_filter = self.symbol
        rows: list[dict] = []

        # Build the optional date-cap clause and its bind params
        date_clause = ""
        date_params: dict = {}
        if max_date is not None:
            date_clause = "AND prediction_date <= :max_date"
            date_params["max_date"] = max_date

        try:
            with session_scope() as session:
                if self.market == "GOLD":
                    result = session.execute(sqlalchemy.text(f"""
                        SELECT algorithm_name, prediction_date, predicted_price,
                               current_price, actual_price, direction_correct, target_date,
                               CONCAT(source, '_', product_type) AS symbol
                        FROM gold_predictions
                        WHERE direction_correct IS NOT NULL
                          AND actual_price IS NOT NULL
                          AND deleted_at IS NULL
                          {date_clause}
                        ORDER BY prediction_date ASC
                    """), date_params).fetchall()

                elif self.market == "NASDAQ":
                    result = session.execute(sqlalchemy.text(f"""
                        SELECT algorithm_name, prediction_date, predicted_price,
                               current_price, actual_price, direction_correct, target_date,
                               symbol
                        FROM nasdaq_predictions
                        WHERE direction_correct IS NOT NULL
                          AND actual_price IS NOT NULL
                          AND deleted_at IS NULL
                          {date_clause}
                        ORDER BY prediction_date ASC
                    """), date_params).fetchall()

                elif self.market == "SP500":
                    result = session.execute(sqlalchemy.text(f"""
                        SELECT algorithm_name, prediction_date, predicted_price,
                               current_price, actual_price, direction_correct, target_date,
                               symbol
                        FROM sp500_predictions
                        WHERE direction_correct IS NOT NULL
                          AND actual_price IS NOT NULL
                          AND deleted_at IS NULL
                          {date_clause}
                        ORDER BY prediction_date ASC
                    """), date_params).fetchall()

                elif self.market == "CRYPTO":
                    result = session.execute(sqlalchemy.text(f"""
                        SELECT algorithm_name, prediction_date, predicted_price,
                               current_price, actual_price, direction_correct, target_date,
                               symbol
                        FROM crypto_predictions
                        WHERE direction_correct IS NOT NULL
                          AND actual_price IS NOT NULL
                          AND deleted_at IS NULL
                          {date_clause}
                        ORDER BY prediction_date ASC
                    """), date_params).fetchall()

                else:
                    log.warning("meta.fetch.unknown_market", market=self.market)
                    return []

                for r in result:
                    sym = r[7]
                    if sym_filter is not None and sym != sym_filter:
                        continue
                    rows.append({
                        "algorithm_name": str(r[0]),
                        "prediction_date": r[1],
                        "predicted_price": float(r[2]),
                        "current_price": float(r[3]),
                        "actual_price": float(r[4]),
                        "direction_correct": bool(r[5]),
                        "target_date": r[6],
                        "symbol": sym,
                    })

        except Exception as exc:
            log.error("meta.fetch_predictions.error", market=self.market, error=str(exc))

        return rows

    def _build_training_records(self, rows: list[dict]) -> list[dict]:
        """Build (x, y, t) records from raw prediction rows.

        AS-OF invariant: w_{t,a} is computed using ONLY direction_correct data
        where target_date < prediction_date (t). This is enforced by
        _rolling_accuracy_as_of via binary search on sorted target_dates.

        Leakage check: within _rolling_accuracy_as_of, only rows with
        target_date STRICTLY LESS THAN the current prediction_date t are
        included. A future target_date is never visible at training time t.
        """
        from collections import defaultdict

        # Build per-algo direction_correct history sorted by target_date
        # (the time at which the outcome becomes known)
        algo_dc_history: dict[str, list[tuple]] = defaultdict(list)
        for r in rows:
            algo_dc_history[r["algorithm_name"]].append(
                (r["target_date"], r["direction_correct"])
            )
        for ak in algo_dc_history:
            algo_dc_history[ak].sort(key=lambda x: x[0])

        # Group predictions by (symbol, prediction_date) for joint feature rows
        joint: dict[tuple, dict[str, dict]] = defaultdict(dict)
        for r in rows:
            key = (r["symbol"], r["prediction_date"])
            joint[key][r["algorithm_name"]] = r

        # Build approximated price series per symbol from current_price
        # Used for sigma/momentum features (no DB round-trip per row)
        sym_prices: dict[str, list[tuple]] = defaultdict(list)
        for (sym, t), algo_rows in joint.items():
            if algo_rows:
                cur = next(iter(algo_rows.values()))["current_price"]
                sym_prices[sym].append((t, float(cur)))
        for sym in sym_prices:
            sym_prices[sym].sort(key=lambda x: x[0])

        records: list[dict] = []
        time_points = sorted(joint.keys(), key=lambda k: k[1])

        for (sym, t) in time_points:
            algo_rows = joint[(sym, t)]

            g_vec: list[float] = []
            w_vec: list[float] = []
            for ak in FIXED_ALGO_KEYS:
                if ak in algo_rows:
                    ar = algo_rows[ak]
                    cur = ar["current_price"]
                    pred = ar["predicted_price"]
                    g = (pred - cur) / cur if cur != 0 else 0.0
                else:
                    g = 0.0
                g_vec.append(g)

                # AS-OF: only use direction_correct for target_date < t
                hist = algo_dc_history.get(ak, [])
                w = _rolling_accuracy_as_of(hist, t, k=ROLLING_K)
                w_vec.append(w)

            # Sigma and momentum from price history up to and including t
            available_prices = [p for (pt, p) in sym_prices[sym] if pt <= t]
            sigma = _compute_sigma(available_prices, window=20)
            mom = _compute_momentum(available_prices, window=5)

            x = np.array(g_vec + w_vec + [sigma, mom], dtype=np.float32)

            # Label: y=1 if actual > current (true direction)
            y: Optional[int] = None
            for ak in FIXED_ALGO_KEYS:
                if ak in algo_rows:
                    ar = algo_rows[ak]
                    cur = ar["current_price"]
                    actual = ar["actual_price"]
                    if cur > 0:
                        y = 1 if actual > cur else 0
                        break

            if y is None:
                # Fallback: infer from direction_correct
                for ak in FIXED_ALGO_KEYS:
                    if ak in algo_rows:
                        ar = algo_rows[ak]
                        dc = ar["direction_correct"]
                        cur = ar["current_price"]
                        pred = ar["predicted_price"]
                        g_sign = (pred - cur) > 0 if cur != 0 else False
                        # dc=True and g_sign=True  → actual went up   → y=1
                        # dc=True and g_sign=False → actual went down → y=0
                        # dc=False means direction was wrong → flip g_sign
                        if dc:
                            y = 1 if g_sign else 0
                        else:
                            y = 0 if g_sign else 1
                        break

            if y is None:
                continue  # Cannot determine label — skip

            records.append({"t": t, "x": x, "y": y, "symbol": sym})

        return records
