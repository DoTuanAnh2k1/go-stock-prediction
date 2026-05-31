"""Abstract base class for all prediction algorithms."""
from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional


@dataclass
class PredictionResult:
    """Result from a prediction algorithm."""
    predicted_price: float
    confidence: float  # 0.0 - 1.0
    current_price: float
    algorithm_name: str


class PredictionAlgorithm(ABC):
    """All prediction algorithms must implement this interface."""

    @abstractmethod
    def predict(self, prices: list[float], volumes: Optional[list[float]] = None) -> PredictionResult:
        """Generate a prediction.

        Args:
            prices: Closing prices in ASC order (oldest first, newest last).
            volumes: Trading volumes in ASC order (optional).

        Returns:
            PredictionResult with predicted_price and confidence [0,1].
        """

    @abstractmethod
    def get_name(self) -> str:
        """Return the human-readable algorithm name."""

    @abstractmethod
    def get_key(self) -> str:
        """Return the short snake_case algorithm key used in DB."""

    def get_accuracy(self) -> float:
        """Return the last computed backtest accuracy. Override if tracked."""
        return 0.0
