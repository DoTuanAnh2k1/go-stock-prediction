"""Ensemble prediction algorithm — accuracy-weighted average of base algorithms.

By default this is an equal-weighted mean of every base algorithm. When
per-algorithm direction-accuracy is supplied via ``set_weights()`` (the
orchestrator does this each run from ``repo.get_direction_accuracy(market)``),
each base is weighted by how much better than a coin-flip it is — weight =
max(0, accuracy_fraction - 0.5). Bases at or below 50% direction accuracy get
zero weight, so a systematically-wrong model no longer drags the mean. If no
useful weights are available (cold start, nothing reconciled yet, or every base
≤ 50%), it falls back to an equal-weighted mean.
"""
from __future__ import annotations

from src.algorithms.base import PredictionAlgorithm, PredictionResult
from src.utils.logger import get_logger

log = get_logger("ensemble")


class EnsemblePredictor(PredictionAlgorithm):
    """Direction-accuracy-weighted ensemble of all registered base algorithms."""

    def __init__(self, base_algorithms: list[PredictionAlgorithm]) -> None:
        if not base_algorithms:
            raise ValueError("Ensemble requires at least one base algorithm")
        self._bases = base_algorithms
        # algo_key -> weight (>= 0). Empty => equal weight for everyone.
        self._weights: dict[str, float] = {}

    def get_name(self) -> str:
        return "Ensemble"

    def get_key(self) -> str:
        return "ensemble"

    def set_weights(self, accuracy_pct: dict[str, float] | None) -> None:
        """Set per-base weights from a {algo_key: direction_accuracy_percent} map.

        Values are percentages in [0, 100] (as returned by
        ``repo.get_direction_accuracy``). Weight = max(0, acc/100 - 0.5): a base
        must beat a coin flip to contribute. Pass None/empty to reset to equal
        weighting.
        """
        weights: dict[str, float] = {}
        for key, pct in (accuracy_pct or {}).items():
            if pct is None:
                continue
            try:
                weights[key] = max(0.0, float(pct) / 100.0 - 0.5)
            except (TypeError, ValueError):
                continue
        self._weights = weights

    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
        if not prices:
            raise ValueError("Empty price list")

        current = float(prices[-1])
        results: list[tuple[str, PredictionResult]] = []

        for algo in self._bases:
            try:
                result = algo.predict(prices, volumes)
                results.append((algo.get_key(), result))
            except Exception as exc:
                log.debug("ensemble.base_failed", algo=algo.get_key(), error=str(exc))

        if not results:
            raise ValueError("All base algorithms failed in ensemble")

        # Build weights aligned to the successful results.
        weights = [max(0.0, self._weights.get(key, 0.0)) for key, _ in results]
        total_w = sum(weights)

        if total_w <= 0.0:
            # Cold start / no base beats coin flip → equal weight.
            avg_price = sum(r.predicted_price for _, r in results) / len(results)
            avg_conf = sum(r.confidence for _, r in results) / len(results)
            weighted = False
        else:
            avg_price = sum(w * r.predicted_price for w, (_, r) in zip(weights, results)) / total_w
            avg_conf = sum(w * r.confidence for w, (_, r) in zip(weights, results)) / total_w
            weighted = True

        log.debug(
            "ensemble.predict",
            bases=len(results),
            weighted=weighted,
            active=sum(1 for w in weights if w > 0),
        )

        return PredictionResult(
            predicted_price=avg_price,
            confidence=avg_conf,
            current_price=current,
            algorithm_name=self.get_key(),
        )
