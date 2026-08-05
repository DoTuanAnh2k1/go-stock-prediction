"""RL DQN prediction algorithm — Deep Q-Network written in PyTorch.

Acts as both a prediction algorithm (algorithm #12) and a trading bot policy.

Architecture (v3):
  Dueling MLP: LayerNorm(in) → 128 → 64 → {V(s), A(s,a)};  Q = V + A − mean(A).
  Legacy plain MLP (in → 128 → 64 → 3) is still supported for loading old
  checkpoints (arch tag stored in the checkpoint; missing tag = legacy "mlp").

Training:
  DQN with prioritized replay buffer, target network (Polyak soft update),
  ε-greedy decay per environment-step, Huber loss, Adam optimizer, Double-DQN,
  3-step returns.

  Improvements over v2:
  - Observation↔reward ALIGNMENT FIX — enhanced-feature row j is as-of price
    index j+20, but windows used to pair obs[j] with prices[j], rewarding the
    agent on returns 20 steps stale.  _make_windows now tail-aligns prices to
    the feature rows before slicing.
  - Static feature scaling — RSI/StochRSI (0-100) and MA ratios (~1.0) are
    rescaled to the same order of magnitude as log-returns before entering the
    network (plus a learned input LayerNorm in the dueling arch).  Scaling is
    tagged in the checkpoint (feat_scale) so legacy checkpoints keep receiving
    raw features.
  - Dueling architecture — separates state value from action advantages;
    stabilises Q estimates when hold/buy/sell values are close (flat markets).
  - Prioritized experience replay (proportional, α=0.6, β=0.4) with
    importance-sampling weights — focuses updates on high-TD-error steps.
  - 3-step returns — faster credit propagation of the dense reward.
  - Volatility-normalized reward — each step's PnL reward is divided by the
    rolling σ of log-returns, so crypto and gold series produce comparable
    reward scales (critical for pooled training) — clipped to ±REWARD_CLIP.
  - Directional shaping term — a small ±SHAPING_BETA · direction(action) ·
    return/σ bonus teaches Q(sell|flat) vs Q(buy|flat) to encode direction
    even though sell-when-flat is a no-op trade.  This is what makes
    predict()'s action→direction mapping meaningful and breaks the
    Q(sell|flat) ≈ Q(hold|flat) degeneracy of v2.
  - Walk-forward validation — the most recent window of every real series is
    held out; after each epoch the policy is greedily scored (RAW reward) on
    the holdout and the best epoch's weights are kept (early stopping).
    Mirror windows are built from the training region only.
  - Polyak soft target update (τ=TAU) replaces the hard copy every 50 steps.
  - Mirror/inverted-return augmentation and sliding-window episodes retained
    from v2.

Persistence:
  Checkpoint saved to ${RL_MODEL_DIR}/rl_dqn_{market_key}.pt after train_batch().
  Loaded lazily on first act() / predict() call.  v3 checkpoints carry
  {"arch": "dueling", "feat_scale": True}; legacy files load as plain MLP with
  raw features — no retrain required to keep serving old checkpoints.

Fallback:
  If PyTorch is unavailable, checkpoint is missing, or any inference error occurs,
  the method falls back to a simple EMA — identical pattern to lstm.py / gru.py.
"""
from __future__ import annotations

import copy
import os
import random
from collections import deque
from typing import Deque

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct
from src.algorithms.features import MIN_DATA_POINTS, build_enhanced_features
from src.storage.model_store import get_store
from src.utils.logger import get_logger

log = get_logger("rl_dqn")

# ---------------------------------------------------------------------------
# Hyper-parameters
# ---------------------------------------------------------------------------
HIDDEN1 = 128
HIDDEN2 = 64
N_ACTIONS = 3           # 0=hold, 1=buy, 2=sell

# Position-state features appended to the enhanced observation
N_POS_FEATURES = 3      # holding_flag, unrealized_pnl_pct, holding_days_norm

# Network architectures (stored in checkpoints; missing tag = legacy "mlp")
ARCH_MLP = "mlp"
ARCH_DUELING = "dueling"
DEFAULT_ARCH = ARCH_DUELING

# DQN training
REPLAY_CAPACITY = 50_000
BATCH_SIZE = 64
GAMMA = 0.99
N_STEP = 3              # n-step return horizon
LR_DQN = 5e-4
EPSILON_START = 1.0
EPSILON_END = 0.05
# Legacy fixed exploration horizon (fallback only). The actual epsilon-decay
# horizon is computed PER MARKET in train_batch() as ~60% of that market's real
# env-step budget, so low-data markets (e.g. GOLD) still anneal ε to the floor
# instead of staying near-random. Kept for backward compatibility / reference.
TARGET_EXPLORE_STEPS = 15_000
TAU = 0.005                 # Polyak soft-update coefficient for the target net
TRAIN_EPOCHS = 12           # passes over the full (windowed + augmented) episode set
MIN_REPLAY = 200            # start gradient updates after this many transitions

# Prioritized replay
PER_ALPHA = 0.6             # priority exponent
PER_BETA = 0.4              # importance-sampling exponent
PER_EPS = 1e-3              # floor added to |TD| so no transition starves

# Sliding-window episode parameters
EPISODE_WINDOW = 130        # steps per sliding-window episode
EPISODE_STRIDE = 35         # stride between consecutive windows
MAX_EPISODES_PER_MARKET = 400   # cap total episodes (real + mirror) to bound time

# Per-step penalty for holding a long position — prevents "always long" policy.
# Value is very small (1/10 of a typical transaction cost) to avoid overriding
# the primary dense reward signal.
HOLD_LONG_PENALTY = 0.0001

