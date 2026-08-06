{{/*
=============================================================================
_ambassador.tpl — partial DÙNG CHUNG cho pattern multi-container ambassador
(CKAD Lab 1.2) ở 5 backend service: api-svc, auth-svc, prediction-svc,
service-mgt, cli-svc.

Chỉ gom các KHỐI GIỐNG HỆT NHAU giữa các deployment (tolerations, topology
spread, init wait-db, log-sidecar, nginx ambassador, nginx.conf gRPC). Phần
KHÁC NHAU thật sự (app container, probes, resources app, volumes, dnsConfig,
fsGroup, strategy, thứ tự container) vẫn để INLINE trong từng *-deployment.yaml
— cố nhét hết vào 1 template sẽ thành "param soup" khó đọc hơn cả bản lặp.

Cách dùng: `{{ include "stock.<name>" <arg> | nindent <n> }}` — content define ở
cột 0, nindent tự thụt đúng mức của chỗ chèn.
=============================================================================
*/}}

{{/*
stock.tolerations — HA: cho phép schedule lên node control-plane (dùng cả 3 node).
Không tham số. Chèn dưới `tolerations:` (nindent 8).
*/}}
{{- define "stock.tolerations" -}}
- key: node-role.kubernetes.io/control-plane
  operator: Exists
  effect: NoSchedule
{{- end -}}

{{/*
stock.topologySpread — HA: rải pod đều các node theo hostname; spread theo từng
ReplicaSet (matchLabelKeys pod-template-hash) để rollout không kẹt.
Tham số: chuỗi tên app (dùng cho labelSelector). Chèn dưới
`topologySpreadConstraints:` (nindent 8).
*/}}
{{- define "stock.topologySpread" -}}
- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: DoNotSchedule
  matchLabelKeys: [pod-template-hash]
  labelSelector:
    matchLabels: { app: {{ . }} }
{{- end -}}

{{/*
stock.waitDb — initContainer chờ DB Ready (pg_isready) rồi app mới start.
Tham số dict: image (image có sẵn pg_isready — thường timescaledb).
Chèn dưới `initContainers:` (nindent 8).
*/}}
{{- define "stock.waitDb" -}}
- name: wait-db
  image: {{ .image }}
  imagePullPolicy: IfNotPresent
  # CKAD 3.2 — init chỉ chạy pg_isready (network client): drop mọi capability, cấm
  # privilege escalation. Không cần cap nào để mở socket TCP tới db:5432.
  securityContext:
    allowPrivilegeEscalation: false
    capabilities:
      drop: ["ALL"]
  command:
    - sh
    - -c
    - |
      echo "waiting for db:5432 ..."
      until pg_isready -h db -p 5432 -U postgres; do
        echo "db not ready, retry in 2s"; sleep 2
      done
      echo "db is ready"
{{- end -}}

{{/*
stock.logSidecar — sidecar tail -f log app qua emptyDir "logs" chung.
Tham số dict: image, logfile (tên file trong /var/log/app), và (tùy chọn)
reqCpu/reqMem/limCpu/limMem override resources. Chèn dưới `containers:` (nindent 8).
*/}}
{{- define "stock.logSidecar" -}}
- name: log-sidecar
  image: {{ .image }}
  imagePullPolicy: IfNotPresent
  # CKAD 3.2 — sidecar chỉ `tail -f` file trên emptyDir "logs" (mount, vẫn ghi được):
  # rootfs read-only, drop mọi capability, cấm privilege escalation. Không ghi gì lên /.
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop: ["ALL"]
  command: ["sh", "-c", "until [ -f /var/log/app/{{ .logfile }} ]; do sleep 1; done; tail -f /var/log/app/{{ .logfile }}"]
  volumeMounts:
    - name: logs
      mountPath: /var/log/app
  resources:
    requests: { cpu: {{ .reqCpu | default "10m" | quote }}, memory: {{ .reqMem | default "16Mi" | quote }} }
    limits:   { cpu: {{ .limCpu | default "50m" | quote }}, memory: {{ .limMem | default "64Mi" | quote }} }
{{- end -}}

{{/*
stock.nginxAmbassador — container nginx reverse-proxy (Service targetPort trỏ vào).
Tham số dict: portName, port (cổng nginx lắng nghe), root (bool — cli-svc cần
runAsUser 0 để ghi /run/nginx.pid vì pod ép non-root). Volume "nginx-conf" +
"logs"? Chỉ mount nginx-conf. Chèn dưới `containers:` (nindent 8).
*/}}
{{- define "stock.nginxAmbassador" -}}
- name: nginx
  image: nginx:1.27-alpine
  imagePullPolicy: IfNotPresent
  # CKAD 3.2 — drop ALL rồi ADD LẠI đúng 3 cap official nginx image cần:
  #   CHOWN  — entrypoint (root) chown /var/cache/nginx/* sang user nginx (uid 101) lúc start
  #   SETUID/SETGID — nginx master (root) hạ quyền worker xuống user nginx (compiled default)
  # (listen cổng cao >1024 nên KHÔNG cần NET_BIND_SERVICE). Vẫn drop ~35 cap còn lại + no
  # priv-esc. KHÔNG readOnlyRootFilesystem: nginx ghi /var/run/nginx.pid + /var/cache/nginx.
  # root=true (cli-svc): override runAsUser 0 để ghi được pid (pod ép non-root 10001).
  securityContext:
    allowPrivilegeEscalation: false
    capabilities:
      drop: ["ALL"]
      add: ["CHOWN", "SETUID", "SETGID"]
{{- if .root }}
    runAsUser: 0        # override pod-level non-root: nginx cần ghi /run/nginx.pid
{{- end }}
  ports:
    - { name: {{ .portName }}, containerPort: {{ .port }} }
  volumeMounts:
    - name: nginx-conf
      mountPath: /etc/nginx/nginx.conf
      subPath: nginx.conf
  resources:
    requests: { cpu: "10m", memory: "16Mi" }
    limits:   { cpu: "100m", memory: "64Mi" }
{{- end -}}

{{/*
stock.nginxConf.grpc — thân nginx.conf ambassador gRPC (http2 + grpc_pass) dùng
chung bởi auth-svc/prediction-svc/service-mgt. Tham số dict: listen (cổng nginx),
upstream (cổng app). Chèn dưới `nginx.conf: |` (nindent 4).
*/}}
{{- define "stock.nginxConf.grpc" -}}
worker_processes 1;
error_log /dev/stderr warn;
events { worker_connections 1024; }
http {
  access_log /dev/stdout;
  server {
    listen {{ .listen }};
    http2 on;
    location / {
      grpc_pass grpc://127.0.0.1:{{ .upstream }};
    }
  }
}
{{- end -}}
