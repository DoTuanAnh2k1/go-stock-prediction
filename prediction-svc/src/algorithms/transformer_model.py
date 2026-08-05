"""Transformer prediction algorithm (#13) — PatchTST-lite with fundamentals context.

A patch-based Transformer encoder over log-returns, optionally conditioned on
per-symbol fundamental snapshots (P/E, EPS, growth, margins, market cap, …)
from the `stock_fundamentals` table (crawled weekly via yfinance).

Architecture:
  - Input: last SEQ_LEN standardized log-returns, split into SEQ_LEN/PATCH_LEN
    patches; each patch linearly embedded to D_MODEL + learned positional emb.
  - 2-layer TransformerEncoder (pre-norm, N_HEADS heads, FF_DIM feed-forward).
  - Mean-pooled tokens concatenated with a projected fundamentals vector
    (FUND_DIM raw fields → 16), then an MLP head regresses the NEXT log-return.

Fundamentals:
  - Fields follow repository.FUNDAMENTAL_FIELDS; market_cap is log10-scaled.
  - Standardized with train-set nan-mean/nan-std (stored in the checkpoint);
    missing values (GOLD/CRYPTO — no financial reports, or ETFs) impute to 0
    (the train-set mean), so the model degrades gracefully to price-only.
  - At predict time the symbol comes from `_context_symbol` (set per-symbol by
    the runner for equities) or `_symbol_key` (per-symbol instances).
  - CAVEAT: snapshots are latest-only, not point-in-time — training joins
    today's ratios onto past windows. Acceptable for slow-moving ratios.

Training (train_batch / train_batch_labeled):
  - Pooled per market; windows from all series; the most recent 10% of every
    series' windows are a chronological validation split.
  - AdamW + Huber loss, early stopping on val loss (patience EARLY_STOP_PATIENCE),
    best-epoch weights kept; val direction accuracy logged.
  - Checkpoint: ${RL_MODEL_DIR}/transformer_{market}[_{symbol}].pt

Prediction:
  - predicted = current · exp(r̂) with r̂ un-standardized and clipped to
    ±4·train-σ, then the market-aware clamp.
  - Confidence scales with |r̂| relative to recent volatility.
  - Cold start (no checkpoint): quick single-series train (like LSTM), then
    predict; failures re-raise (fail-loud policy).
"""
from __future__ import annotations

import math
import os
import time as _time

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct
from src.storage.model_store import get_store
from src.utils.logger import get_logger

log = get_logger("transformer")

# ---------------------------------------------------------------------------
# Hyper-parameters
# ---------------------------------------------------------------------------
SEQ_LEN = 96            # log-return window length (must be divisible by PATCH_LEN)
PATCH_LEN = 8           # returns per patch → SEQ_LEN/PATCH_LEN = 12 tokens
D_MODEL = 64
N_HEADS = 4
N_LAYERS = 2
FF_DIM = 128
DROPOUT = 0.1
FUND_HIDDEN = 16        # projected fundamentals width

EPOCHS = 80
QUICK_EPOCHS = 15       # cold-start single-series training
BATCH_SIZE = 256
LR = 1e-3
WEIGHT_DECAY = 1e-4
EARLY_STOP_PATIENCE = 8
VAL_FRACTION = 0.10     # most recent fraction of each series' windows → validation
MIN_WINDOWS = 32        # minimum train windows for pooled training

# Dual head: the system scores DIRECTION, so a classification head predicting
# P(up) is trained jointly with the return regression (loss = Huber + λ·BCE).
HEAD_SINGLE = "single"  # legacy checkpoints (regression only)
HEAD_DUAL = "dual"
DIR_LOSS_WEIGHT = 0.3   # λ for the BCE direction loss
SKIP_GAP = 0.02         # |P(up) − 0.5| below this → skip_write (no conviction)

MIN_DATA_POINTS_TF = SEQ_LEN + 10   # prices needed for one inference window

_FUND_CACHE_TTL_S = 6 * 3600

_DEFAULT_MODEL_DIR = "/models"

# Module-level fundamentals cache: {MARKET: (fetched_monotonic, {symbol: {...}})}
_fund_cache: dict[str, tuple[float, dict]] = {}