# Reward shaping / normalization
SHAPING_BETA = 0.15         # weight of the directional-correctness shaping term
REWARD_NORM_EPS = 1e-4      # floor added to rolling σ in the reward denominator
REWARD_CLIP = 10.0          # clip normalized per-step reward to ±this

# Prediction magnitude
K_SIGMA = 1.5             # predicted change = K_SIGMA * rolling_σ of log-returns
SIGMA_WINDOW = 20         # window for rolling σ

# The canonical enhanced-feature count from features.py (both the pandas-ta
# and the numpy-fallback builders emit exactly 30 columns).  Static feature
# scaling is applied only when this count matches.
N_ENHANCED_FEATURES = 30

# Checkpoint directory env var
_DEFAULT_MODEL_DIR = "/models"


def _get_model_dir() -> str:
    """Read RL_MODEL_DIR from environment (falls back to config if available)."""
    env = os.environ.get("RL_MODEL_DIR")
    if env:
        return env
    try:
        from src.config import get_settings
        return get_settings().rl_model_dir
    except Exception:
        return _DEFAULT_MODEL_DIR


def _checkpoint_path(market_key: str, symbol_key: str | None = None) -> str:
    mdir = _get_model_dir()
    # Normalise market key: NASDAQ100 → NASDAQ100, etc.
    safe_market = (market_key or "UNKNOWN").upper().replace("/", "_")
    if symbol_key:
        # Sanitize symbol for safe filesystem use (replace path separators and spaces)
        safe_symbol = symbol_key.replace(os.sep, "_").replace("/", "_").replace(" ", "_")
        return os.path.join(mdir, f"rl_dqn_{safe_market}_{safe_symbol}.pt")
    return os.path.join(mdir, f"rl_dqn_{safe_market}.pt")


# ---------------------------------------------------------------------------
# Q-Networks
# ---------------------------------------------------------------------------

def _build_qnet(in_dim: int, arch: str = DEFAULT_ARCH):
    """Build a fresh Q-network.

    arch="dueling" (default): LayerNorm(in) → 128 → 64 → dueling V/A heads.
    arch="mlp": legacy plain MLP in → 128 → 64 → 3 — kept so checkpoints saved
    before the dueling upgrade still load (state-dict key layout must match).
    """
    try:
        import torch.nn as nn
    except ImportError as exc:
        raise RuntimeError("PyTorch not available") from exc

    if arch == ARCH_MLP:
        class _QNet(nn.Module):
            def __init__(self, d: int) -> None:
                super().__init__()
                self.net = nn.Sequential(
                    nn.Linear(d, HIDDEN1),
                    nn.ReLU(),
                    nn.Linear(HIDDEN1, HIDDEN2),
                    nn.ReLU(),
                    nn.Linear(HIDDEN2, N_ACTIONS),
                )

            def forward(self, x):
                return self.net(x)

        return _QNet(in_dim)

    class _DuelingQNet(nn.Module):
        def __init__(self, d: int) -> None:
            super().__init__()
            self.norm = nn.LayerNorm(d)
            self.trunk = nn.Sequential(
                nn.Linear(d, HIDDEN1),
                nn.ReLU(),
                nn.Linear(HIDDEN1, HIDDEN2),
                nn.ReLU(),
            )
            self.value = nn.Linear(HIDDEN2, 1)
            self.advantage = nn.Linear(HIDDEN2, N_ACTIONS)

        def forward(self, x):
            h = self.trunk(self.norm(x))
            v = self.value(h)
            a = self.advantage(h)
            return v + a - a.mean(dim=1, keepdim=True)

    return _DuelingQNet(in_dim)


# ---------------------------------------------------------------------------
# Replay Buffer — prioritized (proportional) with legacy uniform API
# ---------------------------------------------------------------------------

class _ReplayBuffer:
    """Ring replay buffer with proportional prioritized sampling.

    push()/sample()/__len__ keep the original uniform API; training uses
    sample_per() + update_priorities().  The 5th tuple field is stored
    verbatim — training pushes a float bootstrap discount (0.0 = terminal,
    γ^n = n-step bootstrap), legacy callers may push a bool done flag.
    """

    def __init__(self, capacity: int) -> None:
        self._capacity = capacity
        self._data: list = []
        self._pos = 0
        self._prio = np.zeros(capacity, dtype=np.float64)
        self._max_prio = 1.0

    def push(self, state, action: int, reward: float, next_state, done) -> None:
        entry = (state, action, reward, next_state, done)
        if len(self._data) < self._capacity:
            self._data.append(entry)
        else:
            self._data[self._pos] = entry
        self._prio[self._pos] = self._max_prio
        self._pos = (self._pos + 1) % self._capacity

    def sample(self, n: int):
        """Uniform sample (legacy API)."""
        return random.sample(self._data, n)

    def sample_per(self, n: int):
        """Prioritized sample → (batch, indices, importance-sampling weights)."""
        size = len(self._data)
        probs = self._prio[:size] ** PER_ALPHA
        probs = probs / probs.sum()
        idx = np.random.choice(size, size=n, p=probs, replace=size < n)
        weights = (size * probs[idx]) ** (-PER_BETA)
        weights = weights / weights.max()
        batch = [self._data[i] for i in idx]
        return batch, idx, weights.astype(np.float32)

    def update_priorities(self, indices, td_errors) -> None:
        for i, td in zip(indices, td_errors):
            prio = float(abs(td)) + PER_EPS
            self._prio[int(i)] = prio
            if prio > self._max_prio:
                self._max_prio = prio

    def __len__(self) -> int:
        return len(self._data)


