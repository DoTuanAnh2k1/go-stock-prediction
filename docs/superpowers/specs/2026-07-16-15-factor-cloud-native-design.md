# 15-Factor Cloud-Native Compliance — Design & Conventions

> Ngày: 2026-07-16 · Branch: `k8s` · Trạng thái: approved, đang implement.
> Mục tiêu: đưa toàn bộ stack đạt chuẩn 15-factor (Beyond the Twelve-Factor App),
> **trừ factor #15 secret** (giữ nguyên plaintext `secret.yaml` cho mục đích education).

## Quyết định đã chốt

| Câu | Quyết định |
|-----|-----------|
| Mức độ | **Production-grade full stack** — deploy backend observability thật trên kind |
| Obs stack | **Grafana-unified**: Prometheus + Grafana + **Tempo** (traces) + **OTel Collector** |
| Tracing scope | **Cả 6 service app** (Go×3, Python, Java, Rust) |
| Stateless prediction-svc | **Full**: checkpoint→MinIO + tắt APScheduler(k8s) + `replicas: 2` |
| Parity scheduler | **Unify**: CronJob cho k8s + cron container cho compose; gỡ hẳn in-app scheduler |
| auth.proto | Gộp về 1 nguồn (factor 13 housekeeping) — làm trong Phase C |
| Secret (#15) | **KHÔNG đụng** — giữ plaintext |

## Namespaces
- `stock` — app hiện tại + **MinIO** (mới).
- `observability` — **OTel Collector, Prometheus, Tempo, Grafana** (mới).

---

## CONVENTIONS (hợp đồng chung — mọi service tuân theo y hệt)

### Tracing (OpenTelemetry → OTLP/gRPC → Collector → Tempo)
Env var chuẩn OTel, set qua ConfigMap cho từng service:
```
OTEL_EXPORTER_OTLP_ENDPOINT = http://otel-collector.observability:4317
OTEL_EXPORTER_OTLP_PROTOCOL = grpc
OTEL_SERVICE_NAME           = <service-name>   # api-svc | auth-svc | prediction-svc | gateway-svc | cli-svc | service-mgt
OTEL_TRACES_SAMPLER         = parentbased_always_on   # education: trace hết
OTEL_RESOURCE_ATTRIBUTES    = service.namespace=stock,deployment.environment=kind
```
Yêu cầu instrument:
- **Extract** context từ request đến (HTTP header / gRPC metadata — W3C `traceparent`).
- **Inject** context vào mọi outbound call (HTTP client, gRPC client) để nối chuỗi.
- Chuỗi phải liền: `gateway (Rust,HTTP) → api-svc (Go,HTTP) → {auth-svc (Java,gRPC), prediction-svc (Python,gRPC)}`; `service-mgt` nhận gRPC register/heartbeat; `cli-svc` là HTTP client → gateway.
- Exporter: OTLP/gRPC tới `otel-collector.observability:4317`.

### Metrics (native Prometheus client → scrape trực tiếp)
- Mỗi service expose `GET /metrics` (Prometheus text format).
  - HTTP service: dùng port sẵn có nếu tiện, KHÔNG proxy ra ngoài. `api-svc` → `/metrics` trên :8118. `gateway-svc` → `/metrics` trên **admin port :9100** (không nằm trong route proxy).
  - gRPC-only / non-HTTP service (`prediction-svc`, `auth-svc`, `service-mgt`, `cli-svc`): mở HTTP metrics server phụ trên **:9464**.
- Metric tối thiểu: request/rpc count, latency histogram, error count + default runtime metrics (Go/Python/JVM/process).
- Prometheus discover qua **pod annotations** (kubernetes_sd):
```
prometheus.io/scrape: "true"
prometheus.io/port:   "<metrics-port>"     # 8118 | 9100 | 9464 | 8120(actuator) ...
prometheus.io/path:   "/metrics"
```
- Java: dùng Spring Boot Actuator + `micrometer-registry-prometheus` → `/actuator/prometheus` (path khai trong annotation).

### Health probes (factor 14 status)
Mọi Deployment phải có `readinessProbe` + `livenessProbe`:
- HTTP service: `httpGet` tới health path.
- gRPC service (`prediction-svc`, `auth-svc`, `service-mgt`): `grpc` probe (k8s ≥1.24 native `grpc:` field) hoặc `exec grpc_health_probe`. Ưu tiên native `grpc:` field.

### Ports tổng hợp
| Service | App port | Metrics | Trace export |
|---|---|---|---|
| gateway-svc (Rust) | 80/443 | :9100 `/metrics` | OTLP→collector |
| api-svc (Go) | 8118 | :8118 `/metrics` | OTLP→collector |
| auth-svc (Java) | 8120 gRPC | :8120 `/actuator/prometheus` (actuator HTTP) | OTel javaagent→collector |
| prediction-svc (Python) | 8119 gRPC | :9464 `/metrics` | OTLP→collector |
| service-mgt (Go) | 8121 gRPC | :9464 `/metrics` | OTLP→collector |
| cli-svc (Go) | 2345 SSH | :9464 `/metrics` | OTLP→collector |

---

## Phase A — Telemetry (factor 14)
1. **Infra**: manifests `deploy/k8s/observability/` — namespace, OTel Collector (Deployment+Config: OTLP receiver → Tempo exporter + debug), Tempo (single-binary + PVC nhỏ), Prometheus (Deployment+Config kubernetes_sd pod annotations), Grafana (Deployment + datasource Prometheus & Tempo + 1 dashboard mẫu).
2. **Per-service**: OTel SDK tracing + `/metrics` + pod annotations + probes cho cả 6 service.

## Phase B — Stateless prediction-svc (factor 4/6/8)
1. **MinIO**: manifest `deploy/k8s/minio/` trong ns `stock` (Deployment + PVC + Service + bucket `models` init). Env cho prediction: `MODEL_STORE_BACKEND=s3`, `S3_ENDPOINT=http://minio.stock:9000`, `S3_BUCKET=models`, `S3_ACCESS_KEY`/`S3_SECRET_KEY` (giữ trong secret giáo dục).
2. **Code**: `prediction-svc/src/storage/model_store.py` — abstraction `save(path)/load(path)` với backend `local` (mặc định, cho test/compose) hoặc `s3` (MinIO). Read-through cache local `/models` (ephemeral emptyDir). Wire vào `rl_dqn.py`, `transformer_model.py`, `training.py` (meta) — các chỗ `torch.save/load` + `pickle`.
3. **Deploy**: `prediction-svc/deployment.yaml` → `replicas: 2`; `/models` PVC → `emptyDir` (cache); thêm env S3; probe gRPC.

## Phase C — Scheduler unification + housekeeping (factor 10/13)
1. **CronJob còn thiếu** (`deploy/k8s/pipeline/`, mirror `cronjob-crypto.yaml`): `train_gold`, `train_nasdaq`, `train_crypto`, `train_sp500`, `train_meta`, `crawler_fundamentals`, `train_transformer`, `simulation_daily`, `daily_backup`.
   - Cần bổ sung entrypoint tương ứng trong `jobs_cli.py` (hoặc `jobs_cli.py <job_key>` tổng quát map vào `JOB_FUNCTIONS`). `daily_backup` chạy pg_dump — có thể là CronJob riêng gọi api-svc trigger hoặc chạy pg_dump trực tiếp.
2. **Gỡ in-app scheduler**: `prediction-svc/src/main.py` không `init_scheduler()` khi k8s (guard bằng env `SCHEDULER_ENABLED=false`); `api-svc` BackupScheduler tương tự (env `BACKUP_SCHEDULER_ENABLED=false`).
3. **Compose parity**: thêm service `cron` trong `deploy/docker-compose.yaml` (ofelia hoặc cron container) chạy `jobs_cli.py` theo lịch; app service tắt in-app scheduler.
4. **auth.proto**: 1 nguồn sự thật; generate cả Go + Java từ đó. Cập nhật CLAUDE.md ghi chú.

## Thứ tự thực thi
Fan-out song song theo service cho Phase A code (file rời nhau). Infra observability + MinIO song song. Phase B/C code của prediction-svc gộp cùng agent Python. Cross-cutting (CronJob mới, compose cron, proto merge) do main làm ở integration.

## Verify (mỗi phase)
- Build image từng service OK (`go build` / `cargo build` / `mvn -q compile` / `python -c import`).
- `kind load` + `kubectl apply` + pod Ready.
- Grafana: thấy metrics 6 service; 1 request qua gateway → trace liền mạch xuyên 6 service trong Tempo.
- Không job nào chạy đôi; không job nào mất (so với DEFAULT_SCHEDULES).
- `make test` (Go + Python) vẫn pass.