def _direction_targets(y_std, r_mean: float, r_std: float):
    """Binary 'next return is positive' labels for the direction head.

    y_std holds *standardized* returns ((ret - r_mean) / r_std), so the raw
    return is positive exactly when y_std > -r_mean / r_std. Thresholding at 0
    instead would ask 'return above the mean' — a different question from the
    one the system scores (direction_correct := actual > current, i.e. ret > 0).
    In up-drifting markets (r_mean > 0) that mismatch biased the head toward
    'down', dragging live direction accuracy below chance. Works on numpy
    arrays and torch tensors alike (comparison only). Returns a bool mask;
    callers apply .float().
    """
    thresh = (-r_mean / r_std) if r_std else 0.0
    return y_std > thresh


def _get_model_dir() -> str:
    env = os.environ.get("RL_MODEL_DIR")
    if env:
        return env
    try:
        from src.config import get_settings
        return get_settings().rl_model_dir
    except Exception:
        return _DEFAULT_MODEL_DIR


def _tf_checkpoint_path(market_key: str, symbol_key: str | None = None) -> str:
    safe_market = (market_key or "UNKNOWN").upper().replace("/", "_")
    if symbol_key:
        safe_symbol = symbol_key.replace(os.sep, "_").replace("/", "_").replace(" ", "_")
        return os.path.join(_get_model_dir(), f"transformer_{safe_market}_{safe_symbol}.pt")
    return os.path.join(_get_model_dir(), f"transformer_{safe_market}.pt")


# ---------------------------------------------------------------------------
# Fundamentals helpers
# ---------------------------------------------------------------------------

def _fundamental_fields() -> list[str]:
    from src.database.repository import FUNDAMENTAL_FIELDS
    return list(FUNDAMENTAL_FIELDS)


FUND_DIM = 11   # len(repository.FUNDAMENTAL_FIELDS) — pinned so checkpoints are stable


def _get_fundamentals_for_market(market_key: str) -> dict[str, dict]:
    """DB fundamentals map with in-process TTL cache. {} when unavailable."""
    mk = (market_key or "").upper()
    now = _time.monotonic()
    cached = _fund_cache.get(mk)
    if cached and now - cached[0] < _FUND_CACHE_TTL_S:
        return cached[1]
    try:
        from src.database import repository as repo
        fmap = repo.get_fundamentals_map(mk)
    except Exception as exc:
        log.debug("transformer.fundamentals.unavailable", market=mk, error=str(exc))
        fmap = {}
    _fund_cache[mk] = (now, fmap)
    return fmap


def _normalize_symbol(label: str | None) -> str | None:
    """Series labels arrive as 'nasdaq/AAPL' / 'sp500/JPM' (training) or plain
    'AAPL' (runner context / per-symbol instances) — reduce to the plain
    symbol used as the stock_fundamentals key."""
    if not label:
        return None
    return label.rsplit("/", 1)[-1] or None


def _raw_fund_vector(fields: dict | None) -> np.ndarray:
    """Fundamentals dict → raw FUND_DIM vector (np.nan for missing).

    market_cap is log10-scaled so it lives on a comparable scale.
    """
    vec = np.full(FUND_DIM, np.nan, dtype=np.float64)
    if not fields:
        return vec
    for i, name in enumerate(_fundamental_fields()):
        val = fields.get(name)
        if val is None:
            continue
        try:
            f = float(val)
        except (TypeError, ValueError):
            continue
        if not math.isfinite(f):
            continue
        if name == "market_cap":
            f = math.log10(max(f, 1.0))
        vec[i] = f
    return vec


# ---------------------------------------------------------------------------
# Network
# ---------------------------------------------------------------------------

