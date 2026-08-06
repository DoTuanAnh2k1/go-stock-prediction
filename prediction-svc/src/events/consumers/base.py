"""Shared consumer loop — poll a topic, dispatch each message to a handler.

At-least-once: the offset is committed only after the handler returns without
raising. A handler error is logged and the message is retried on the next poll
(the offset is not advanced). Each message runs under a fresh request_id so the
whole event chain is greppable across services.
"""
from __future__ import annotations

import json
import signal
import uuid
from collections.abc import Callable

import structlog

from src.config import get_settings

log = structlog.get_logger()

_running = True


def _stop(*_a) -> None:
    global _running
    _running = False


def run_consumer(topic: str, group: str, handler: Callable[[dict], None]) -> None:
    """Consume `topic` in consumer-group `group`, calling handler(payload) per message."""
    cfg = get_settings()
    if not cfg.kafka_enabled:
        log.warning("kafka.consumer.disabled", topic=topic, group=group)
        return

    from confluent_kafka import Consumer  # imported here so the toggle-off path needs no lib

    signal.signal(signal.SIGTERM, _stop)
    signal.signal(signal.SIGINT, _stop)

    consumer = Consumer(
        {
            "bootstrap.servers": cfg.kafka_brokers,
            "group.id": group,
            "auto.offset.reset": "earliest",
            "enable.auto.commit": False,  # commit manually after successful handle
        }
    )
    consumer.subscribe([topic])
    log.info("kafka.consumer.start", topic=topic, group=group, brokers=cfg.kafka_brokers)

    try:
        while _running:
            msg = consumer.poll(1.0)
            if msg is None:
                continue
            if msg.error():
                log.error("kafka.consumer.msg.error", topic=topic, error=str(msg.error()))
                continue
            rid = str(uuid.uuid4())
            structlog.contextvars.bind_contextvars(request_id=rid)
            try:
                payload = json.loads(msg.value().decode())
                key = msg.key().decode() if msg.key() else ""
                log.info("kafka.consume", topic=topic, group=group, key=key)
                handler(payload)
                consumer.commit(msg)  # at-least-once: advance offset only on success
            except Exception as exc:
                log.error("kafka.consumer.handle.error", topic=topic, error=str(exc))
                # do not commit → message is redelivered on the next poll
            finally:
                structlog.contextvars.unbind_contextvars("request_id")
    finally:
        consumer.close()
        log.info("kafka.consumer.stop", topic=topic, group=group)
