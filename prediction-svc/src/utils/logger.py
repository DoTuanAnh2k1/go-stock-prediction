"""Structured logging configuration using structlog."""
from __future__ import annotations

import logging
import os
import sys

import structlog


def init_logger(log_level: str = "INFO") -> None:
    """Configure structlog for the application.

    Output format is controlled by the LOG_FORMAT env var:
      console (default) — colored human-readable output, ANSI forced ON even
                          without a TTY (suits Docker stdout).
      json              — machine-readable JSON (opt-in for log aggregators).
    """
    level = getattr(logging, log_level.upper(), logging.INFO)

    log_format = os.environ.get("LOG_FORMAT", "console").lower()
    if log_format == "json":
        renderer = structlog.processors.JSONRenderer()
    else:
        renderer = structlog.dev.ConsoleRenderer(colors=True)

    logging.basicConfig(
        format="%(message)s",
        stream=sys.stdout,
        level=level,
    )

    # Quiet noisy third-party loggers so the app's own structured logs stay
    # readable. SQLAlchemy in particular floods raw SQL at INFO/DEBUG when the
    # root level is low (e.g. LOG_LEVEL=DEBUG); cap these at WARNING regardless.
    for noisy in (
        "sqlalchemy.engine",
        "sqlalchemy.pool",
        "sqlalchemy.dialects",
        "sqlalchemy.orm",
        "apscheduler",
        "urllib3",
        "httpx",
        "httpcore",
        "yfinance",
        "peewee",
    ):
        logging.getLogger(noisy).setLevel(logging.WARNING)

    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            renderer,
        ],
        wrapper_class=structlog.make_filtering_bound_logger(level),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


def get_logger(name: str = "prediction") -> structlog.BoundLogger:
    return structlog.get_logger(name)