def _build_model(seq_len: int = SEQ_LEN, patch_len: int = PATCH_LEN,
                 d_model: int = D_MODEL, n_heads: int = N_HEADS,
                 n_layers: int = N_LAYERS, ff_dim: int = FF_DIM,
                 fund_dim: int = FUND_DIM, dropout: float = DROPOUT,
                 head: str = HEAD_DUAL):
    """Build a fresh PatchTST-lite model.

    head="dual" (default): shared trunk → (return, direction-logit) tuple.
    head="single": legacy regression-only layout — kept so checkpoints saved
    before the dual-head upgrade still load (state-dict keys must match).
    """
    try:
        import torch
        import torch.nn as nn
    except ImportError as exc:
        raise RuntimeError("PyTorch not available") from exc

    n_patches = seq_len // patch_len

    class _PatchTransformer(nn.Module):
        def __init__(self) -> None:
            super().__init__()
            self.patch_proj = nn.Linear(patch_len, d_model)
            self.pos_emb = nn.Parameter(torch.zeros(1, n_patches, d_model))
            enc_layer = nn.TransformerEncoderLayer(
                d_model=d_model, nhead=n_heads, dim_feedforward=ff_dim,
                dropout=dropout, batch_first=True, norm_first=True,
            )
            self.encoder = nn.TransformerEncoder(enc_layer, num_layers=n_layers)
            self.fund_proj = nn.Sequential(nn.Linear(fund_dim, FUND_HIDDEN), nn.ReLU())
            if head == HEAD_SINGLE:
                self.head = nn.Sequential(
                    nn.Linear(d_model + FUND_HIDDEN, 64),
                    nn.ReLU(),
                    nn.Dropout(dropout),
                    nn.Linear(64, 1),
                )
            else:
                self.head_shared = nn.Sequential(
                    nn.Linear(d_model + FUND_HIDDEN, 64),
                    nn.ReLU(),
                    nn.Dropout(dropout),
                )
                self.head_ret = nn.Linear(64, 1)
                self.head_dir = nn.Linear(64, 1)

        def forward(self, x, fund):
            # x: (B, seq_len) standardized returns; fund: (B, fund_dim)
            b = x.shape[0]
            patches = x.view(b, n_patches, patch_len)
            h = self.patch_proj(patches) + self.pos_emb
            h = self.encoder(h)
            h = h.mean(dim=1)
            f = self.fund_proj(fund)
            z = torch.cat([h, f], dim=1)
            if head == HEAD_SINGLE:
                return self.head(z).squeeze(-1)
            s = self.head_shared(z)
            return self.head_ret(s).squeeze(-1), self.head_dir(s).squeeze(-1)

    return _PatchTransformer()


def _forward(model, x, fund):
    """Normalize model output to (return_pred, direction_logit_or_None)."""
    out = model(x, fund)
    if isinstance(out, tuple):
        return out
    return out, None


# ---------------------------------------------------------------------------
# Main class
# ---------------------------------------------------------------------------

