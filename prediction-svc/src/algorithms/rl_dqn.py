"""RL DQN prediction algorithm — Deep Q-Network written in PyTorch.

Acts as both a prediction algorithm (algorithm #12) and a trading bot policy.

Architecture:
  MLP: in_dim → 128 → 64 → 3 (hold / buy / sell)

Training:
  DQN with replay buffer, target network (periodic hard update), ε-greedy decay
  per environment-step (not per episode), Huber loss, Adam optimizer, Double-DQN.

  Improvements over v1:
  - Mirror/inverted-return augmentation  — every real series is paired with a
    "mirror" series whose log-returns are negated.  This gives the agent equal
    exposure to downtrends, preventing the "always long" degenerate policy that
    forms when data is mostly uptrending (NASDAQ / SP500).
  - Sliding-window episodes — each obs-matrix is sliced into overlapping windows
    (EPISODE_WINDOW steps, stride EPISODE_STRIDE) so GOLD/CRYPTO with few symbols
    still generate many training episodes.  Total windows are capped at
    MAX_EPISODES_PER_MARKET to bound training time.
  - Epsilon decays per environment-step (global_step) from EPSILON_START to
    EPSILON_END over TARGET_EXPLORE_STEPS, using linear annealing.  This ensures
    exploration finishes well before training ends even when there are few series.
  - Greedy evaluation pass after training — runs epsilon=0 on real (non-mirror)
    series and logs greedy_total_reward, giving a reliable quality signal.
  - avg_loss logged per epoch alongside epsilon and buffer size.
  - Small per-step long-holding penalty (-HOLD_LONG_PENALTY) discourages the
    agent from blindly holding forever even when the market is flat.

Persistence:
  Checkpoint saved to ${RL_MODEL_DIR}/rl_dqn_{market_key}.pt after train_batch().
  Loaded lazily on first act() / predict() call.

Fallback:
  If PyTorch is unavailable, checkpoint is missing, or any inference error occurs,
  the method falls back to a simple EMA — identical pattern to lstm.py / gru.py.
"""
from __future__ import annotations

import os
import random
from collections import deque
from typing import Deque

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult, get_max_change_pct
from src.algorithms.features import MIN_DATA_POINTS, build_enhanced_features
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

# DQN training
REPLAY_CAPACITY = 10_000
BATCH_SIZE = 64
GAMMA = 0.99
LR_DQN = 1e-3
EPSILON_START = 1.0
EPSILON_END = 0.05
# Legacy fixed exploration horizon (fallback only). The actual epsilon-decay
# horizon is computed PER MARKET in train_batch() as ~60% of that market's real
# env-step budget, so low-data markets (e.g. GOLD) still anneal ε to the floor
# instead of staying near-random. Kept for backward compatibility / reference.
TARGET_EXPLORE_STEPS = 15_000
TARGET_UPDATE_EVERY = 50    # hard-copy target net every N gradient steps
TRAIN_EPOCHS = 12           # passes over the full (windowed + augmented) episode set
MIN_REPLAY = 200            # start gradient updates after this many transitions

# Sliding-window episode parameters
EPISODE_WINDOW = 130        # steps per sliding-window episode
EPISODE_STRIDE = 35         # stride between consecutive windows
MAX_EPISODES_PER_MARKET = 400   # cap total episodes (real + mirror) to bound time

# Per-step penalty for holding a long position — prevents "always long" policy.
# Value is very small (1/10 of a typical transaction cost) to avoid overriding
# the primary dense reward signal.
HOLD_LONG_PENALTY = 0.0001

# Prediction magnitude
K_SIGMA = 1.5             # predicted change = K_SIGMA * rolling_σ of log-returns
SIGMA_WINDOW = 20         # window for rolling σ

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


def _checkpoint_path(market_key: str) -> str:
    mdir = _get_model_dir()
    # Normalise market key: NASDAQ100 → NASDAQ100, etc.
    safe_key = (market_key or "UNKNOWN").upper().replace("/", "_")
    return os.path.join(mdir, f"rl_dqn_{safe_key}.pt")


# ---------------------------------------------------------------------------
# Q-Network (MLP)
# ---------------------------------------------------------------------------

def _build_qnet(in_dim: int):
    """Build a fresh MLP Q-network: in_dim → HIDDEN1 → HIDDEN2 → N_ACTIONS."""
    try:
        import torch.nn as nn
    except ImportError as exc:
        raise RuntimeError("PyTorch not available") from exc

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


