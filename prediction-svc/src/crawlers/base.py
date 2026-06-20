"""Abstract base class for all crawlers."""
from __future__ import annotations

from abc import ABC, abstractmethod

from src.utils.logger import get_logger

log = get_logger("crawler.base")

USER_AGENT = (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
    "AppleWebKit/537.36 (KHTML, like Gecko) "
    "Chrome/124.0.0.0 Safari/537.36"
)

DEFAULT_HEADERS = {
    "User-Agent": USER_AGENT,
    "Accept": "application/json",
}


class BaseCrawler(ABC):
    @abstractmethod
    def crawl(self) -> int:
        """Crawl latest data. Returns number of records saved."""

    @abstractmethod
    def crawl_history(self, days: int = 180) -> int:
        """Crawl historical data. Returns number of records saved."""
