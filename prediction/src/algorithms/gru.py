"""GRU Neural Network prediction algorithm using PyTorch.

A real 2-layer GRU. Supports per-market model caching via train().
When a trained model is cached, predict() runs inference-only (no retraining).
Falls back to fresh train-and-predict or EMA when needed.
"""
from __future__ import annotations

import math

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("gru")

SEQUENCE_LENGTH = 60
HIDDEN_SIZE = 64
NUM_LAYERS = 2
DROPOUT = 0.2
EPOCHS = 50
LR = 0.001
MIN_DATA_POINTS = SEQUENCE_LENGTH + 10


class GRUPredictor(PredictionAlgorithm):
    """PyTorch GRU-based stock price predictor."""

    def __init__(self) -> None:
        self._model = None        # cached PyTorch model
        self._seq_len: int = SEQUENCE_LENGTH
        self._p_min: float = 0.0
        self._p_max: float = 1.0

    def get_name(self) -> str:
        return "GRU Neural Network"

    def get_key(self) -> str:
        return "gru_nn"

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
            import torch
            import torch.nn.functional as F

            arr = np.array(prices, dtype=np.float32)
            p_min, p_max = arr.min(), arr.max()
            if p_max - p_min < 1e-8:
                p_max = p_min + 1.0
            arr_norm = (arr - p_min) / (p_max - p_min)

            seq_len = min(SEQUENCE_LENGTH, len(arr) - 1)
            X, y = [], []
            for i in range(len(arr_norm) - seq_len):
                X.append(arr_norm[i : i + seq_len])
                y.append(arr_norm[i + seq_len])

            X_t = torch.tensor(np.array(X), dtype=torch.float32).unsqueeze(-1)
            y_t = torch.tensor(np.array(y), dtype=torch.float32).unsqueeze(-1)

            model = self._build_model()
            optimizer = torch.optim.Adam(model.parameters(), lr=LR)
            model.train()
            for _ in range(EPOCHS):
                pred = model(X_t)
                loss = F.mse_loss(pred, y_t)
                optimizer.zero_grad()
                loss.backward()
                optimizer.step()

            model.eval()
            self._model = model
            self._seq_len = seq_len
            self._p_min = float(p_min)
            self._p_max = float(p_max)
            log.info("gru.trained", data_points=len(prices))
        except Exception as exc:
            log.warning("gru.train_failed", error=str(exc))

    def train_batch(self, series: list[tuple[list[float], "list[float] | None"]]) -> None:
        """Train a single GRU model on all price series combined."""
        try:
            import torch
            import torch.nn.functional as F

            all_X: list = []
            all_y: list = []

            for prices, _volumes in series:
                if len(prices) < MIN_DATA_POINTS:
                    continue
                arr = np.array(prices, dtype=np.float32)
                p_min, p_max = arr.min(), arr.max()
                if p_max - p_min < 1e-8:
                    p_max = p_min + 1.0
                arr_norm = (arr - p_min) / (p_max - p_min)
                seq_len = min(SEQUENCE_LENGTH, len(arr) - 1)
                for i in range(len(arr_norm) - seq_len):
                    all_X.append(arr_norm[i : i + seq_len])
                    all_y.append(arr_norm[i + seq_len])

            if len(all_X) < 10:
                log.warning("gru.batch_train_skip", reason="insufficient sequences", sequences=len(all_X))
                return

            X_t = torch.tensor(np.array(all_X), dtype=torch.float32).unsqueeze(-1)
            y_t = torch.tensor(np.array(all_y), dtype=torch.float32).unsqueeze(-1)

            model = self._build_model()
            optimizer = torch.optim.Adam(model.parameters(), lr=LR)
            model.train()
            for _ in range(EPOCHS):
                pred = model(X_t)
                loss = F.mse_loss(pred, y_t)
                optimizer.zero_grad()
                loss.backward()
                optimizer.step()

            model.eval()
            self._model = model
            self._seq_len = SEQUENCE_LENGTH
            log.info("gru.batch_trained", series=len(series), sequences=len(all_X))
        except Exception as exc:
            log.warning("gru.batch_train_failed", error=str(exc))

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"GRU needs at least {MIN_DATA_POINTS} points, got {len(prices)}")

        if self._model is not None:
            try:
                return self._inference(prices)
            except Exception as exc:
                log.warning("gru.inference_failed", error=str(exc))
                # fall through to fresh train-and-predict

        try:
            return self._train_and_predict(prices)
        except Exception as exc:
            log.warning("gru.fallback", error=str(exc))
            return self._ema_fallback(prices)

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    @staticmethod
    def _build_model():
        """Build a fresh _GRU model instance."""
        try:
            import torch.nn as nn
        except ImportError as e:
            raise RuntimeError("PyTorch not available") from e

        class _GRU(nn.Module):
            def __init__(self):
                super().__init__()
                self.gru = nn.GRU(
                    input_size=1,
                    hidden_size=HIDDEN_SIZE,
                    num_layers=NUM_LAYERS,
                    batch_first=True,
                    dropout=DROPOUT if NUM_LAYERS > 1 else 0,
                )
                self.fc = nn.Linear(HIDDEN_SIZE, 1)

            def forward(self, x):
                output, h_n = self.gru(x)
                return self.fc(output[:, -1, :])

        return _GRU()

    def _inference(self, prices: list[float]) -> PredictionResult:
        """Run inference using cached model. Normalises with current price range."""
        import torch

        arr = np.array(prices, dtype=np.float32)
        current = float(arr[-1])
        p_min, p_max = arr.min(), arr.max()
        if p_max - p_min < 1e-8:
            p_max = p_min + 1.0
        arr_norm = (arr - p_min) / (p_max - p_min)

        seq_len = min(self._seq_len, len(arr) - 1)
        last_seq = (
            torch.tensor(arr_norm[-seq_len:], dtype=torch.float32)
            .unsqueeze(0)
            .unsqueeze(-1)
        )
        self._model.eval()
        with torch.no_grad():
            pred_norm = self._model(last_seq).item()

        predicted_price = float(pred_norm * (p_max - p_min) + p_min)
        max_change = current * 0.07
        predicted_price = max(current - max_change, min(current + max_change, predicted_price))

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=0.70,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    def _train_and_predict(self, prices: list[float]) -> PredictionResult:
        try:
            import torch
            import torch.nn.functional as F
        except ImportError as e:
            raise RuntimeError("PyTorch not available") from e

        arr = np.array(prices, dtype=np.float32)
        current = float(arr[-1])

        # MinMax normalization
        p_min, p_max = arr.min(), arr.max()
        if p_max - p_min < 1e-8:
            p_max = p_min + 1.0
        arr_norm = (arr - p_min) / (p_max - p_min)

        # Build sequences
        seq_len = min(SEQUENCE_LENGTH, len(arr) - 1)
        X, y = [], []
        for i in range(len(arr_norm) - seq_len):
            X.append(arr_norm[i : i + seq_len])
            y.append(arr_norm[i + seq_len])

        X_t = torch.tensor(np.array(X), dtype=torch.float32).unsqueeze(-1)  # (N, seq, 1)
        y_t = torch.tensor(np.array(y), dtype=torch.float32).unsqueeze(-1)  # (N, 1)

        model = self._build_model()
        optimizer = torch.optim.Adam(model.parameters(), lr=LR)

        model.train()
        for _ in range(EPOCHS):
            pred = model(X_t)
            loss = F.mse_loss(pred, y_t)
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()

        model.eval()
        with torch.no_grad():
            last_seq = torch.tensor(
                arr_norm[-seq_len:], dtype=torch.float32
            ).unsqueeze(0).unsqueeze(-1)
            pred_norm = model(last_seq).item()

        predicted_price = float(pred_norm * (p_max - p_min) + p_min)

        # Clamp to ±7% daily limit
        max_change = current * 0.07
        predicted_price = max(current - max_change, min(current + max_change, predicted_price))

        # Confidence based on inverse of final loss
        try:
            with torch.no_grad():
                final_pred = model(X_t[-1:])
                final_loss = float(F.mse_loss(final_pred, y_t[-1:]).item())
            confidence = max(0.3, min(0.9, 1.0 / (1.0 + math.sqrt(final_loss) * 10)))
        except Exception:
            confidence = 0.55

        return PredictionResult(
            predicted_price=predicted_price,
            confidence=confidence,
            current_price=current,
            algorithm_name=self.get_key(),
        )

    @staticmethod
    def _ema_fallback(prices: list[float]) -> PredictionResult:
        """Simple EMA fallback when GRU fails."""
        arr = np.array(prices, dtype=float)
        current = float(arr[-1])
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
            confidence=0.40,
            current_price=current,
            algorithm_name="gru_nn",
        )