class TransformerPredictor(PredictionAlgorithm):
    """PatchTST-lite Transformer price predictor with fundamentals context."""

    def __init__(self) -> None:
        self._model = None
        self._r_mean: float = 0.0     # train-set log-return mean
        self._r_std: float = 1.0      # train-set log-return std
        self._fund_mean: np.ndarray = np.zeros(FUND_DIM, dtype=np.float64)
        self._fund_std: np.ndarray = np.ones(FUND_DIM, dtype=np.float64)
        self._trained: bool = False
        self._load_attempted: bool = False

    def get_name(self) -> str:
        return "Transformer (PatchTST)"

    def get_key(self) -> str:
        return "transformer_nn"

    def is_trained(self) -> bool:
        return self._trained

    # ------------------------------------------------------------------
    # Checkpoint I/O
    # ------------------------------------------------------------------

    def _checkpoint_file(self) -> str:
        return _tf_checkpoint_path(self._market_key, getattr(self, "_symbol_key", None))

    def _try_load_checkpoint(self) -> bool:
        path = self._checkpoint_file()
        # Ensure local (downloads from S3 if backend=s3 and file missing locally).
        path = get_store().ensure_local(path)
        if not os.path.exists(path):
            return False
        try:
            import torch
            data = torch.load(path, map_location="cpu", weights_only=False)
            cfg = data.get("config", {})
            model = _build_model(
                seq_len=cfg.get("seq_len", SEQ_LEN),
                patch_len=cfg.get("patch_len", PATCH_LEN),
                d_model=cfg.get("d_model", D_MODEL),
                n_heads=cfg.get("n_heads", N_HEADS),
                n_layers=cfg.get("n_layers", N_LAYERS),
                ff_dim=cfg.get("ff_dim", FF_DIM),
                fund_dim=cfg.get("fund_dim", FUND_DIM),
                dropout=cfg.get("dropout", DROPOUT),
                head=cfg.get("head", HEAD_SINGLE),   # pre-dual checkpoints = single
            )
            model.load_state_dict(data["state_dict"])
            model.eval()
            self._model = model
            self._r_mean = float(data.get("r_mean", 0.0))
            self._r_std = float(data.get("r_std", 1.0)) or 1.0
            self._fund_mean = np.array(data.get("fund_mean", np.zeros(FUND_DIM)), dtype=np.float64)
            self._fund_std = np.array(data.get("fund_std", np.ones(FUND_DIM)), dtype=np.float64)
            self._trained = True
            log.info("transformer.checkpoint.loaded", market=self._market_key, path=path)
            return True
        except Exception as exc:
            log.warning("transformer.checkpoint.load_failed", market=self._market_key,
                        path=path, error=str(exc))
            return False

    def _save_checkpoint(self) -> None:
        path = self._checkpoint_file()
        try:
            import torch
            os.makedirs(os.path.dirname(path), exist_ok=True)
            torch.save({
                "state_dict": self._model.state_dict(),
                "config": {
                    "seq_len": SEQ_LEN, "patch_len": PATCH_LEN, "d_model": D_MODEL,
                    "n_heads": N_HEADS, "n_layers": N_LAYERS, "ff_dim": FF_DIM,
                    "fund_dim": FUND_DIM, "dropout": DROPOUT, "head": HEAD_DUAL,
                },
                "r_mean": self._r_mean,
                "r_std": self._r_std,
                "fund_mean": self._fund_mean,
                "fund_std": self._fund_std,
                "version": 1,
            }, path)
            log.info("transformer.checkpoint.saved", market=self._market_key, path=path)
            # Upload to S3/MinIO (no-op for local backend).
            get_store().upload_if_remote(path)
        except Exception as exc:
            log.warning("transformer.checkpoint.save_failed", market=self._market_key,
                        path=path, error=str(exc))

    # ------------------------------------------------------------------
    # Fundamentals
    # ------------------------------------------------------------------

    def _fund_vector_for_symbol(self, symbol: str | None) -> np.ndarray:
        """Standardized fundamentals vector for a symbol (zeros when unknown)."""
        sym = _normalize_symbol(symbol)
        raw = _raw_fund_vector(
            _get_fundamentals_for_market(self._market_key).get(sym) if sym else None
        )
        std = np.where(self._fund_std > 1e-12, self._fund_std, 1.0)
        z = (raw - self._fund_mean) / std
        return np.nan_to_num(z, nan=0.0, posinf=0.0, neginf=0.0).astype(np.float32)

    # ------------------------------------------------------------------
    # Training
    # ------------------------------------------------------------------

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Unlabeled entry point — uses _symbol_key (per-symbol instances) as the label."""
        label = getattr(self, "_symbol_key", None)
        self.train_batch_labeled([(p, v, label) for p, v in series])

    def train_batch_labeled(
        self, series: list[tuple[list[float], "list[float] | None", "str | None"]]
    ) -> None:
        """Train pooled model on all labeled series; save checkpoint.

        Each element is (prices ASC, volumes-or-None, symbol-or-None).  The
        symbol joins the series to its fundamentals snapshot; None → zeros.
        """
        try:
            import torch
            import torch.nn.functional as F
        except ImportError as exc:
            log.warning("transformer.train.no_torch", error=str(exc))
            return

        # ── Collect returns + per-series fundamentals ──────────────────────
        usable: list[tuple[np.ndarray, np.ndarray]] = []   # (returns, raw_fund)
        all_returns: list[np.ndarray] = []
        for prices, _vols, label in series:
            if len(prices) < MIN_DATA_POINTS_TF:
                continue
            arr = np.asarray(prices, dtype=np.float64)
            if np.any(arr <= 0):
                arr = np.maximum(arr, 1e-9)
            rets = np.diff(np.log(arr))
            sym = _normalize_symbol(label)
            fund_raw = _raw_fund_vector(
                _get_fundamentals_for_market(self._market_key).get(sym) if sym else None
            )
            usable.append((rets, fund_raw))
            all_returns.append(rets)

        if not usable:
            log.warning("transformer.train.skip", market=self._market_key,
                        reason="no series with enough data")
            return

        # ── Standardization stats (returns + fundamentals) ────────────────
        cat = np.concatenate(all_returns)
        self._r_mean = float(np.mean(cat))
        self._r_std = float(np.std(cat)) or 1.0

        fund_matrix = np.stack([f for _, f in usable])
        # GOLD/CRYPTO have no fundamentals → all-NaN columns; silence the
        # empty-slice RuntimeWarnings, the isfinite guards below handle them.
        import warnings as _warnings
        with _warnings.catch_warnings():
            _warnings.simplefilter("ignore", RuntimeWarning)
            fmean = np.nanmean(fund_matrix, axis=0)
            fstd = np.nanstd(fund_matrix, axis=0)
        self._fund_mean = np.where(np.isfinite(fmean), fmean, 0.0)
        self._fund_std = np.where(np.isfinite(fstd) & (fstd > 1e-12), fstd, 1.0)

        # ── Build windows with chronological per-series val split ─────────
        Xtr, Ftr, ytr, Xva, Fva, yva = [], [], [], [], [], []
        for rets, fund_raw in usable:
            z = (rets - self._r_mean) / self._r_std
            fz = (fund_raw - self._fund_mean) / self._fund_std
            fz = np.nan_to_num(fz, nan=0.0, posinf=0.0, neginf=0.0).astype(np.float32)
            n_windows = len(z) - SEQ_LEN
            if n_windows <= 0:
                continue
            n_val = max(1, int(n_windows * VAL_FRACTION))
            for i in range(n_windows):
                x = z[i: i + SEQ_LEN].astype(np.float32)
                y = np.float32(z[i + SEQ_LEN])
                if i >= n_windows - n_val:
                    Xva.append(x); Fva.append(fz); yva.append(y)
                else:
                    Xtr.append(x); Ftr.append(fz); ytr.append(y)

        if len(Xtr) < MIN_WINDOWS:
            log.warning("transformer.train.skip", market=self._market_key,
                        reason=f"too few windows ({len(Xtr)})")
            return

        Xtr_t = torch.tensor(np.array(Xtr)); Ftr_t = torch.tensor(np.array(Ftr))
        ytr_t = torch.tensor(np.array(ytr))
        Xva_t = torch.tensor(np.array(Xva)); Fva_t = torch.tensor(np.array(Fva))
        yva_t = torch.tensor(np.array(yva))

        model = _build_model()
        optimizer = torch.optim.AdamW(model.parameters(), lr=LR, weight_decay=WEIGHT_DECAY)

        best_val = float("inf")
        best_state = None
        best_epoch = 0
        patience = 0
        n = len(Xtr_t)

        # Label on raw-return sign (ret > 0), the metric the system scores —
        # NOT standardized sign (ret > mean). See _direction_targets.
        ytr_dir = _direction_targets(ytr_t, self._r_mean, self._r_std).float()
        yva_dir = _direction_targets(yva_t, self._r_mean, self._r_std).float()

        for epoch in range(EPOCHS):
            model.train()
            perm = torch.randperm(n)
            epoch_loss = 0.0
            for start in range(0, n, BATCH_SIZE):
                idx = perm[start: start + BATCH_SIZE]
                pred, logit = _forward(model, Xtr_t[idx], Ftr_t[idx])
                loss = F.huber_loss(pred, ytr_t[idx])
                if logit is not None:
                    # Joint objective: the system SCORES direction, so train it
                    loss = loss + DIR_LOSS_WEIGHT * F.binary_cross_entropy_with_logits(
                        logit, ytr_dir[idx]
                    )
                optimizer.zero_grad()
                loss.backward()
                torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
                optimizer.step()
                epoch_loss += float(loss.item()) * len(idx)

            model.eval()
            with torch.no_grad():
                val_pred, val_logit = _forward(model, Xva_t, Fva_t)
                val_loss = float(F.huber_loss(val_pred, yva_t).item())
                if val_logit is not None:
                    val_loss += DIR_LOSS_WEIGHT * float(
                        F.binary_cross_entropy_with_logits(val_logit, yva_dir).item()
                    )
                    dir_acc = float(((val_logit > 0) == yva_dir.bool()).float().mean().item())
                else:
                    dir_acc = float(((val_pred > 0) == yva_dir.bool()).float().mean().item())

            if val_loss < best_val - 1e-6:
                best_val = val_loss
                best_state = {k: v.detach().clone() for k, v in model.state_dict().items()}
                best_epoch = epoch + 1
                patience = 0
            else:
                patience += 1

            if (epoch + 1) % 10 == 0 or patience >= EARLY_STOP_PATIENCE:
                log.info(
                    "transformer.train.epoch",
                    market=self._market_key, epoch=epoch + 1,
                    train_loss=round(epoch_loss / max(n, 1), 6),
                    val_loss=round(val_loss, 6),
                    val_dir_acc=round(dir_acc, 4),
                    best_epoch=best_epoch,
                )
            if patience >= EARLY_STOP_PATIENCE:
                break

        if best_state is not None:
            model.load_state_dict(best_state)
        model.eval()
        self._model = model
        self._trained = True

        with torch.no_grad():
            val_pred, val_logit = _forward(model, Xva_t, Fva_t)
            dir_src = val_logit if val_logit is not None else val_pred
            final_dir = float(((dir_src > 0) == yva_dir.bool()).float().mean().item())
        log.info(
            "transformer.train.done",
            market=self._market_key, series=len(usable),
            train_windows=len(Xtr), val_windows=len(Xva),
            best_epoch=best_epoch, best_val_loss=round(best_val, 6),
            val_dir_acc=round(final_dir, 4),
        )

        self._save_checkpoint()

    # ------------------------------------------------------------------
    # Prediction
    # ------------------------------------------------------------------

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS_TF:
            raise ValueError(
                f"Transformer needs at least {MIN_DATA_POINTS_TF} points, got {len(prices)}"
            )

        # Lazy checkpoint load, once
        if self._model is None and not self._load_attempted:
            self._load_attempted = True
            self._try_load_checkpoint()

        try:
            if self._model is not None:
                return self._inference(prices)
            return self._quick_train_and_predict(prices)
        except Exception as exc:
            log.error(
                "algo.transformer_nn.failed",
                market=self._market_key,
                error=str(exc),
                exc_info=True,
            )
            raise

    def _inference(self, prices: list[float]) -> PredictionResult:
        import torch

        arr = np.maximum(np.asarray(prices, dtype=np.float64), 1e-9)
        current = float(arr[-1])
        rets = np.diff(np.log(arr))

        z = (rets - self._r_mean) / self._r_std
        x = torch.tensor(z[-SEQ_LEN:].astype(np.float32)).unsqueeze(0)

        symbol = getattr(self, "_context_symbol", None) or getattr(self, "_symbol_key", None)
        fund = torch.tensor(self._fund_vector_for_symbol(symbol)).unsqueeze(0)

        self._model.eval()
        with torch.no_grad():
            ret_out, logit_out = _forward(self._model, x, fund)
            y_hat = float(ret_out.item())
            p_up = float(torch.sigmoid(logit_out).item()) if logit_out is not None else None

        # Un-standardize and clip to a sane multiple of the train-set σ
        r_hat = y_hat * self._r_std + self._r_mean
        r_hat = float(np.clip(r_hat, -4.0 * self._r_std, 4.0 * self._r_std))

        skip = False
        if p_up is not None:
            # Direction comes from the classification head (trained on the
            # scored metric); the regression head supplies the magnitude.
            direction = 1.0 if p_up >= 0.5 else -1.0
            r_hat = direction * abs(r_hat)
            conf = float(min(0.85, max(0.35, 0.35 + abs(p_up - 0.5))))
            skip = abs(p_up - 0.5) < SKIP_GAP   # coin-flip → don't persist
        else:
            # Legacy single-head checkpoint: |r̂| relative to recent vol
            recent = rets[-20:] if len(rets) >= 20 else rets
            sigma = float(np.std(recent)) if len(recent) > 1 else self._r_std
            conf = 0.45 + 0.40 * math.tanh(2.0 * abs(r_hat) / (sigma + 1e-9))
            conf = float(min(0.85, max(0.35, conf)))

        predicted = current * math.exp(r_hat)
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))

        return PredictionResult(
            predicted_price=predicted,
            confidence=conf,
            current_price=current,
            algorithm_name=self.get_key(),
            skip_write=skip,
        )

    def predict_direction_proba(self, prices: list[float]) -> float | None:
        """Return P(up) from the direction head, or None when unavailable.

        None means: no checkpoint, legacy single-head checkpoint, not enough
        data, or any inference error — callers (conviction bots) treat None
        as HOLD. No quick-train fallback here: trading decisions should only
        come from a properly trained pooled model.
        """
        if len(prices) < MIN_DATA_POINTS_TF:
            return None
        if self._model is None and not self._load_attempted:
            self._load_attempted = True
            self._try_load_checkpoint()
        if self._model is None:
            return None
        try:
            import torch
            arr = np.maximum(np.asarray(prices, dtype=np.float64), 1e-9)
            rets = np.diff(np.log(arr))
            z = (rets - self._r_mean) / self._r_std
            x = torch.tensor(z[-SEQ_LEN:].astype(np.float32)).unsqueeze(0)
            symbol = getattr(self, "_context_symbol", None) or getattr(self, "_symbol_key", None)
            fund = torch.tensor(self._fund_vector_for_symbol(symbol)).unsqueeze(0)
            self._model.eval()
            with torch.no_grad():
                _, logit = _forward(self._model, x, fund)
            if logit is None:   # legacy single-head checkpoint
                return None
            return float(torch.sigmoid(logit).item())
        except Exception as exc:
            log.warning("transformer.direction_proba.failed", error=str(exc))
            return None

    def _quick_train_and_predict(self, prices: list[float]) -> PredictionResult:
        """Cold start: brief single-series training (no fundamentals), then predict.

        Mirrors the LSTM/GRU pattern; the quick model is NOT cached or saved so
        the next scheduled train_batch produces the real pooled checkpoint.
        """
        import torch
        import torch.nn.functional as F

        arr = np.maximum(np.asarray(prices, dtype=np.float64), 1e-9)
        rets = np.diff(np.log(arr))
        r_mean = float(np.mean(rets))
        r_std = float(np.std(rets)) or 1.0
        z = ((rets - r_mean) / r_std).astype(np.float32)

        n_windows = len(z) - SEQ_LEN
        if n_windows < 4:
            raise ValueError(f"Not enough windows for quick training ({n_windows})")

        X = np.stack([z[i: i + SEQ_LEN] for i in range(n_windows)])
        y = np.array([z[i + SEQ_LEN] for i in range(n_windows)], dtype=np.float32)
        fund = np.zeros((n_windows, FUND_DIM), dtype=np.float32)

        X_t = torch.tensor(X); y_t = torch.tensor(y); f_t = torch.tensor(fund)

        model = _build_model()
        optimizer = torch.optim.AdamW(model.parameters(), lr=LR, weight_decay=WEIGHT_DECAY)
        y_dir = _direction_targets(y_t, r_mean, r_std).float()  # raw-return sign
        model.train()
        for _ in range(QUICK_EPOCHS):
            pred, logit = _forward(model, X_t, f_t)
            loss = F.huber_loss(pred, y_t)
            if logit is not None:
                loss = loss + DIR_LOSS_WEIGHT * F.binary_cross_entropy_with_logits(logit, y_dir)
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()

        model.eval()
        current = float(arr[-1])
        with torch.no_grad():
            ret_out, logit_out = _forward(
                model,
                torch.tensor(z[-SEQ_LEN:]).unsqueeze(0),
                torch.zeros(1, FUND_DIM),
            )
            y_hat = float(ret_out.item())
            if logit_out is not None and float(torch.sigmoid(logit_out).item()) < 0.5:
                y_hat = -abs(y_hat)
            elif logit_out is not None:
                y_hat = abs(y_hat)

        r_hat = float(np.clip(y_hat * r_std + r_mean, -4.0 * r_std, 4.0 * r_std))
        predicted = current * math.exp(r_hat)
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))

        return PredictionResult(
            predicted_price=predicted,
            confidence=0.45,   # cold-start model — low fixed confidence
            current_price=current,
            algorithm_name=self.get_key(),
        )
