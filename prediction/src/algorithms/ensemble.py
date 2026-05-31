"""Ensemble prediction algorithm — weighted average of all base algorithms."""
from __future__ import annotations

from typing import Optional

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("ensemble")


class EnsemblePredictor(PredictionAlgorithm):
    """Equal-weighted ensemble of all registered base algorithms."""

    def __init__(self, base_algorithms: list[PredictionAlgorithm]) -> None:
        if not base_algorithms:
            raise ValueError("Ensemble requires at least one base algorithm")
        self._bases = base_algorithms

    def get_name(self) -> str:
        return "Ensemble"

    def get_key(self) -> str:
        return "ensemble"

    def predict(self, prices: list[float], volumes: Optional[list[float]] = None) -> PredictionResult:
        if not prices:
            raise ValueError("Empty price list")

        current = float(prices[-1])
        successful: list[PredictionResult] = []

        for algo in self._bases:
            try:
                result = algo.predict(prices, volumes)
                successful.append(result)
            except Exception as exc:
                log.debug("ensemble.base_failed", algo=algo.get_key(), error=str(exc))

        if not successful:
            raise ValueError("All base algorithms failed in ensemble")

        # Equal-weight average
        avg_price = sum(r.predicted_price for r in successful) / len(successful)
        avg_conf = sum(r.confidence for r in successful) / len(successful)

        return PredictionResult(
            predicted_price=avg_price,
            confidence=avg_conf,
            current_price=current,
            algorithm_name=self.get_key(),
        )