# ---------------------------------------------------------------------------
# Mirror / augmentation helpers
# ---------------------------------------------------------------------------

def _build_mirror_prices(prices: np.ndarray) -> np.ndarray:
    """Build a mirrored price series by negating log-returns.

    Given prices p[0..T], compute log-returns r[t] = log(p[t]/p[t-1]).
    The mirror series keeps p_mirror[0] = p[0] and evolves as:
        p_mirror[t] = p_mirror[t-1] * exp(-r[t])   for t = 1 .. T

    The mirror series has the SAME amplitude of moves as the original but
    in the OPPOSITE direction: every uptrend becomes a downtrend and vice
    versa.  Feeding both the original and its mirror to the agent ensures
    roughly 50/50 up/down exposure regardless of the market's historical bias.
    """
    if len(prices) < 2:
        return prices.copy()
    log_ret = np.diff(np.log(prices.astype(np.float64) + 1e-12))
    mirror = np.empty(len(prices), dtype=np.float32)
    mirror[0] = prices[0]
    for t in range(1, len(prices)):
        mirror[t] = mirror[t - 1] * np.exp(-log_ret[t - 1])
    # Ensure strictly positive
    mirror = np.maximum(mirror, 1e-6)
    return mirror.astype(np.float32)


def _scale_feature_matrix(mat: np.ndarray) -> np.ndarray:
    """Statically rescale the canonical 30 enhanced features to comparable scale.

    Without this, RSI/StochRSI (0-100) dwarf log-returns (~0.001) in the MLP
    input.  Column layout follows build_enhanced_features():
      0-9   lag returns          — already small, untouched
      10-13 MA ratios (~1.0)     — centered by −1.0
      14-16 multi-tf returns     — untouched
      17    RSI (0-100)          — /100 − 0.5
      18-19 StochRSI %K/%D       — /100 − 0.5
      20    Bollinger %B (0-1)   — −0.5
      21-22 MACD normalised      — untouched
      23-25 rolling std          — untouched
      26    ROC(10) (percent)    — /100
      27-28 momentum             — untouched
      29    volume ratio (~1.0)  — −1.0

    Applied only when the matrix has exactly N_ENHANCED_FEATURES columns
    (both feature builders emit 30); anything else passes through unchanged.
    """
    if mat.ndim != 2 or mat.shape[1] != N_ENHANCED_FEATURES:
        return mat
    x = np.array(mat, dtype=np.float32, copy=True)
    x[:, 10:14] -= 1.0
    x[:, 17:20] = x[:, 17:20] / 100.0 - 0.5
    x[:, 20] -= 0.5
    x[:, 26] /= 100.0
    x[:, 29] -= 1.0
    return x


def _build_obs_matrix(prices: np.ndarray, volumes: np.ndarray | None) -> np.ndarray:
    """Build the (scaled) enhanced-feature matrix for a price series.

    Returns array of shape (T, n_features) where T = len(features).
    Uses build_enhanced_features() then applies the static v3 feature scaling
    — every consumer (train_batch, rl_replay multiseed eval) therefore feeds
    the network consistently-scaled observations.
    """
    feats, _ = build_enhanced_features(prices, volumes)
    return _scale_feature_matrix(np.array(feats, dtype=np.float32))


def _make_windows(
    obs_matrix: np.ndarray,
    prices: np.ndarray,
    window: int,
    stride: int,
) -> list[tuple[np.ndarray, np.ndarray]]:
    """Slice (obs_matrix, prices) into overlapping ALIGNED windows.

    build_enhanced_features starts emitting rows at price index 20, so
    obs_matrix[j] is as-of prices[j + (len(prices) - len(obs_matrix))].
    When the caller passes the full price array we tail-align it first so
    each window pairs obs row j with the price it was computed from —
    otherwise episode rewards are ~20 steps stale relative to the state.

    Each window is (obs_window, prices_window) of length `window`.
    If the series is shorter than `window`, return it as a single episode.
    """
    n = len(obs_matrix)
    if len(prices) > n:
        prices = prices[len(prices) - n:]

    if n < window:
        return [(obs_matrix, prices[:n])]

    windows = []
    start = 0
    while start + window <= n:
        windows.append((
            obs_matrix[start: start + window],
            prices[start: start + window],
        ))
        start += stride
    # Always include the tail segment if it was not covered exactly
    if start < n and (n - start) >= window // 2:
        windows.append((obs_matrix[n - window: n], prices[n - window: n]))
    return windows


def _rolling_sigma_array(prices: np.ndarray, window: int = SIGMA_WINDOW) -> np.ndarray:
    """Per-step rolling σ of log-returns, aligned to price indices.

    sig[t] = std of the ≤`window` log-returns ENDING at t (strictly past —
    no look-ahead).  Early steps with <5 returns fall back to the window's
    global σ so the reward normalizer never divides by a noisy estimate.
    """
    n = len(prices)
    log_ret = np.diff(np.log(np.asarray(prices, dtype=np.float64) + 1e-12))
    global_sig = float(np.std(log_ret)) if len(log_ret) > 1 else 0.005
    if not np.isfinite(global_sig) or global_sig <= 0:
        global_sig = 0.005

    sig = np.full(n, global_sig, dtype=np.float64)
    for t in range(n):
        chunk = log_ret[max(0, t - window): t]
        if len(chunk) >= 5:
            s = float(np.std(chunk))
            if np.isfinite(s) and s > 0:
                sig[t] = s
    return sig


