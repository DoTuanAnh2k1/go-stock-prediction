"""reconcile-consumer — reacts to `predictions.ready`, scores matured predictions
for that market, emits `predictions.reconciled`.

Run: python -m src.events.consumers.reconcile
"""
from __future__ import annotations

import structlog

from src.config import get_settings
from src.events.consumers.base import run_consumer
from src.events.producer import publish

log = structlog.get_logger()


def handle(payload: dict) -> None:
    market = payload.get("market", "")
    if not market:
        log.warning("reconcile.consumer.no_market", payload=payload)
        return
    from src.orchestrator.training import reconcile_predictions

    scored = reconcile_predictions(only_market=market)
    log.info("reconcile.consumer.done", market=market, scored=scored)
    cfg = get_settings()
    publish(cfg.kafka_topic_reconciled, key=market, payload={"market": market, "scored": scored})


def main() -> None:
    cfg = get_settings()
    run_consumer(cfg.kafka_topic_predictions, group="reconcile-consumer", handler=handle)


if __name__ == "__main__":
    main()
