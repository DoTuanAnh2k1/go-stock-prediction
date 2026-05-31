"""LSTM Neural Network prediction algorithm using PyTorch.

A real 2-layer LSTM trained per inference (lightweight: 50 epochs).
Handles graceful fallback to EMA if data is insufficient or training fails.
"""
from __future__ import annotations

import math
from typing import Optional

import numpy as np

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("lstm")

SEQUENCE_LENGTH = 60
HIDDEN_SIZE = 64
NUM_LAYERS = 2
DROPOUT = 0.2
EPOCHS = 50
LR = 0.001
MIN_DATA_POINTS = SEQUENCE_LENGTH + 10


class LSTMPredictor(PredictionAlgorithm):
    """PyTorch LSTM-based stock price predictor."""

    def get_name(self) -> str:
        return "LSTM Neural Network"

    def get_key(self) -> str:
        return "lstm_nn"

    def predict(self, prices: list[float], volumes: Optional[list[float]] = None) -> PredictionResult:
        if len(prices) < MIN_DATA_POINTS:
            raise ValueError(f"LSTM needs at least {MIN_DATA_POINTS} points, got {len(prices)}")

        try:
            return self._train_and_predict(prices)
        except Exception as exc:
            log.warning("lstm.fallback", error=str(exc))
            # Fallback: simple exponential moving average
            return self._ema_fallback(prices)

    def _train_and_predict(self, prices: list[float]) -> PredictionResult:
        try:
            import torch
            import torch.nn as nn
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

        # Model
        class _LSTM(nn.Module):
            def __init__(self):
                super().__init__()
                self.lstm = nn.LSTM(
                    input_size=1,
                    hidden_size=HIDDEN_SIZE,
                    num_layers=NUM_LAYERS,
                    batch_first=True,
                    dropout=DROPOUT if NUM_LAYERS > 1 else 0,
                )
                self.fc = nn.Linear(HIDDEN_SIZE, 1)

            def forward(self, x):
                out, _ = self.lstm(x)
                return self.fc(out[:, -1, :])

        model = _LSTM()
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
        """Simple EMA fallback when LSTM fails."""
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
            algorithm_name="lstm_nn",
        )
