"""predict-consumer — reacts to `market.crawled`, runs predictions, emits
`predictions.ready`. Bot simulation is left to the simulation-consumer.

Run: python -m src.events.consumers.predict
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
        log.warning("predict.consumer.no_market", payload=payload)
        return
    from src.orchestrator.runner import run_for_market

    n = run_for_market(market, run_sim=False)  # simulation-consumer owns the bot step
    log.info("predict.consumer.done", market=market, predictions=n)
    cfg = get_settings()
    publish(cfg.kafka_topic_predictions, key=market, payload={"market": market, "predictions": n})


def main() -> None:
    cfg = get_settings()
    run_consumer(cfg.kafka_topic_crawled, group="predict-consumer", handler=handle)


if __name__ == "__main__":
    main()