# ---------------------------------------------------------------------------
# Environment helper
# ---------------------------------------------------------------------------

def _run_episode(
    obs_matrix: np.ndarray,
    prices: np.ndarray,
    policy_net,
    buffer: _ReplayBuffer,
    epsilon: float,
    global_step: int,
    transaction_cost: float = 0.0005,
    target_explore_steps: int = TARGET_EXPLORE_STEPS,
) -> tuple[float, int]:
    """Simulate one episode over obs_matrix, collecting n-step transitions.

    Epsilon is computed PER ENVIRONMENT STEP using linear annealing:
        eps(t) = max(END, START - (START - END) * t / target_explore_steps)
    `global_step` is the cumulative env-step count across ALL episodes so far;
    it is updated locally and returned so the caller can persist it.

    Reward (training only — greedy eval keeps RAW economic reward):
        raw     = position·return − cost·|Δposition| − hold_penalty·position
        shaping = SHAPING_BETA · direction(action) · return
        r_t     = clip((raw + shaping) / (σ_t + ε), ±REWARD_CLIP)
    where direction(buy)=+1, direction(sell)=−1, direction(hold)=0.  The
    shaping term gives Q(buy|·) vs Q(sell|·) a directional signal even when
    the action is a positional no-op (sell while flat) — that is what makes
    the predictor role's action→direction mapping learnable.

    Transitions are pushed as n-step tuples: (s_t, a_t, Σ γ^k r_{t+k},
    s_{t+n}, disc) with disc = γ^n for bootstrapped steps and 0.0 at episode
    end (the 5th field is the bootstrap discount, not a bool).

    Position model: flat or long-only (all-in / all-out).
    Returns (total_episode_reward, updated_global_step).
    """
    import torch

    n_steps, n_base = obs_matrix.shape

    sigma = _rolling_sigma_array(prices)

    position: float = 0.0       # 1.0 = long, 0.0 = flat
    entry_price: float = 0.0
    days_held: int = 0
    total_reward = 0.0
    gamma_n = GAMMA ** N_STEP

    # Pending raw steps awaiting n-step aggregation: (state, action, r_norm)
    pending: Deque = deque()

    for t in range(n_steps - 1):
        # Linear epsilon annealing per step
        eps = max(
            EPSILON_END,
            EPSILON_START - (EPSILON_START - EPSILON_END) * global_step / max(target_explore_steps, 1),
        )

        # Build state
        unrealized_pnl = (prices[t] / entry_price - 1.0) if (position > 0 and entry_price > 0) else 0.0
        days_held_norm = min(days_held / 30.0, 1.0)
        pos_feats = np.array([position, unrealized_pnl, days_held_norm], dtype=np.float32)
        state = np.concatenate([obs_matrix[t], pos_feats])

        # ε-greedy action
        if random.random() < eps:
            action = random.randrange(N_ACTIONS)
        else:
            with torch.no_grad():
                q = policy_net(torch.tensor(state, dtype=torch.float32).unsqueeze(0))
                action = int(q.argmax(dim=1).item())

        # Execute action
        prev_position = position
        if action == 1 and position == 0.0:   # BUY
            position = 1.0
            entry_price = prices[t]
            days_held = 0
        elif action == 2 and position == 1.0:  # SELL
            position = 0.0
            days_held = 0
        # action == 0 → HOLD

        # Dense step reward: raw PnL + directional shaping, vol-normalized
        price_return = prices[t + 1] / prices[t] - 1.0
        raw = (
            position * price_return
            - transaction_cost * abs(position - prev_position)
            - HOLD_LONG_PENALTY * position   # small penalty for holding long
        )
        direction = 1.0 if action == 1 else (-1.0 if action == 2 else 0.0)
        shaping = SHAPING_BETA * direction * price_return
        reward = float(np.clip(
            (raw + shaping) / (sigma[t] + REWARD_NORM_EPS),
            -REWARD_CLIP, REWARD_CLIP,
        ))
        total_reward += reward

        # Next state
        days_held += 1 if position > 0 else 0
        next_unrealized = (prices[t + 1] / entry_price - 1.0) if (position > 0 and entry_price > 0) else 0.0
        next_days_norm = min(days_held / 30.0, 1.0)
        next_pos_feats = np.array([position, next_unrealized, next_days_norm], dtype=np.float32)
        next_state = np.concatenate([obs_matrix[t + 1], next_pos_feats])

        done = (t == n_steps - 2)

        # n-step aggregation: emit the oldest pending step once n rewards
        # have accumulated; at episode end flush everything terminal.
        pending.append((state, action, reward))
        if len(pending) == N_STEP:
            s0, a0, _ = pending[0]
            r_n = sum((GAMMA ** k) * pending[k][2] for k in range(len(pending)))
            buffer.push(s0, a0, r_n, next_state, 0.0 if done else gamma_n)
            pending.popleft()
        if done:
            while pending:
                s0, a0, _ = pending[0]
                r_n = sum((GAMMA ** k) * pending[k][2] for k in range(len(pending)))
                buffer.push(s0, a0, r_n, next_state, 0.0)
                pending.popleft()

        global_step += 1

    return total_reward, global_step


