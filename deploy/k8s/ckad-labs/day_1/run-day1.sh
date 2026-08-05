#!/usr/bin/env bash
# =============================================================================
# CKAD — Day 1 — chạy TRỌN Lab 1.3 (Jobs & CronJobs) + Lab 1.4 (Label & Annotation)
# In từng lệnh + OUTPUT THẬT, chạy "1 loạt" để demo/nộp bài.
#
#   ./run-day1.sh                 # cả 1.3 + 1.4, cuối tự dọn (demo-*, verify/fail job)
#   ONLY=1.3 ./run-day1.sh        # chỉ Lab 1.3   (ONLY=1.4 → chỉ 1.4)
#   SKIP_RETRY=1 ./run-day1.sh    # bỏ demo retry (Job cố tình exit 1) cho nhanh
#   KEEP=1 ./run-day1.sh          # GIỮ lại object đã tạo (không dọn) để tự xem thêm
#   -- TỐC ĐỘ Lab 1.3 (pipeline crypto thật nặng ~7' — predict 13 thuật toán×3 coin + 390 bot) --
#   ./run-day1.sh                      # MẶC ĐỊNH: bắn Job crypto, hiện Running + mốc đầu (~90s) rồi để chạy nền
#   WAIT_COMPLETE=1 ./run-day1.sh      # CHỜ tới 'Complete 1/1' (crypto ~7'); dùng khi cần transcript đầy đủ
#   SKIP_VERIFY=1 ./run-day1.sh        # bỏ hẳn Job pipeline thật; vòng đời Job vẫn demo qua fail-demo (~40s)
#   VERIFY_MARKET=crypto ./run-day1.sh # đổi market Job verify: crypto(mặc định) | gold(~2') | nasdaq | sp500
#   VERIFY_TIMEOUT=120 ./run-day1.sh   # ngân sách poll (giây); mặc định 90 (nhanh) hoặc 600 khi WAIT_COMPLETE=1
#
# Yêu cầu: context=kind-ckad, ns=stock, image api-svc:dev & prediction-svc:dev đã kind load
#          (đúng môi trường project trên nhánh k8s — coi ns stock là sandbox).
# =============================================================================
set -uo pipefail          # KHÔNG set -e: Lab 1.4 có 1 lệnh CỐ TÌNH lỗi (drill --overwrite)

NS=stock
IMG=api-svc:dev
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"   # repo root
PIPE="$ROOT/deploy/k8s/pipeline"
ONLY="${ONLY:-all}"
VERIFY_MARKET="${VERIFY_MARKET:-crypto}" # market cho Job verify (đúng ví dụ lab gốc)
VJOB="verify-${VERIFY_MARKET}"           # tên Job; dùng cả ở cleanup

# ---- màu (tắt khi không phải terminal) --------------------------------------
if [ -t 1 ]; then B=$'\e[1m'; C=$'\e[36m'; Y=$'\e[33m'; G=$'\e[32m'; R=$'\e[31m'; D=$'\e[2m'; X=$'\e[0m'
else B=; C=; Y=; G=; R=; D=; X=; fi

