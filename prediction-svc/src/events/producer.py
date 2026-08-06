"""Kafka producer helper — publish domain events.

Safe by construction: if KAFKA_ENABLED is false or confluent-kafka isn't installed,
publish() is a no-op that logs a warning. Publishing must NEVER crash the pipeline.
"""
from __future__ import annotations

import json
from datetime import datetime
from typing import Any

import structlog

from src.config import get_settings

log = structlog.get_logger()

_producer: Any = None
_unavailable = False


def _get_producer() -> Any | None:
    """Lazily build a singleton confluent_kafka.Producer; None if unavailable."""
    global _producer, _unavailable
    if _producer is not None:
        return _producer
    if _unavailable:
        return None
    cfg = get_settings()
    try:
        from confluent_kafka import Producer  # noqa: PLC0415
    except ImportError:
        _unavailable = True
        log.warning("kafka.producer.unavailable", reason="confluent-kafka not installed")
        return None
    try:
        _producer = Producer(
            {
                "bootstrap.servers": cfg.kafka_brokers,
                "enable.idempotence": True,
                "acks": "all",
                "linger.ms": 20,
                "client.id": "prediction-svc",
            }
        )
        log.info("kafka.producer.ready", brokers=cfg.kafka_brokers)
        return _producer
    except Exception as exc:  # pragma: no cover - broker/config errors
        _unavailable = True
        log.error("kafka.producer.init.error", error=str(exc))
        return None


def publish(topic: str, key: str, payload: dict) -> None:
    """Publish a JSON event to a topic keyed by `key` (e.g. the market).

    No-op when Kafka is disabled or unavailable. `ts` is stamped automatically.
    """
    cfg = get_settings()
    if not cfg.kafka_enabled:
        return
    producer = _get_producer()
    if producer is None:
        return
    body = {"ts": datetime.now().isoformat(), **payload}
    try:
        producer.produce(topic, key=key.encode(), value=json.dumps(body).encode())
        producer.poll(0)  # serve delivery callbacks without blocking
        log.info("kafka.publish", topic=topic, key=key)
    except Exception as exc:  # never break the caller's pipeline
        log.error("kafka.publish.error", topic=topic, key=key, error=str(exc))


def flush(timeout: float = 5.0) -> None:
    """Block until queued messages are delivered (call before shutdown)."""
    if _producer is not None:
        try:
            _producer.flush(timeout)
        except Exception:
            pass
