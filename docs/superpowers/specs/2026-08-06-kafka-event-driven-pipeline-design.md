# Design — Kafka event-driven pipeline

Date: 2026-08-06
Status: Approved (broker = Apache Kafka + Zookeeper; scope = full pipeline event-driven)
Motivation: The microservices rubric values real async/event-driven messaging. Today the
hourly pipeline runs **sequentially in one process** (`_run_pipeline`: crawl → train → predict
→ reconcile). This adds a **real Kafka pub/sub** so pipeline stages become decoupled
producers/consumers — independently deployable, scalable, retryable, replayable.

## Non-goals / guardrails
- **Toggle, never a hard dependency.** `KAFKA_ENABLED=false` by default → the pipeline runs the
  existing sequential way (zero behavior change). `true` → event-driven. Mirrors the repo's
  `SCHEDULER_ENABLED` / `SERVICE_MGT_ENABLED` factor toggles.
- Reuse the existing stage functions (`run_for_market`, `reconcile_predictions`,
  `run_live_step_for_market`) — no rewrite of business logic.

## Broker
- **Apache Kafka + Zookeeper**, single broker + single ZK (not a cluster) — chart `deploy/helm/kafka/`
  (StatefulSets + Services + a topic-init Job) and compose services `zookeeper` + `kafka`.
- Modest resources (kind is tight): Kafka ~768Mi/1Gi, ZK ~256Mi. One partition-set, RF=1.
- Client: Python `confluent-kafka` (pyproject `[kafka]` extra). Go client not needed — the
  pipeline lives in prediction-svc; api-svc is DB-less/thin.

## Topics
| Topic | Key | Payload | Producer | Consumers |
|-------|-----|---------|----------|-----------|
| `market.crawled` | market | `{market, crawled, trained, ts}` | crawl stage of `_run_pipeline` | predict-consumer |
| `predictions.ready` | market | `{market, predictions, ts}` | predict-consumer | reconcile-consumer, simulation-consumer |
| `predictions.reconciled` | market | `{market, scored, ts}` | reconcile-consumer | (metrics/audit) |

## Flow (KAFKA_ENABLED=true)
```
cron/trigger → _run_pipeline: crawl → (every 10th) train → PUBLISH market.crawled → return
   market.crawled ─▶ predict-consumer:    run_for_market(m, run_sim=False) → PUBLISH predictions.ready
   predictions.ready ─▶ reconcile-consumer: reconcile_predictions(only_market=m) → PUBLISH predictions.reconciled
   predictions.ready ─▶ simulation-consumer: run_live_step_for_market(m)
```
When `KAFKA_ENABLED=false`, `_run_pipeline` keeps doing crawl→train→predict(+sim)→reconcile inline
(unchanged), and no consumers run.

## Code
- `prediction-svc/src/events/`
  - `config`: `kafka_enabled`, `kafka_brokers`, topic names (added to `config.py` Settings).
  - `producer.py`: `publish(topic, key, payload)` — lazy singleton `confluent_kafka.Producer`;
    no-op + warn if disabled or lib missing (never crash the pipeline).
  - `consumers/base.py`: `run_consumer(topic, group, handler)` — poll loop, at-least-once
    (commit after handler), structured logging + `request_id` per message, graceful SIGTERM.
  - `consumers/{predict,reconcile,simulation}.py`: thin handlers calling the existing functions.
  - entrypoints: `python -m src.events.consumers.predict|reconcile|simulation`.
- `run_for_market(..., run_sim: bool = True)` — new flag so the predict-consumer can skip the
  inline sim (simulation-consumer owns it in event mode). Default True = unchanged.
- `_run_pipeline`: after crawl(+train), if `kafka_enabled` → `publish("market.crawled", ...)` and
  return before the inline predict/reconcile.

## Deploy
- k8s: 3 consumer Deployments in a new `deploy/helm/kafka-consumers/` chart (image = prediction-svc,
  command `python -m src.events.consumers.<x>`, envFrom prediction-config/secret + KAFKA_* ). Gated
  `--set enabled=true`. Kafka chart separate. Topic-init Job creates the 3 topics.
- compose: `zookeeper`, `kafka`, `kafka-init` (topics), and 3 consumer services (profile `kafka`,
  off unless `KAFKA_ENABLED=true`). ofelia gold/crypto jobs keep crawling; events fan out the rest.

## Delivery semantics
- At-least-once (commit offset after successful handle). Reconcile is idempotent. Predict for
  `target=now+1h` is naturally deduped by the upsert/unique constraints; a reprocessed
  `market.crawled` at worst re-runs predict for the same hour (idempotent enough for the demo).

## Verify
- Mechanical: `py_compile` + `ruff` (events module), `helm lint`/`template` (kafka + consumers),
  `docker compose config`. Live (optional, heavy on kind): deploy Kafka + set KAFKA_ENABLED=true,
  trigger a crawl, watch `market.crawled → predictions.ready → predictions.reconciled` flow.