hr(){ printf '%s%s%s\n' "$D" "────────────────────────────────────────────────────────────────" "$X"; }
title(){ printf '\n'; hr; printf '%s%s  %s%s\n' "$B" "$C" "$1" "$X"; hr; }
note(){ printf '%s# %s%s\n' "$D" "$1" "$X"; }
run(){ printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"; "$@"; }
runsh(){ printf '\n%s$ %s%s\n' "$Y" "$1" "$X"; sh -c "$1"; }   # cho lệnh có pipe
runx(){ # chạy lệnh MONG ĐỢI lỗi (demo)
  printf '\n%s$' "$Y"; printf ' %s' "$@"; printf '%s\n' "$X"
  if "$@"; then printf '%s(?!) lệnh này lẽ ra phải lỗi%s\n' "$R" "$X"
  else printf '%s↑ ĐÚNG NHƯ MONG ĐỢI — lệnh này cố tình lỗi (thiếu --overwrite)%s\n' "$G" "$X"; fi; }

# ---- prereq -----------------------------------------------------------------
ctx="$(kubectl config current-context 2>/dev/null || true)"
[ "$ctx" = "kind-ckad" ] || { printf '%sContext hiện tại = "%s" (mong đợi kind-ckad). Đổi: kubectl config use-context kind-ckad%s\n' "$R" "$ctx" "$X"; exit 1; }
kubectl get ns "$NS" >/dev/null 2>&1 || { printf '%sKhông thấy namespace %s%s\n' "$R" "$NS" "$X"; exit 1; }
DBPOD="$(kubectl get pod -n "$NS" -l app=db -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"; DBPOD="${DBPOD:-db-0}"

# =============================================================================
lab_1_3(){
title "LAB 1.3 — Jobs & CronJobs (chuyển pipeline crawl 4 market sang k8s-native)"

note "1) Liệt kê CronJob (đã apply sẵn) — SCHEDULE + TIMEZONE + SUSPEND + LAST SCHEDULE (Cron k8s 5 trường)"
run kubectl get cronjob -n "$NS" -l app=pipeline

note "   1a) Job gần đây do CronJob đẻ (k8s chỉ giữ successfulJobsHistoryLimit=3 / failed=1 mỗi CronJob)"
runsh "kubectl get jobs -n $NS --sort-by=.metadata.creationTimestamp 2>/dev/null | awk 'NR==1 || /^pipeline-/'"

note "   1b) SỐ LẦN CHẠY tích luỹ mỗi market (pipeline_crawl_counters — bền, không mất khi Job bị prune)"
runsh "kubectl exec -n $NS $DBPOD -- psql -U postgres -d go_stock_prediction -tAc \"select rpad(market_key,10)||' : '||crawl_count||' lần' from pipeline_crawl_counters order by market_key;\" 2>/dev/null"

note "   1c) LOG lần chạy GẦN NHẤT của cronjob/pipeline-$VERIFY_MARKET (pod còn trong ttl 1800s; lọc mốc chính)"
_lastjob="$(kubectl get jobs -n "$NS" --sort-by=.metadata.creationTimestamp -o name 2>/dev/null | grep "^job.batch/pipeline-$VERIFY_MARKET-" | tail -1)"
if [ -n "$_lastjob" ]; then
  printf '%s   (job gần nhất: %s)%s\n' "$D" "${_lastjob#job.batch/}" "$X"
  runsh "kubectl logs -n $NS $_lastjob --tail=-1 2>/dev/null | grep -aE 'jobs_cli\.|\.crawl\.done|pipeline\.crawl|predict\.[a-z0-9]+\.(start|done)|live_step\.market_start|reconcile\.done|intraday\.done|pipeline\.skip' | sed -E 's/\x1b\[[0-9;]*m//g'"
else
  note "   (chưa có Job lịch cho $VERIFY_MARKET trong history — có thể đã bị prune)"
fi

note "   1d) Tóm tắt lần chạy gần nhất (pipeline_reports — bền 7 ngày, sống cả khi pod đã GC)"
runsh "kubectl exec -n $NS $DBPOD -- psql -U postgres -d go_stock_prediction -c \"select status,crawled_count,predictions_count,trained,round(duration_ms/1000.0)||'s' as dur,created_at from pipeline_reports where pipeline_key='crawler_$VERIFY_MARKET' order by created_at desc limit 1;\" 2>/dev/null"

if [ "${SKIP_VERIFY:-0}" = "1" ]; then
  note "2-5) SKIP_VERIFY=1 → bỏ Job pipeline thật (nặng). Vòng đời Job vẫn demo ở bước retry bên dưới."
else
note "2) Tạo Job tức thời TỪ CronJob '$VERIFY_MARKET' để test ngay (không chờ tới giờ)"
kubectl delete job "$VJOB" -n "$NS" --ignore-not-found >/dev/null 2>&1
run kubectl create job "$VJOB" --from=cronjob/pipeline-"$VERIFY_MARKET" -n "$NS"

if [ "${WAIT_COMPLETE:-0}" = "1" ]; then _deadline="${VERIFY_TIMEOUT:-600}"
else _deadline="${VERIFY_TIMEOUT:-90}"; fi
note "3) Chờ Job — Job Complete ⟺ pipeline TỰ chạy tới hết rồi exit 0"
note "   $VERIFY_MARKET pipeline nặng (~7' cho crypto). Mặc định poll ${_deadline}s rồi để chạy nền; WAIT_COMPLETE=1 → chờ tới Complete."
printf '%s$ poll: kubectl get job %s -o jsonpath (Complete/Failed)%s\n' "$Y" "$VJOB" "$X"
_t=0; _done=0
while :; do
  _cpl="$(kubectl get job "$VJOB" -n "$NS" -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' 2>/dev/null)"
  _fld="$(kubectl get job "$VJOB" -n "$NS" -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null)"
  [ "$_cpl" = "True" ] && { printf '%s  → Job Complete sau ~%ds%s\n' "$G" "$_t" "$X"; _done=1; break; }
  [ "$_fld" = "True" ] && { printf '%s  → Job Failed sau ~%ds%s\n' "$R" "$_t" "$X"; _done=1; break; }
  [ "$_t" -ge "$_deadline" ] && break
  sleep 20; _t=$((_t+20)); printf '%s  … %ds (Running)%s\n' "$D" "$_t" "$X"
done
[ "$_done" = 0 ] && note "   Job VẪN Running → chạy nền tới Complete (~7' crypto). Theo dõi: kubectl get job $VJOB -n $NS -w"

note "4) Log pod: pipeline crawl→predict→reconcile chạy TRONG pod rồi thoát (lọc các mốc chính)"
runsh "kubectl logs -n $NS -l job-name=$VJOB --tail=-1 2>/dev/null | grep -aE 'jobs_cli\.|\.crawl\.done|pipeline\.crawl|predict\.[a-z0-9]+\.(start|done)|live_step\.market_start|reconcile\.done|intraday\.done|pipeline\.skip' | sed -E 's/\x1b\[[0-9;]*m//g'"

note "5) Trạng thái Job (COMPLETIONS 1/1 = xong thật)"
run kubectl get job "$VJOB" -n "$NS"
fi

if [ "${SKIP_RETRY:-0}" != "1" ]; then
  note "6) (tuỳ chọn) DEMO RETRY — Job cố tình exit 1, backoffLimit=2 → 3 Pod Error → Job Failed"
  kubectl delete job fail-demo -n "$NS" --ignore-not-found >/dev/null 2>&1
  cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata: { name: fail-demo, namespace: $NS }
spec:
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: boom
          image: $IMG
          command: ["sh","-c","echo attempt; exit 1"]
EOF
  run kubectl wait --for=condition=failed job/fail-demo -n "$NS" --timeout=120s \
    || printf '%s(chưa Failed — có thể còn đang retry)%s\n' "$R" "$X"
  note "   → 3 Pod (1 + 2 retry), tất cả Error; Job Failed reason=BackoffLimitExceeded"
  run kubectl get pods -n "$NS" -l job-name=fail-demo
  run kubectl get job fail-demo -n "$NS"
fi
}

# =============================================================================
lab_1_4(){
title "LAB 1.4 — Label & Annotation Drill (ns stock = sandbox, chỉ nghịch demo-*)"

note "setup) Pod nháp — imperative (exam-speed). Xoá trước cho idempotent."
kubectl delete pod demo-web demo-api -n "$NS" --ignore-not-found >/dev/null 2>&1
kubectl delete deployment demo-dep -n "$NS" --ignore-not-found >/dev/null 2>&1
kubectl delete pod -n "$NS" -l app=quarantine --ignore-not-found >/dev/null 2>&1
run kubectl run demo-web --image="$IMG" -n "$NS" --labels=app=demo,tier=frontend,env=dev --command -- sleep 3600
run kubectl run demo-api --image="$IMG" -n "$NS" --labels=app=demo,tier=backend,env=prod --command -- sleep 3600
run kubectl create deployment demo-dep --image="$IMG" -n "$NS" -- sleep 3600
run kubectl scale deployment demo-dep --replicas=3 -n "$NS"
note "   chờ Pod demo Ready để query cho ra STATUS Running..."
kubectl wait --for=condition=ready pod -l app=demo -n "$NS" --timeout=90s >/dev/null 2>&1 || true

note "QUERY bằng selector"
run kubectl get pods -n "$NS" -l app=demo --show-labels
note "-L (hoa) = label thành CỘT"
run kubectl get pods -n "$NS" -l app=demo -L tier,env
note "set-based + AND"
run kubectl get pods -n "$NS" -l 'app=demo,env in (prod)'
note "bất đẳng thức"
run kubectl get pods -n "$NS" -l 'tier!=frontend'
note "key KHÔNG tồn tại"
run kubectl get pods -n "$NS" -l '!release'

note "THÊM / SỬA / XOÁ label"
run kubectl label pod demo-web release=canary -n "$NS"
runx kubectl label pod demo-web tier=backend -n "$NS"                 # LỖI: đã có value
run kubectl label pod demo-web tier=backend --overwrite -n "$NS"      # sửa đúng
run kubectl label pod demo-web release- -n "$NS"                      # xoá (hậu tố -)
run kubectl label pods -l app=demo reviewed=yes -n "$NS"              # hàng loạt theo selector

note "ANNOTATION (không query bằng selector được)"
run kubectl annotate pod demo-web owner=team-x description=demo -n "$NS"
run kubectl annotate pod demo-web owner- -n "$NS"                     # xoá annotation
run kubectl get pod demo-web -n "$NS" -o jsonpath='{.metadata.annotations}'; echo

note "KỸ THUẬT: TÁCH 1 Pod khỏi ReplicaSet → RS đẻ Pod bù, Pod tách vẫn sống để debug"
V="$(kubectl get pods -n "$NS" -l app=demo-dep -o jsonpath='{.items[0].metadata.name}')"
note "   pod bị tách = $V"
run kubectl label pod "$V" app=quarantine --overwrite -n "$NS"
sleep 2
run kubectl get pods -n "$NS" -l app=demo-dep -L app      # vẫn 3 (có 1 Pod mới bù)
run kubectl get pods -n "$NS" -l app=quarantine -L app    # Pod bị tách, vẫn Running

note "QUERY trên RESOURCE THẬT (chỉ đọc, an toàn)"
run kubectl get cronjob -n "$NS" -l app=pipeline
run kubectl get pods -n "$NS" -l app=api-svc -L app
}

# =============================================================================
cleanup(){
  [ "${KEEP:-0}" = "1" ] && { printf '\n%sKEEP=1 → giữ lại object (demo-*, verify-crypto, fail-demo).%s\n' "$D" "$X"; return; }
  title "DỌN DẸP (roll back — đồ thật giữ nguyên)"
  run kubectl delete deployment demo-dep -n "$NS" --ignore-not-found
  run kubectl delete pod -n "$NS" -l 'app in (demo,quarantine)' --ignore-not-found
  run kubectl delete job fail-demo -n "$NS" --ignore-not-found
  # verify job: chỉ xoá khi ĐÃ kết thúc; còn Running (chế độ nhanh) thì GIỮ để xem tới Complete
  if kubectl get job "$VJOB" -n "$NS" >/dev/null 2>&1; then
    if kubectl get job "$VJOB" -n "$NS" -o jsonpath='{.status.conditions[*].type}' 2>/dev/null | grep -qE 'Complete|Failed'; then
      run kubectl delete job "$VJOB" -n "$NS" --ignore-not-found
    else
      note "GIỮ Job $VJOB (còn Running — pipeline chạy nền). Tự xoá: kubectl delete job $VJOB -n $NS"
    fi
  fi
  note "Kiểm chứng sạch (rỗng = OK):"
  run kubectl get pods,deploy -n "$NS" -l 'app in (demo,demo-dep,quarantine)'
}

# ---- điều phối --------------------------------------------------------------
case "$ONLY" in
  1.3) lab_1_3 ;;
  1.4) lab_1_4 ;;
  *)   lab_1_3; lab_1_4 ;;
esac
cleanup
printf '\n%s%s✔ XONG Day 1 (Lab %s).%s\n' "$B" "$G" "$ONLY" "$X"
