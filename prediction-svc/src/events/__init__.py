"""Kafka event-driven pipeline (opt-in via KAFKA_ENABLED).

Producers publish domain events (market.crawled, predictions.ready,
predictions.reconciled); the consumers under src.events.consumers react to them,
decoupling the crawl → predict → reconcile → simulation pipeline stages.
"""