def _run_episode_greedy(
    obs_matrix: np.ndarray,
    prices: np.ndarray,
    policy_net,
    transaction_cost: float = 0.0005,
) -> float:
    """Greedy (epsilon=0) evaluation episode — no buffer writes.

    Returns total episode reward under the current policy, in RAW economic
    units (no vol-normalization, no shaping) so scores stay comparable
    across epochs, seeds, and code versions.
    Used for walk-forward validation and post-training evaluation.
    """
    import torch

    n_steps, _ = obs_matrix.shape
    position: float = 0.0
    entry_price: float = 0.0
    days_held: int = 0
    total_reward = 0.0

    for t in range(n_steps - 1):
        unrealized_pnl = (prices[t] / entry_price - 1.0) if (position > 0 and entry_price > 0) else 0.0
        days_held_norm = min(days_held / 30.0, 1.0)
        pos_feats = np.array([position, unrealized_pnl, days_held_norm], dtype=np.float32)
        state = np.concatenate([obs_matrix[t], pos_feats])

        with torch.no_grad():
            q = policy_net(torch.tensor(state, dtype=torch.float32).unsqueeze(0))
            action = int(q.argmax(dim=1).item())

        prev_position = position
        if action == 1 and position == 0.0:
            position = 1.0
            entry_price = prices[t]
            days_held = 0
        elif action == 2 and position == 1.0:
            position = 0.0
            days_held = 0

        price_return = prices[t + 1] / prices[t] - 1.0
        reward = (
            position * price_return
            - transaction_cost * abs(position - prev_position)
            - HOLD_LONG_PENALTY * position
        )
        total_reward += reward
        days_held += 1 if position > 0 else 0

    return total_reward


# ---------------------------------------------------------------------------
# Main class
# ---------------------------------------------------------------------------