# ---------------------------------------------------------------------------
# Replay Buffer
# ---------------------------------------------------------------------------

class _ReplayBuffer:
    """Simple circular replay buffer."""

    def __init__(self, capacity: int) -> None:
        self._buf: Deque = deque(maxlen=capacity)

    def push(self, state, action: int, reward: float, next_state, done: bool) -> None:
        self._buf.append((state, action, reward, next_state, done))

    def sample(self, n: int):
        return random.sample(self._buf, n)

    def __len__(self) -> int:
        return len(self._buf)


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


def _build_obs_matrix(prices: np.ndarray, volumes: np.ndarray | None) -> np.ndarray:
    """Build enhanced-feature matrix for a price series.

    Returns array of shape (T, n_features) where T = len(features).
    Uses build_enhanced_features() which returns (features_list, targets_list).
    """
    feats, _ = build_enhanced_features(prices, volumes)
    return np.array(feats, dtype=np.float32)


def _make_windows(
    obs_matrix: np.ndarray,
    prices: np.ndarray,
    window: int,
    stride: int,
) -> list[tuple[np.ndarray, np.ndarray]]:
    """Slice (obs_matrix, prices) into overlapping windows.

    Each window is (obs_window, prices_window) of length `window`.
    If the series is shorter than `window`, return it as a single episode.
    """
    n = len(obs_matrix)
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
    """Simulate one episode over obs_matrix, collecting transitions into buffer.

    Epsilon is computed PER ENVIRONMENT STEP using linear annealing:
        eps(t) = max(END, START - (START - END) * t / target_explore_steps)
    `global_step` is the cumulative env-step count across ALL episodes so far;
    it is updated locally and returned so the caller can persist it.

    Position model: flat or long-only (all-in / all-out).
    Returns (total_episode_reward, updated_global_step).
    """
    import torch

    n_steps, n_base = obs_matrix.shape

    position: float = 0.0       # 1.0 = long, 0.0 = flat
    entry_price: float = 0.0
    days_held: int = 0
    total_reward = 0.0

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

        # Dense step reward
        price_return = prices[t + 1] / prices[t] - 1.0
        reward = (
            position * price_return
            - transaction_cost * abs(position - prev_position)
            - HOLD_LONG_PENALTY * position   # small penalty for holding long
        )
        total_reward += reward

        # Next state
        days_held += 1 if position > 0 else 0
        next_unrealized = (prices[t + 1] / entry_price - 1.0) if (position > 0 and entry_price > 0) else 0.0
        next_days_norm = min(days_held / 30.0, 1.0)
        next_pos_feats = np.array([position, next_unrealized, next_days_norm], dtype=np.float32)
        next_state = np.concatenate([obs_matrix[t + 1], next_pos_feats])

        done = (t == n_steps - 2)
        buffer.push(state, action, reward, next_state, done)

        global_step += 1

    return total_reward, global_step


