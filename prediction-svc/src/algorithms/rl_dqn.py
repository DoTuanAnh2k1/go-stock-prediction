"""RL DQN prediction algorithm — Deep Q-Network written in PyTorch.

Acts as both a prediction algorithm (algorithm #12) and a trading bot policy.

Architecture:
  MLP: in_dim → 128 → 64 → 3 (hold / buy / sell)

Training:
  DQN with replay buffer, target network (periodic hard update), ε-greedy decay,
  Huber loss, Adam optimizer.  One market = one episode-sequence built from the
  concatenated price history of all instruments in that market.

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
EPSILON_DECAY = 0.995
TARGET_UPDATE_EVERY = 50  # hard update every N steps
TRAIN_EPOCHS = 3          # passes over the collected experience
MIN_REPLAY = 200          # start training after this many transitions

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
# Environment helper
# ---------------------------------------------------------------------------

def _build_obs_matrix(prices: np.ndarray, volumes: np.ndarray | None) -> np.ndarray:
    """Build enhanced-feature matrix for a price series.

    Returns array of shape (T, n_features) where T = len(features).
    Uses build_enhanced_features() which returns (features_list, targets_list).
    The last row is the inference row; we keep ALL rows.
    """
    feats, _ = build_enhanced_features(prices, volumes)
    return np.array(feats, dtype=np.float32)


def _run_episode(
    obs_matrix: np.ndarray,
    prices: np.ndarray,
    policy_net,
    buffer: _ReplayBuffer,
    epsilon: float,
    transaction_cost: float = 0.0005,
) -> float:
    """Simulate one episode over obs_matrix, collecting transitions into buffer.

    Position model: flat or long-only (all-in / all-out).
    Returns total episode reward.
    """
    import torch

    n_steps, n_base = obs_matrix.shape
    in_dim = n_base + N_POS_FEATURES

    position: float = 0.0       # 1.0 = long, 0.0 = flat
    entry_price: float = 0.0
    days_held: int = 0
    total_reward = 0.0

    for t in range(n_steps - 1):
        # Build state
        unrealized_pnl = (prices[t] / entry_price - 1.0) if (position > 0 and entry_price > 0) else 0.0
        days_held_norm = min(days_held / 30.0, 1.0)
        pos_feats = np.array([position, unrealized_pnl, days_held_norm], dtype=np.float32)
        state = np.concatenate([obs_matrix[t], pos_feats])

        # ε-greedy action
        if random.random() < epsilon:
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
        reward = position * price_return - transaction_cost * abs(position - prev_position)
        total_reward += reward

        # Next state
        days_held += 1 if position > 0 else 0
        next_unrealized = (prices[t + 1] / entry_price - 1.0) if (position > 0 and entry_price > 0) else 0.0
        next_days_norm = min(days_held / 30.0, 1.0)
        next_pos_feats = np.array([position, next_unrealized, next_days_norm], dtype=np.float32)
        next_state = np.concatenate([obs_matrix[t + 1], next_pos_feats])

        done = (t == n_steps - 2)
        buffer.push(state, action, reward, next_state, done)

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

        Runs episode collection + mini-batch gradient updates in multiple passes.
        After training, saves checkpoint and updates self._qnet.
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
        # Build observation matrices for all series
        # ------------------------------------------------------------------
        obs_mats: list[tuple[np.ndarray, np.ndarray]] = []
        for prices_list, vols_list in series:
            if len(prices_list) < MIN_DATA_POINTS:
                continue
            arr = np.array(prices_list, dtype=np.float32)
            vol_arr = np.array(vols_list, dtype=np.float32) if vols_list else None
            try:
                mat = _build_obs_matrix(arr, vol_arr)
                if len(mat) > 0:
                    obs_mats.append((mat, arr))
            except Exception as exc:
                log.warning("rl_dqn.train.obs_failed", error=str(exc))

        if not obs_mats:
            log.warning("rl_dqn.train.skip", reason="no valid series after feature build")
            return

        # in_dim: enhanced feature count + position features
        in_dim = obs_mats[0][0].shape[1] + N_POS_FEATURES

        # ------------------------------------------------------------------
        # Build networks
        # ------------------------------------------------------------------
        policy_net = _build_qnet(in_dim)
        target_net = _build_qnet(in_dim)
        target_net.load_state_dict(policy_net.state_dict())
        target_net.eval()

        optimizer = torch.optim.Adam(policy_net.parameters(), lr=LR_DQN)
        buffer = _ReplayBuffer(REPLAY_CAPACITY)

        epsilon = EPSILON_START
        step_count = 0
        total_reward = 0.0

        # ------------------------------------------------------------------
        # Collect experience: run TRAIN_EPOCHS passes over all series
        # ------------------------------------------------------------------
        for epoch in range(TRAIN_EPOCHS):
            for obs_mat, prices_arr in obs_mats:
                ep_reward = _run_episode(
                    obs_matrix=obs_mat,
                    prices=prices_arr,
                    policy_net=policy_net,
                    buffer=buffer,
                    epsilon=epsilon,
                )
                total_reward += ep_reward
                epsilon = max(EPSILON_END, epsilon * EPSILON_DECAY)
                step_count += len(obs_mat)

                # Mini-batch updates whenever buffer is large enough
                if len(buffer) >= MIN_REPLAY:
                    for _ in range(min(10, len(buffer) // BATCH_SIZE)):
                        loss = self._update_step(
                            policy_net, target_net, buffer, optimizer
                        )
                        step_count += 1
                        if step_count % TARGET_UPDATE_EVERY == 0:
                            target_net.load_state_dict(policy_net.state_dict())

            log.info(
                "rl_dqn.train.epoch",
                market=self._market_key,
                epoch=epoch + 1,
                epsilon=round(epsilon, 4),
                buffer_size=len(buffer),
                total_reward=round(total_reward, 4),
            )

        # Final target-net sync
        target_net.load_state_dict(policy_net.state_dict())
        policy_net.eval()

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
            series=len(obs_mats),
            steps=step_count,
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
