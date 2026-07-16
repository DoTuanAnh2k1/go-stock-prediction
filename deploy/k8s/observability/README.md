# Observability stack (namespace `observability`)

Backend quan sát cho toàn stack: **traces** (OpenTelemetry → Tempo) và
**metrics** (Prometheus), hiển thị hợp nhất trong **Grafana**.

```
6 service app  ──OTLP/gRPC──►  otel-collector  ──►  tempo   ──┐
(traceparent)                    :4317/:4318         :4317    │
                                                     :3200 ◄──┤ Grafana
pod annotations ─(kubernetes_sd)─► prometheus :9090 ◄────────┘  :3000
prometheus.io/scrape
```

| Thành phần | File | Ports | Ghi chú |
|---|---|---|---|
| Namespace | `namespace.yaml` | — | `observability` |
| OTel Collector | `otel-collector.yaml` | 4317 gRPC / 4318 HTTP | OTLP receiver → Tempo + debug |
| Tempo | `tempo.yaml` | 4317 OTLP / 3200 HTTP | single-binary, local storage, PVC 1Gi |
| Prometheus | `prometheus.yaml` | 9090 | kubernetes_sd pod discovery + RBAC, PVC 1Gi, retention 6h |
| Grafana | `grafana.yaml` | 3000 (NodePort 30300) | anonymous Admin, datasource Prometheus + Tempo, dashboard mẫu |

## Apply (theo thứ tự — exporter/datasource phụ thuộc lẫn nhau)

```bash
kubectl apply -f deploy/k8s/observability/namespace.yaml
kubectl apply -f deploy/k8s/observability/tempo.yaml
kubectl apply -f deploy/k8s/observability/otel-collector.yaml   # trỏ tới tempo:4317
kubectl apply -f deploy/k8s/observability/prometheus.yaml
kubectl apply -f deploy/k8s/observability/grafana.yaml          # datasource -> prometheus + tempo

# hoặc apply cả thư mục (k8s tự resolve, chỉ cần re-apply nếu có lỗi thứ tự)
kubectl apply -f deploy/k8s/observability/

# chờ Ready
kubectl -n observability rollout status deploy/tempo
kubectl -n observability rollout status deploy/otel-collector
kubectl -n observability rollout status deploy/prometheus
kubectl -n observability rollout status deploy/grafana
```

Các image cần load vào kind (nếu cluster không có internet):
```bash
for img in \
  otel/opentelemetry-collector-contrib:0.109.0 \
  grafana/tempo:2.6.1 \
  prom/prometheus:v2.54.1 \
  grafana/grafana:11.2.2 ; do
  docker pull "$img" && kind load docker-image "$img" --name ckad
done
```

## Mở Grafana

```bash
# Cách 1 — port-forward (luôn dùng được)
kubectl -n observability port-forward svc/grafana 3000:3000
# → http://localhost:3000  (vào thẳng, anonymous Admin, không cần login)

# Cách 2 — NodePort 30300 (nếu kind map port ra host)
# → http://<node-ip>:30300
```

Dashboard mẫu: **Stock Prediction — Services Overview** (request rate, latency
p95, error rate per service + 1 panel traces Tempo). Datasource Prometheus là
default; Tempo có sẵn ở tab **Explore** để search trace bằng TraceQL (`{}` = tất cả).

## Metrics — pod annotation contract

Prometheus scrape mọi pod (toàn cluster) có annotation trong pod template:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port:   "8118"        # 8118 | 9100 | 9464 | 8120 ...
    prometheus.io/path:   "/metrics"    # hoặc /actuator/prometheus (Java)
```

Không cần Service cho scrape — discovery theo pod IP + port annotation.
Series được gán label `service` (từ pod label `app`), `namespace`, `pod`.

## Traces — sinh traffic để thấy trace

Trace xuất hiện khi service app đã instrument OTel và có request đi qua chuỗi.

```bash
# 1) Đảm bảo 6 service app đã có env OTEL_EXPORTER_OTLP_ENDPOINT trỏ collector:
#    http://otel-collector.observability:4317   (protocol grpc)

# 2) Bắn request qua gateway để tạo chuỗi trace gateway→api-svc→{auth,prediction}:
kubectl -n stock port-forward svc/gateway-svc 8080:80 &
TOKEN=$(curl -s -X POST http://localhost:8080/api/x/grant \
  -H "Content-Type: application/json" \
  -H "X-Token: $(printf '%s' 'chon:Ch1nch2n@' | base64)" \
  -d '{"request":""}' | jq -r '.token')
curl -s http://localhost:8080/api/monitoring/overview -H "Authorization: Bearer $TOKEN" >/dev/null
curl -s -X POST http://localhost:8080/api/trigger/reconcile -H "Authorization: Bearer $TOKEN" >/dev/null

# 3) Trong Grafana → Explore → datasource Tempo → Search (TraceQL `{}` hoặc theo service)
#    → mở trace, thấy span xuyên gateway (Rust) → api-svc (Go) → auth/prediction.

# 4) Debug nhanh trace tại collector (in ra stdout):
kubectl -n observability logs deploy/otel-collector -f
```

Nếu chưa thấy trace: kiểm tra service đã set OTLP endpoint đúng, và collector
log có dòng `TracesExporter ... spans`. Metrics thì check Prometheus:
`kubectl -n observability port-forward svc/prometheus 9090:9090` →
`http://localhost:9090/targets` (mọi pod annotated phải UP).
