"""simulation-consumer — reacts to `predictions.ready`, runs the bot live-step for
that market (the stage that used to run inline inside run_for_market).

Run: python -m src.events.consumers.simulation
"""
from __future__ import annotations

import structlog

from src.config import get_settings
from src.events.consumers.base import run_consumer

log = structlog.get_logger()


def handle(payload: dict) -> None:
    market = payload.get("market", "")
    if not market:
        log.warning("simulation.consumer.no_market", payload=payload)
        return
    from src.simulation.engine import SimulationEngine

    SimulationEngine().run_live_step_for_market(market)
    log.info("simulation.consumer.done", market=market)


def main() -> None:
    cfg = get_settings()
    run_consumer(cfg.kafka_topic_predictions, group="simulation-consumer", handler=handle)


if __name__ == "__main__":
    main()