class RLDQNPredictor(PredictionAlgorithm):
    """Deep Q-Network prediction algorithm.

    Key points:
    - predict()      : greedy policy (ε=0) → action → price via σ-scaling,
                       magnitude weighted by the softmax buy/sell gap.
    - act(obs)       : shared greedy policy used by both predict() and bot.step().
    - train_batch()  : full DQN training loop; saves checkpoint to disk.
    - is_trained()   : True when a Q-network is loaded / trained.
    - Fallback       : EMA on any failure (same pattern as lstm.py / gru.py).
    """

    def __init__(self) -> None:
        self._qnet = None           # loaded Q-network (eval mode)
        self._in_dim: int = 0       # expected observation dimension
        self._trained: bool = False
        self._arch: str = DEFAULT_ARCH   # arch of the loaded/trained network
        self._feat_scale: bool = False   # network was trained on scaled features

    # ------------------------------------------------------------------
    # Interface
    # ------------------------------------------------------------------

    def get_name(self) -> str:
        return "RL DQN"

    def get_key(self) -> str:
        return "rl_dqn"

    def is_trained(self) -> bool:
        return self._trained

    # ------------------------------------------------------------------
    # Checkpoint I/O
    # ------------------------------------------------------------------

    def _try_load_checkpoint(self) -> bool:
        """Try to load checkpoint for self._market_key (and optionally self._symbol_key). Returns True on success."""
        symbol = getattr(self, "_symbol_key", None)
        path = _checkpoint_path(self._market_key, symbol)
        # Ensure local (downloads from S3 if backend=s3 and file missing locally).
        path = get_store().ensure_local(path)
        if not os.path.exists(path):
            return False
        try:
            import torch
            data = torch.load(path, map_location="cpu", weights_only=False)
            in_dim = data["in_dim"]
            arch = data.get("arch", ARCH_MLP)   # legacy checkpoints = plain MLP
            net = _build_qnet(in_dim, arch=arch)
            net.load_state_dict(data["state_dict"])
            net.eval()
            self._qnet = net
            self._in_dim = in_dim
            self._arch = arch
            self._feat_scale = bool(data.get("feat_scale", False))
            self._trained = True
            log.info("rl_dqn.checkpoint.loaded", market=self._market_key, symbol=symbol,
                     path=path, arch=arch, feat_scale=self._feat_scale)
            return True
        except Exception as exc:
            log.warning("rl_dqn.checkpoint.load_failed", market=self._market_key, symbol=symbol, path=path, error=str(exc))
            return False

    def _save_checkpoint(self, net, in_dim: int) -> None:
        symbol = getattr(self, "_symbol_key", None)
        path = _checkpoint_path(self._market_key, symbol)
        try:
            import torch
            os.makedirs(os.path.dirname(path), exist_ok=True)
            torch.save({
                "in_dim": in_dim,
                "state_dict": net.state_dict(),
                "arch": self._arch,
                "feat_scale": self._feat_scale,
                "version": 3,
            }, path)
            log.info("rl_dqn.checkpoint.saved", market=self._market_key, symbol=symbol,
                     path=path, arch=self._arch)
            # Upload to S3/MinIO (no-op for local backend).
            get_store().upload_if_remote(path)
        except Exception as exc:
            log.warning("rl_dqn.checkpoint.save_failed", market=self._market_key, symbol=symbol, path=path, error=str(exc))

    # ------------------------------------------------------------------
    # act() — shared greedy policy (ε=0)
    # ------------------------------------------------------------------

    def _prep_obs(self, observation: np.ndarray) -> np.ndarray:
        """Convert an observation to the network's input space.

        v3 checkpoints (feat_scale=True) were trained on statically-scaled
        features; legacy checkpoints receive the raw observation unchanged.
        The trailing N_POS_FEATURES position features are never rescaled.
        """
        obs = np.array(observation, dtype=np.float32)
        if (
            self._feat_scale
            and obs.ndim == 1
            and len(obs) - N_POS_FEATURES == N_ENHANCED_FEATURES
        ):
            feat = _scale_feature_matrix(obs[np.newaxis, :N_ENHANCED_FEATURES])[0]
            obs = np.concatenate([feat, obs[N_ENHANCED_FEATURES:]])
        return obs

    def act(self, observation: np.ndarray) -> int:
        """Greedy action selection (ε=0). Returns action ∈ {0, 1, 2}.

        Called by both predict() (flat position observation) and bot.step()
        (real position state appended to enhanced features).

        Lazy-loads checkpoint on first call.
        Returns 0 (hold) on any failure.
        """
        # Lazy-load
        if self._qnet is None:
            if not self._try_load_checkpoint():
                return 0  # hold — no policy available

        try:
            import torch
            obs = self._prep_obs(observation)
            with torch.no_grad():
                q = self._qnet(torch.tensor(obs, dtype=torch.float32).unsqueeze(0))
                return int(q.argmax(dim=1).item())
        except Exception as exc:
            log.warning("rl_dqn.act.failed", error=str(exc))
            return 0  # hold

    # ------------------------------------------------------------------
    # predict()
    # ------------------------------------------------------------------

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        """Predict next price.

        Builds enhanced-feature observation (flat position: no holding),
        calls act() greedy, maps action → predicted_price via σ-scaling,
        then clamps with market-aware limit.
        """
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"RL DQN needs at least {MIN_DATA_POINTS} points, got {len(prices)}")

        try:
            return self._inference(prices, volumes)
        except Exception as exc:
            log.error(
                "algo.rl_dqn.failed",
                market=self._market_key,
                error=str(exc),
                exc_info=True,
            )
            raise

    def _inference(self, prices: list[float], volumes: list[float] | None) -> PredictionResult:
        arr = np.array(prices, dtype=np.float32)
        current = float(arr[-1])

        # Build enhanced features — last row is the inference row
        vol_arr = np.array(volumes, dtype=np.float32) if volumes else None
        obs_matrix, _ = build_enhanced_features(arr, vol_arr)
        if not obs_matrix:
            raise ValueError("build_enhanced_features returned no rows")

        last_feat = np.array(obs_matrix[-1], dtype=np.float32)
        # Append flat position-state (no holding for predictor role)
        pos_state = np.array([0.0, 0.0, 0.0], dtype=np.float32)
        obs = np.concatenate([last_feat, pos_state])

        # Act greedy
        action = self.act(obs)

        # Rolling σ of log-returns (last SIGMA_WINDOW steps)
        log_ret = np.diff(np.log(arr + 1e-12))
        window = log_ret[-SIGMA_WINDOW:] if len(log_ret) >= SIGMA_WINDOW else log_ret
        sigma = float(np.std(window)) if len(window) > 1 else 0.005

        # Softmax buy/sell gap scales the predicted magnitude: a confident
        # policy predicts a full K_SIGMA·σ move, a marginal one much less.
        probs = self._action_probs(obs)
        gap = float(probs[1] - probs[2]) if probs is not None else 0.0

        if action == 1:    # BUY → predict up, magnitude ∝ conviction
            strength = min(1.0, max(0.25, gap))
            predicted = current * (1.0 + K_SIGMA * sigma * strength)
        elif action == 2:  # SELL → predict down, magnitude ∝ conviction
            strength = min(1.0, max(0.25, -gap))
            predicted = current * (1.0 - K_SIGMA * sigma * strength)
        else:              # HOLD → nearly flat, nudged toward the dominant side
            lean = 1.0 if gap >= 0 else -1.0
            predicted = current * (1.0 + 0.1 * sigma * lean)

        # Clamp
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))

        # Confidence = softmax probability of chosen action
        confidence = self._softmax_confidence(obs, action)

        return PredictionResult(
            predicted_price=predicted,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    def _action_probs(self, obs: np.ndarray) -> np.ndarray | None:
        """Softmax over Q-values → per-action pseudo-probabilities, or None."""
        if self._qnet is None:
            return None
        try:
            import torch
            import torch.nn.functional as F
            with torch.no_grad():
                x = torch.tensor(self._prep_obs(obs), dtype=torch.float32).unsqueeze(0)
                q = self._qnet(x)
                return F.softmax(q, dim=1)[0].numpy()
        except Exception:
            return None

    def _softmax_confidence(self, obs: np.ndarray, action: int) -> float:
        """Return the softmax probability of the selected action."""
        probs = self._action_probs(obs)
        if probs is None:
            return 0.40
        return float(probs[action])

    # ------------------------------------------------------------------
    # train_batch()
    # ------------------------------------------------------------------

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Train DQN on all price series for the market.

        Strategy (v3):
        1. Build (scaled) obs-matrix for each real series; slice into aligned
           overlapping windows (EPISODE_WINDOW, stride EPISODE_STRIDE).
        2. Hold out the MOST RECENT window of every real series as the
           walk-forward validation set (never trained on).
        3. Build mirror obs-matrices (negated log-returns); mirror windows of
           the validation region are dropped so no mirrored leak reaches it.
        4. Cap total training-episode count at MAX_EPISODES_PER_MARKET.
        5. Run TRAIN_EPOCHS passes; epsilon decays by global_step (linear);
           rewards are vol-normalized + direction-shaped n-step returns.
        6. Prioritized mini-batch Double-DQN updates (Huber loss, IS weights,
           Polyak soft target update).
        7. After each epoch: greedy RAW-reward eval on the validation windows;
           the best epoch's weights are kept (early stopping).
        8. Final greedy evaluation on all real windows; save checkpoint.
        """
        if len(series) == 0:
            return

        try:
            import torch
            import torch.nn.functional as F
        except ImportError as exc:
            log.warning("rl_dqn.train.no_torch", error=str(exc))
            return

        # ------------------------------------------------------------------
        # Build episodes: real train / validation holdout / mirror train
        # ------------------------------------------------------------------
        real_train: list[tuple[np.ndarray, np.ndarray]] = []    # (obs_mat, prices)
        val_episodes: list[tuple[np.ndarray, np.ndarray]] = []  # most recent window per series
        mirror_train: list[tuple[np.ndarray, np.ndarray]] = []

        for prices_list, vols_list in series:
            if len(prices_list) < MIN_DATA_POINTS:
                continue
            arr = np.array(prices_list, dtype=np.float32)
            vol_arr = np.array(vols_list, dtype=np.float32) if vols_list else None

            # Real series — last window (most recent data) goes to validation
            try:
                mat = _build_obs_matrix(arr, vol_arr)
                if len(mat) == 0:
                    continue
                wins = _make_windows(mat, arr, EPISODE_WINDOW, EPISODE_STRIDE)
            except Exception as exc:
                log.warning("rl_dqn.train.obs_failed", series="real", error=str(exc))
                continue
            if len(wins) >= 2:
                val_episodes.append(wins[-1])
                real_train.extend(wins[:-1])
            else:
                real_train.extend(wins)

            # Mirror series — drop the tail window mirroring the validation zone
            try:
                mirror_arr = _build_mirror_prices(arr)
                # volumes stay the same (mirror only flips price direction)
                mirror_mat = _build_obs_matrix(mirror_arr, vol_arr)
                if len(mirror_mat) > 0:
                    m_wins = _make_windows(mirror_mat, mirror_arr, EPISODE_WINDOW, EPISODE_STRIDE)
                    mirror_train.extend(m_wins[:-1] if len(m_wins) >= 2 else m_wins)
            except Exception as exc:
                log.warning("rl_dqn.train.obs_failed", series="mirror", error=str(exc))

        all_train = real_train + mirror_train

        if not all_train:
            log.warning("rl_dqn.train.skip", reason="no valid series after feature build")
            return

        # Cap total training episodes to bound training time
        if len(all_train) > MAX_EPISODES_PER_MARKET:
            # Shuffle first so the cap samples from the full distribution
            random.shuffle(all_train)
            all_train = all_train[:MAX_EPISODES_PER_MARKET]
            log.info(
                "rl_dqn.train.episodes_capped",
                market=self._market_key,
                capped_to=MAX_EPISODES_PER_MARKET,
            )

        # Fallback when data is too scarce for a holdout: validate in-sample
        if not val_episodes:
            val_episodes = list(real_train)
            log.warning("rl_dqn.train.no_holdout", market=self._market_key,
                        reason="too few windows — validating in-sample")

        # in_dim: enhanced feature count + position features
        in_dim = all_train[0][0].shape[1] + N_POS_FEATURES

        # Adaptive exploration horizon: anneal epsilon over ~60% of the ACTUAL
        # env-steps this market will collect, so low-data markets (e.g. GOLD with
        # few series) still reach the exploitation phase (ε→floor) instead of
        # staying near-random. A fixed-constant horizon under-anneals small markets.
        steps_per_epoch = sum(max(0, m.shape[0] - 1) for m, _ in all_train)
        explore_steps = max(1, int(0.6 * steps_per_epoch * TRAIN_EPOCHS))

        # ------------------------------------------------------------------
        # Build networks
        # ------------------------------------------------------------------
        self._arch = DEFAULT_ARCH
        self._feat_scale = True   # matrices from _build_obs_matrix are scaled
        policy_net = _build_qnet(in_dim, arch=self._arch)
        target_net = _build_qnet(in_dim, arch=self._arch)
        target_net.load_state_dict(policy_net.state_dict())
        target_net.eval()

        optimizer = torch.optim.Adam(policy_net.parameters(), lr=LR_DQN)
        buffer = _ReplayBuffer(REPLAY_CAPACITY)

        global_step = 0      # cumulative env-steps across all episodes + epochs
        grad_step = 0        # cumulative gradient steps
        total_reward = 0.0

        best_val_reward = float("-inf")
        best_state_dict = None
        best_epoch = 0

        # ------------------------------------------------------------------
        # Collect experience: TRAIN_EPOCHS passes over all training episodes
        # ------------------------------------------------------------------
        for epoch in range(TRAIN_EPOCHS):
            epoch_losses: list[float] = []
            # Shuffle episode order each epoch for diversity
            epoch_episodes = all_train.copy()
            random.shuffle(epoch_episodes)

            for obs_mat, prices_arr in epoch_episodes:
                ep_reward, global_step = _run_episode(
                    obs_matrix=obs_mat,
                    prices=prices_arr,
                    policy_net=policy_net,
                    buffer=buffer,
                    epsilon=0.0,            # epsilon is computed inside per-step
                    global_step=global_step,
                    target_explore_steps=explore_steps,
                )
                total_reward += ep_reward

                # Mini-batch updates whenever buffer is large enough
                if len(buffer) >= MIN_REPLAY:
                    n_updates = min(10, len(buffer) // BATCH_SIZE)
                    for _ in range(n_updates):
                        loss_val = self._update_step(policy_net, target_net, buffer, optimizer)
                        epoch_losses.append(loss_val)
                        grad_step += 1

            # Walk-forward validation: greedy RAW reward on the holdout windows
            policy_net.eval()
            val_reward = 0.0
            for v_mat, v_prices in val_episodes:
                try:
                    val_reward += float(_run_episode_greedy(v_mat, v_prices, policy_net))
                except Exception as exc:
                    log.warning("rl_dqn.train.val_failed", error=str(exc))
            policy_net.train()

            if val_reward > best_val_reward:
                best_val_reward = val_reward
                best_state_dict = copy.deepcopy(policy_net.state_dict())
                best_epoch = epoch + 1

            # Compute current epsilon for logging (post-epoch)
            current_eps = max(
                EPSILON_END,
                EPSILON_START - (EPSILON_START - EPSILON_END) * global_step / max(explore_steps, 1),
            )
            avg_loss = float(np.mean(epoch_losses)) if epoch_losses else float("nan")

            log.info(
                "rl_dqn.train.epoch",
                market=self._market_key,
                epoch=epoch + 1,
                epsilon=round(current_eps, 4),
                buffer_size=len(buffer),
                total_reward=round(total_reward, 4),
                val_reward=round(val_reward, 6),
                best_epoch=best_epoch,
                avg_loss=round(avg_loss, 6) if not (avg_loss != avg_loss) else None,
                episodes=len(epoch_episodes),
                global_steps=global_step,
            )

        # Restore the best epoch's weights (early stopping on holdout reward)
        if best_state_dict is not None:
            policy_net.load_state_dict(best_state_dict)
        policy_net.eval()

        # ------------------------------------------------------------------
        # Greedy evaluation pass — all real windows (train + holdout), RAW
        # reward, epsilon=0.  Measures true policy quality.
        # ------------------------------------------------------------------
        greedy_reward = 0.0
        real_episodes = real_train + val_episodes
        for obs_mat, prices_arr in real_episodes:
            greedy_reward += float(_run_episode_greedy(obs_mat, prices_arr, policy_net))

        log.info(
            "rl_dqn.train.eval",
            market=self._market_key,
            greedy_total_reward=round(greedy_reward, 6),
            best_epoch=best_epoch,
            best_val_reward=round(best_val_reward, 6) if best_val_reward != float("-inf") else None,
            real_episodes=len(real_episodes),
            val_episodes=len(val_episodes),
        )

        # ------------------------------------------------------------------
        # Save and cache
        # ------------------------------------------------------------------
        self._save_checkpoint(policy_net, in_dim)
        self._qnet = policy_net
        self._in_dim = in_dim
        self._trained = True

        log.info(
            "rl_dqn.train.done",
            market=self._market_key,
            series=len(series),
            train_episodes=len(all_train),
            mirror_episodes=len(mirror_train),
            val_episodes=len(val_episodes),
            global_steps=global_step,
            grad_steps=grad_step,
        )

    @staticmethod
    def _update_step(policy_net, target_net, buffer: _ReplayBuffer, optimizer):
        """One prioritized mini-batch update (Huber loss, IS weights, Polyak).

        The 5th transition field is the bootstrap discount (γ^n for n-step
        bootstrapped transitions, 0.0 for terminal), so the target is simply
        r + disc·Q_target(s', argmax_a Q_policy(s', a)).
        """
        import torch
        import torch.nn.functional as F

        batch, indices, is_weights = buffer.sample_per(BATCH_SIZE)
        states, actions, rewards, next_states, discs = zip(*batch)

        states_t = torch.tensor(np.array(states), dtype=torch.float32)
        actions_t = torch.tensor(actions, dtype=torch.long).unsqueeze(1)
        rewards_t = torch.tensor(rewards, dtype=torch.float32).unsqueeze(1)
        next_states_t = torch.tensor(np.array(next_states), dtype=torch.float32)
        discs_t = torch.tensor([float(d) for d in discs], dtype=torch.float32).unsqueeze(1)
        weights_t = torch.tensor(is_weights, dtype=torch.float32).unsqueeze(1)

        # Current Q values
        q_current = policy_net(states_t).gather(1, actions_t)

        # Target Q values (Double-DQN style: policy selects, target evaluates)
        with torch.no_grad():
            next_actions = policy_net(next_states_t).argmax(dim=1, keepdim=True)
            q_next = target_net(next_states_t).gather(1, next_actions)
            q_target = rewards_t + discs_t * q_next

        td_errors = (q_target - q_current).detach()
        loss = (weights_t * F.smooth_l1_loss(q_current, q_target, reduction="none")).mean()
        optimizer.zero_grad()
        loss.backward()
        torch.nn.utils.clip_grad_norm_(policy_net.parameters(), 1.0)
        optimizer.step()

        buffer.update_priorities(indices, td_errors.squeeze(1).numpy())

        # Polyak soft target update
        with torch.no_grad():
            for tp, pp in zip(target_net.parameters(), policy_net.parameters()):
                tp.data.mul_(1.0 - TAU).add_(TAU * pp.data)

        return float(loss.item())

    # ------------------------------------------------------------------
    # EMA fallback
    # ------------------------------------------------------------------

    def _ema_fallback(self, prices: list[float]) -> PredictionResult:
        """Simple EMA fallback when DQN is unavailable or fails."""
        arr = np.array(prices, dtype=float)
        current = float(arr[-1])
        period = min(26, len(arr))
        k = 2.0 / (period + 1)
        ema = float(np.mean(arr[:period]))
        for p in arr[period:]:
            ema = float(p) * k + ema * (1.0 - k)
        trend = (ema - current) / current
        predicted = current * (1.0 + trend * 0.5)
        max_change = current * get_max_change_pct(self._market_key)
        predicted = max(current - max_change, min(current + max_change, predicted))
        return PredictionResult(
            predicted_price=predicted,
            confidence=0.35,
            current_price=current,
            algorithm_name=self.get_key(),
        )
