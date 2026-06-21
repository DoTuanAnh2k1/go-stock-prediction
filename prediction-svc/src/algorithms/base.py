"""Abstract base class for all prediction algorithms."""
from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass

# ---------------------------------------------------------------------------
# Market-aware clamp limits
# ---------------------------------------------------------------------------

MARKET_MAX_CHANGE: dict[str, float] = {
    "GOLD": 0.15,
    "NASDAQ100": 0.20,
    "SP500": 0.15,
    "CRYPTO": 0.50,
}
DEFAULT_MAX_CHANGE = 0.15


def get_max_change_pct(market_key: str) -> float:
    """Return the maximum allowed price-change fraction for a given market.

    Args:
        market_key: Market identifier string (e.g. "GOLD", "NASDAQ100", "CRYPTO").
                    Case-insensitive. Returns DEFAULT_MAX_CHANGE for unknown keys.

    Returns:
        Float fraction, e.g. 0.07 means ±7%.
    """
    return MARKET_MAX_CHANGE.get((market_key or "").upper(), DEFAULT_MAX_CHANGE)


@dataclass
class PredictionResult:
    """Result from a prediction algorithm."""
    predicted_price: float
    confidence: float  # 0.0 - 1.0
    current_price: float
    algorithm_name: str


class PredictionAlgorithm(ABC):
    """All prediction algorithms must implement this interface."""

    # Market key set by the registry when algorithms are instantiated per-market.
    # Algorithms use this to apply market-aware clamp limits.
    _market_key: str = ""

    # Symbol key set by the registry when algorithms are instantiated per-symbol.
    # None for pooled (per-market) instances; non-empty string for per-symbol instances.
    _symbol_key: str | None = None

    @abstractmethod
    def predict(self, prices: list[float], volumes: list[float] | None = None) -> PredictionResult:
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

    def train(self, prices: list[float], volumes: list[float] | None = None) -> None:
        """Pre-train and cache model on training data. No-op for stateless algorithms."""

    def train_batch(self, series: "list[tuple[list[float], list[float] | None]]") -> None:
        """Train on multiple price series at once. Override for stateful algorithms.

        Args:
            series: List of (prices, volumes) tuples where prices is ASC order.
                    volumes may be None for markets without volume data.

        Default implementation falls back to calling train() on each series individually.
        Override this method for stateful algorithms that should build a single combined
        model from all series instead of overwriting the model each iteration.
        """
        for prices, volumes in series:
            self.train(prices, volumes)

    def is_trained(self) -> bool:
        """Return True if this instance has a cached trained model."""
        return False