def _run_episode_greedy(
    obs_matrix: np.ndarray,
    prices: np.ndarray,
    policy_net,
    transaction_cost: float = 0.0005,
) -> float:
    """Greedy (epsilon=0) evaluation episode — no buffer writes.

    Returns total episode reward under the current policy.
    Used after training to measure true policy quality.
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
    - predict()      : greedy policy (ε=0) → action → price via σ-scaling.
    - act(obs)       : shared greedy policy used by both predict() and bot.step().
    - train_batch()  : full DQN training loop; saves checkpoint to disk.
    - is_trained()   : True when a Q-network is loaded / trained.
    - Fallback       : EMA on any failure (same pattern as lstm.py / gru.py).
    """

    def __init__(self) -> None:
        self._qnet = None           # loaded Q-network (eval mode)
        self._in_dim: int = 0       # expected observation dimension
        self._trained: bool = False

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
        """Try to load checkpoint for self._market_key. Returns True on success."""
        path = _checkpoint_path(self._market_key)
        if not os.path.exists(path):
            return False
        try:
            import torch
            data = torch.load(path, map_location="cpu", weights_only=False)
            in_dim = data["in_dim"]
            net = _build_qnet(in_dim)
            net.load_state_dict(data["state_dict"])
            net.eval()
            self._qnet = net
            self._in_dim = in_dim
            self._trained = True
            log.info("rl_dqn.checkpoint.loaded", market=self._market_key, path=path)
            return True
        except Exception as exc:
            log.warning("rl_dqn.checkpoint.load_failed", market=self._market_key, path=path, error=str(exc))
            return False

    def _save_checkpoint(self, net, in_dim: int) -> None:
        path = _checkpoint_path(self._market_key)
        try:
            import torch
            os.makedirs(os.path.dirname(path), exist_ok=True)
            torch.save({"in_dim": in_dim, "state_dict": net.state_dict()}, path)
            log.info("rl_dqn.checkpoint.saved", market=self._market_key, path=path)
        except Exception as exc:
            log.warning("rl_dqn.checkpoint.save_failed", market=self._market_key, path=path, error=str(exc))

    # ------------------------------------------------------------------
    # act() — shared greedy policy (ε=0)
    # ------------------------------------------------------------------

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
            obs = np.array(observation, dtype=np.float32)
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
            log.warning("rl_dqn.predict.fallback", error=str(exc))
            return self._ema_fallback(prices)

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

        if action == 1:    # BUY → predict up
            predicted = current * (1.0 + K_SIGMA * sigma)
        elif action == 2:  # SELL → predict down
            predicted = current * (1.0 - K_SIGMA * sigma)
        else:              # HOLD → nearly flat (slight noise)
            predicted = current * (1.0 + 0.1 * sigma)

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

    def _softmax_confidence(self, obs: np.ndarray, action: int) -> float:
        """Return the softmax probability of the selected action."""
        if self._qnet is None:
            return 0.40
        try:
            import torch
            import torch.nn.functional as F
            with torch.no_grad():
                q = self._qnet(torch.tensor(obs, dtype=torch.float32).unsqueeze(0))
                probs = F.softmax(q, dim=1)
                return float(probs[0, action].item())
        except Exception:
            return 0.40

    # ------------------------------------------------------------------
    # train_batch()
    # ------------------------------------------------------------------

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Train DQN on all price series for the market.

        Strategy (v2):
        1. Build obs-matrix for each real series.
        2. Build mirror obs-matrix (negated log-returns) for each series.
        3. Slice every (real + mirror) matrix into overlapping windows
           (EPISODE_WINDOW, stride EPISODE_STRIDE).
        4. Cap total episode count at MAX_EPISODES_PER_MARKET.
        5. Run TRAIN_EPOCHS passes; epsilon decays by global_step (linear).
        6. Collect mini-batch gradient updates (Double DQN, Huber loss).
        7. After training, run a greedy evaluation pass on real series only
           and log greedy_total_reward.
        8. Save checkpoint and cache network.
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
        # Build observation matrices for all series (real + mirror)
        # ------------------------------------------------------------------
        real_episodes: list[tuple[np.ndarray, np.ndarray]] = []   # (obs_mat, prices)
        mirror_episodes: list[tuple[np.ndarray, np.ndarray]] = [] # (obs_mat, prices)

        for prices_list, vols_list in series:
            if len(prices_list) < MIN_DATA_POINTS:
                continue
            arr = np.array(prices_list, dtype=np.float32)
            vol_arr = np.array(vols_list, dtype=np.float32) if vols_list else None

            # Real series
            try:
                mat = _build_obs_matrix(arr, vol_arr)
                if len(mat) > 0:
                    for win_mat, win_prices in _make_windows(mat, arr, EPISODE_WINDOW, EPISODE_STRIDE):
                        real_episodes.append((win_mat, win_prices))
            except Exception as exc:
                log.warning("rl_dqn.train.obs_failed", series="real", error=str(exc))
                continue

            # Mirror series — build mirror prices, then features on top of them
            try:
                mirror_arr = _build_mirror_prices(arr)
                # volumes stay the same (mirror only flips price direction)
                mirror_mat = _build_obs_matrix(mirror_arr, vol_arr)
                if len(mirror_mat) > 0:
                    for win_mat, win_prices in _make_windows(mirror_mat, mirror_arr, EPISODE_WINDOW, EPISODE_STRIDE):
                        mirror_episodes.append((win_mat, win_prices))
            except Exception as exc:
                log.warning("rl_dqn.train.obs_failed", series="mirror", error=str(exc))

        all_episodes = real_episodes + mirror_episodes

        if not all_episodes:
            log.warning("rl_dqn.train.skip", reason="no valid series after feature build")
            return

        # Cap total episodes to bound training time
        if len(all_episodes) > MAX_EPISODES_PER_MARKET:
            # Shuffle first so the cap samples from the full distribution
            random.shuffle(all_episodes)
            all_episodes = all_episodes[:MAX_EPISODES_PER_MARKET]
            log.info(
                "rl_dqn.train.episodes_capped",
                market=self._market_key,
                capped_to=MAX_EPISODES_PER_MARKET,
            )

        # in_dim: enhanced feature count + position features
        in_dim = all_episodes[0][0].shape[1] + N_POS_FEATURES

        # Adaptive exploration horizon: anneal epsilon over ~60% of the ACTUAL
        # env-steps this market will collect, so low-data markets (e.g. GOLD with
        # few series) still reach the exploitation phase (ε→floor) instead of
        # staying near-random. A fixed-constant horizon under-anneals small markets.
        steps_per_epoch = sum(max(0, m.shape[0] - 1) for m, _ in all_episodes)
        explore_steps = max(1, int(0.6 * steps_per_epoch * TRAIN_EPOCHS))

        # ------------------------------------------------------------------
        # Build networks
        # ------------------------------------------------------------------
        policy_net = _build_qnet(in_dim)
        target_net = _build_qnet(in_dim)
        target_net.load_state_dict(policy_net.state_dict())
        target_net.eval()

        optimizer = torch.optim.Adam(policy_net.parameters(), lr=LR_DQN)
        buffer = _ReplayBuffer(REPLAY_CAPACITY)

        global_step = 0      # cumulative env-steps across all episodes + epochs
        grad_step = 0        # cumulative gradient steps (for target-net sync)
        total_reward = 0.0

        # ------------------------------------------------------------------
        # Collect experience: TRAIN_EPOCHS passes over all windowed episodes
        # ------------------------------------------------------------------
        for epoch in range(TRAIN_EPOCHS):
            epoch_losses: list[float] = []
            # Shuffle episode order each epoch for diversity
            epoch_episodes = all_episodes.copy()
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
                        if grad_step % TARGET_UPDATE_EVERY == 0:
                            target_net.load_state_dict(policy_net.state_dict())

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
                avg_loss=round(avg_loss, 6) if not (avg_loss != avg_loss) else None,
                episodes=len(epoch_episodes),
                global_steps=global_step,
            )

        # Final target-net sync
        target_net.load_state_dict(policy_net.state_dict())
        policy_net.eval()

        # ------------------------------------------------------------------
        # Greedy evaluation pass — real series only, epsilon=0
        # Measures true policy quality (independent of exploration noise).
        # ------------------------------------------------------------------
        greedy_reward = 0.0
        for obs_mat, prices_arr in real_episodes:
            greedy_reward += _run_episode_greedy(obs_mat, prices_arr, policy_net)

        log.info(
            "rl_dqn.train.eval",
            market=self._market_key,
            greedy_total_reward=round(greedy_reward, 6),
            real_episodes=len(real_episodes),
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
            real_episodes=len(real_episodes),
            mirror_episodes=len(mirror_episodes),
            total_episodes_used=len(all_episodes),
            global_steps=global_step,
            grad_steps=grad_step,
        )

    @staticmethod
    def _update_step(policy_net, target_net, buffer: _ReplayBuffer, optimizer):
        """One mini-batch gradient update (Huber / smooth-L1 loss)."""
        import torch
        import torch.nn.functional as F

        batch = buffer.sample(BATCH_SIZE)
        states, actions, rewards, next_states, dones = zip(*batch)

        states_t = torch.tensor(np.array(states), dtype=torch.float32)
        actions_t = torch.tensor(actions, dtype=torch.long).unsqueeze(1)
        rewards_t = torch.tensor(rewards, dtype=torch.float32).unsqueeze(1)
        next_states_t = torch.tensor(np.array(next_states), dtype=torch.float32)
        dones_t = torch.tensor(dones, dtype=torch.float32).unsqueeze(1)

        # Current Q values
        q_current = policy_net(states_t).gather(1, actions_t)

        # Target Q values (Double-DQN style: policy selects, target evaluates)
        with torch.no_grad():
            next_actions = policy_net(next_states_t).argmax(dim=1, keepdim=True)
            q_next = target_net(next_states_t).gather(1, next_actions)
            q_target = rewards_t + GAMMA * q_next * (1 - dones_t)

        loss = F.smooth_l1_loss(q_current, q_target)
        optimizer.zero_grad()
        loss.backward()
        torch.nn.utils.clip_grad_norm_(policy_net.parameters(), 1.0)
        optimizer.step()
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
