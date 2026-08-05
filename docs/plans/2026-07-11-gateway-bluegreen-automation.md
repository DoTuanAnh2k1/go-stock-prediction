# Gateway-native Blue/Green Automation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tự động hoá blue/green cho web-svc + api-svc thẳng trong gateway-svc (Rust): set image màu idle → gateway phát hiện, lật 100% traffic, đo success-rate, tự promote hoặc rollback.

**Architecture:** Thêm module `bluegreen/` vào gateway. `proxy_handler` route theo màu `active` (giữ trong `BlueGreenState`) + ghi metric mỗi request. Một tokio controller (kube-rs) watch Deployment blue/green (edge-triggered), chạy state machine Flip→Analyze→Promote/Rollback, patch image qua k8s API. State persist ở ConfigMap `gateway-bluegreen-state`.

**Tech Stack:** Rust, axum 0.8, reqwest, tokio, serde_yaml, **kube 0.95 + k8s-openapi 0.23** (thêm mới).

## ✅ Trạng thái: ĐÃ HIỆN THỰC + VERIFY (2026-07-11)

Tất cả 10 task xong, 23 unit test pass, tích hợp trên kind ckad:
- **Happy path:** `set image` màu idle → `bluegreen.rollout.start` → `flip` (active đổi màu, traffic 100% sang candidate) → phân tích 60s với tải thật → `promote` (patch màu còn lại = image mới). Xác nhận **không ping-pong** sau promote.
- **Bad deploy:** idle set image lỗi (`web-svc:nope`, ImagePullBackOff) → candidate không Ready → **KHÔNG flip**, active giữ nguyên, `curl /` = 200 (service không gián đoạn).
- **Rollback (fail-rate):** unit-tested trong `decide()` (early rollback khi fail rate vượt ngưỡng).
- 2 bug tìm+fix khi tích hợp: (1) edge bị consume khi candidate còn mid-rollout → mất trigger; (2) promote patch màu còn lại bị tick sau nhận diện là candidate mới → ping-pong. Xem commit `fix(gateway): bluegreen controller edge-consume + ping-pong`.
- Build: Dockerfile `rust:1.90-slim-bookworm` (dep `home@0.5.12` cần rustc 1.88; bookworm glibc khớp runtime). reqwest `default-features=false` (rustls, bỏ openssl thừa).

Giới hạn đã biết (để tương lai): controller loop tuần tự → trong lúc 1 app rollout (60s) các app khác không được tick.

### Log + command tích hợp (kind ckad, 2026-07-11)

**Happy path — web-svc:**
```bash
kubectl set image deploy/web-svc-green web-svc=web-svc:v2 -n stock   # set image màu idle
# controller tự: rollout.start -> flip -> analyze 60s (có tải) -> promote (patch màu còn lại)
#   bluegreen.rollout.start app=web-svc from=green to=blue ... (khi test blue là idle)
#   bluegreen.flip app=web-svc color=blue
#   bluegreen.promote app=web-svc total=... fail=0   -> blue.img tự patch = web-svc:v2
```

**Happy path — api-svc:**
```
bluegreen.rollout.start app=api-svc from="green" to="blue" image=Some("api-svc:v2")
bluegreen.flip         app=api-svc color="blue"
bluegreen.promote      app=api-svc total=1019 fail=0        # sau 60s, 0 lỗi -> promote
```

**Rollback thật — candidate Ready-nhưng-503** (readiness `/healthz`=200, `/`=503):
```bash
kubectl create configmap web-bad-conf -n stock --from-literal=default.conf='server { listen 3000; location = /healthz { return 200 "ok"; } location / { return 503 "bad"; } }'
kubectl patch deploy web-svc-blue -n stock --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"web-svc","image":"nginx:1.27-alpine","readinessProbe":{"httpGet":{"path":"/healthz","port":3000}},"volumeMounts":[{"name":"conf","mountPath":"/etc/nginx/conf.d"}]}],"volumes":[{"name":"conf","configMap":{"name":"web-bad-conf"}}]}}}}'
# controller: flip sang blue -> traffic / trả 503 -> fail rate cao -> EARLY rollback
```
```
bluegreen.rollout.start app=web-svc from="green" to="blue" image=Some("nginx:1.27-alpine")
bluegreen.flip         app=web-svc color="blue"
bluegreen.rollback     app=web-svc total=155 fail=39        # 25% fail > ngưỡng 1% -> lật về green (~6s)
```

**Bad deploy — candidate không Ready** (ImagePullBackOff):
```bash
kubectl set image deploy/web-svc-blue web-svc=web-svc:nope -n stock
# controller phát hiện image đổi nhưng deployment_ready=false -> KHÔNG flip; active giữ nguyên; curl / = 200
```

## Global Constraints

- Rust edition 2021; crate name `gateway`; binary `gateway`.
- KHÔNG canary (lật 100%). Blue/green thuần.
- Metric fail = HTTP status ≥ `failure_status_from` (default 500) HOẶC proxy error. 4xx = success.
- Defaults: `window_seconds=60`, `success_threshold=0.99`, `min_requests=20`, `max_window_multiplier=3`, `failure_status_from=500`.
- Namespace mặc định `stock`. Chỉ 1 rollout/app cùng lúc.
- Log 1 dòng, tiếng Anh, prefix `bluegreen.*` (khớp convention log dự án).
- Spec nguồn: `docs/specs/2026-07-11-gateway-bluegreen-automation-design.md`.

---

## File Structure

| File | Trách nhiệm |
|---|---|
| `gateway-svc/src/bluegreen/mod.rs` | re-export + `init()` (build state, spawn controller nếu enabled) |
| `gateway-svc/src/bluegreen/config.rs` | serde structs khối `bluegreen` |
| `gateway-svc/src/bluegreen/state.rs` | `Color`, `ResolvedBgApp`, `BlueGreenState` (active + counters), `is_failure()` |
| `gateway-svc/src/bluegreen/analysis.rs` | `Verdict`, `decide()` — hàm thuần quyết định |
| `gateway-svc/src/bluegreen/k8s.rs` | kube-rs: đọc/ghi image Deployment, Ready, state ConfigMap |
| `gateway-svc/src/bluegreen/controller.rs` | tokio task: watch + state machine |
| `gateway-svc/src/config/mod.rs` | +field `bluegreen: Option<BlueGreenConfig>` |
| `gateway-svc/src/router/mod.rs` | +`RouteAction::BlueGreen { app_id }` |
| `gateway-svc/src/routes/proxy.rs` | AppState +`bg`; xử lý BlueGreen action + record metric |
| `gateway-svc/src/routes/mod.rs` | truyền `bg` vào AppState |
| `gateway-svc/src/main.rs` | gọi `bluegreen::init()`, spawn controller |
| `deploy/k8s/web-svc/bluegreen.yaml` | 2 Deployment + 2 Service màu |
| `deploy/k8s/api-svc/bluegreen.yaml` | 2 Deployment + 2 Service màu |
| `deploy/k8s/gateway-svc/rbac.yaml` | ServiceAccount + Role + RoleBinding + state ConfigMap |
| `gateway-svc/config.yaml` | +khối `bluegreen` |

---

## Task 1: bluegreen config schema

**Files:**
- Create: `gateway-svc/src/bluegreen/config.rs`
- Modify: `gateway-svc/src/config/mod.rs` (thêm field + `mod bluegreen` không cần — module ở crate root)
- Modify: `gateway-svc/src/main.rs` (thêm `mod bluegreen;`)

**Interfaces:**
- Produces: `BlueGreenConfig { enabled: bool, analysis: AnalysisConfig, apps: Vec<BgAppConfig> }`; `AnalysisConfig { window_seconds: u64, success_threshold: f64, min_requests: u64, max_window_multiplier: u32, failure_status_from: u16 }`; `BgAppConfig { name: String, prefix: String, namespace: String, blue: BgColorConfig, green: BgColorConfig, default_active: String }`; `BgColorConfig { deployment: String, backend: String }`.

- [ ] **Step 1: Add module declaration**

Trong `gateway-svc/src/main.rs`, thêm cạnh các `mod` khác:
```rust
mod bluegreen;
```

- [ ] **Step 2: Write the failing test** (`gateway-svc/src/bluegreen/config.rs`)

```rust
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BlueGreenConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub analysis: AnalysisConfig,
    #[serde(default)]
    pub apps: Vec<BgAppConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AnalysisConfig {
    #[serde(default = "d_window")] pub window_seconds: u64,
    #[serde(default = "d_threshold")] pub success_threshold: f64,
    #[serde(default = "d_min_req")] pub min_requests: u64,
    #[serde(default = "d_max_mult")] pub max_window_multiplier: u32,
    #[serde(default = "d_fail_from")] pub failure_status_from: u16,
}

impl Default for AnalysisConfig {
    fn default() -> Self {
        Self { window_seconds: d_window(), success_threshold: d_threshold(),
            min_requests: d_min_req(), max_window_multiplier: d_max_mult(),
            failure_status_from: d_fail_from() }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BgAppConfig {
    pub name: String,
    pub prefix: String,
    #[serde(default = "d_ns")] pub namespace: String,
    pub blue: BgColorConfig,
    pub green: BgColorConfig,
    #[serde(default = "d_active")] pub default_active: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BgColorConfig {
    pub deployment: String,
    pub backend: String,
}

fn d_window() -> u64 { 60 }
fn d_threshold() -> f64 { 0.99 }
fn d_min_req() -> u64 { 20 }
fn d_max_mult() -> u32 { 3 }
fn d_fail_from() -> u16 { 500 }
fn d_ns() -> String { "stock".to_string() }
fn d_active() -> String { "green".to_string() }

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_minimal_app() {
        let yaml = r#"
enabled: true
analysis: { window_seconds: 30 }
apps:
  - name: web-svc
    prefix: "/"
    namespace: stock
    blue:  { deployment: web-svc-blue,  backend: "http://web-svc-blue:3000" }
    green: { deployment: web-svc-green, backend: "http://web-svc-green:3000" }
    default_active: green
"#;
        let c: BlueGreenConfig = serde_yaml::from_str(yaml).unwrap();
        assert!(c.enabled);
        assert_eq!(c.analysis.window_seconds, 30);
        assert_eq!(c.analysis.success_threshold, 0.99); // default kept
        assert_eq!(c.apps.len(), 1);
        assert_eq!(c.apps[0].blue.deployment, "web-svc-blue");
        assert_eq!(c.apps[0].default_active, "green");
    }
}
```

- [ ] **Step 3: Wire into AppConfig** (`gateway-svc/src/config/mod.rs`)

Thêm field vào struct `AppConfig` (sau `logging`):
```rust
    #[serde(default)]
    pub bluegreen: Option<crate::bluegreen::config::BlueGreenConfig>,
```
Và tạo `gateway-svc/src/bluegreen/mod.rs` tối thiểu để module compile:
```rust
pub mod config;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd gateway-svc && cargo test bluegreen::config`
Expected: PASS (`parses_minimal_app`).

- [ ] **Step 5: Commit**

```bash
git add gateway-svc/src/bluegreen/config.rs gateway-svc/src/bluegreen/mod.rs gateway-svc/src/config/mod.rs gateway-svc/src/main.rs
git commit -m "feat(gateway): bluegreen config schema"
```

---

## Task 2: Color, ResolvedBgApp, BlueGreenState, is_failure

**Files:**
- Create: `gateway-svc/src/bluegreen/state.rs`
- Modify: `gateway-svc/src/bluegreen/mod.rs` (`pub mod state;`)

**Interfaces:**
- Consumes: `BgAppConfig` (Task 1).
- Produces:
  - `enum Color { Blue, Green }` with `fn other(self)->Color`, `fn as_str(&self)->&'static str`, `fn parse(&str)->Color` (default Green on unknown).
  - `struct ResolvedBgApp { name, prefix, namespace, blue_dep, green_dep, blue_backend, green_backend, default_active: Color }`.
  - `struct BlueGreenState` with `fn new(apps: Vec<ResolvedBgApp>) -> Self`, `fn app_count(&self)->usize`, `fn app(&self,id)->&ResolvedBgApp`, `fn active(&self,id)->Color`, `fn set_active(&self,id,Color)`, `fn backend(&self,id)->String` (active color backend), `fn deployment(&self,id,Color)->&str`, `fn record(&self,id,Color,ok:bool)`, `fn reset(&self,id,Color)`, `fn snapshot(&self,id,Color)->(u64,u64)`.
  - `fn is_failure(status: Option<u16>, proxy_error: bool, failure_status_from: u16) -> bool`.

- [ ] **Step 1: Write the failing test** (`gateway-svc/src/bluegreen/state.rs`)

```rust
use std::sync::atomic::{AtomicU8, AtomicU64, Ordering};
use crate::bluegreen::config::BgAppConfig;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Color { Blue, Green }

impl Color {
    pub fn other(self) -> Color { match self { Color::Blue => Color::Green, Color::Green => Color::Blue } }
    pub fn as_str(&self) -> &'static str { match self { Color::Blue => "blue", Color::Green => "green" } }
    pub fn parse(s: &str) -> Color { if s.eq_ignore_ascii_case("blue") { Color::Blue } else { Color::Green } }
    fn idx(self) -> usize { match self { Color::Blue => 0, Color::Green => 1 } }
}

#[derive(Debug, Clone)]
pub struct ResolvedBgApp {
    pub name: String,
    pub prefix: String,
    pub namespace: String,
    pub blue_dep: String,
    pub green_dep: String,
    pub blue_backend: String,
    pub green_backend: String,
    pub default_active: Color,
}

impl ResolvedBgApp {
    pub fn from_cfg(c: &BgAppConfig) -> Self {
        Self {
            name: c.name.clone(), prefix: c.prefix.clone(), namespace: c.namespace.clone(),
            blue_dep: c.blue.deployment.clone(), green_dep: c.green.deployment.clone(),
            blue_backend: c.blue.backend.clone(), green_backend: c.green.backend.clone(),
            default_active: Color::parse(&c.default_active),
        }
    }
    pub fn deployment(&self, color: Color) -> &str {
        match color { Color::Blue => &self.blue_dep, Color::Green => &self.green_dep }
    }
    pub fn backend(&self, color: Color) -> &str {
        match color { Color::Blue => &self.blue_backend, Color::Green => &self.green_backend }
    }
}

struct Counter { total: AtomicU64, fail: AtomicU64 }
impl Counter { fn new() -> Self { Self { total: AtomicU64::new(0), fail: AtomicU64::new(0) } } }

pub struct BlueGreenState {
    apps: Vec<ResolvedBgApp>,
    active: Vec<AtomicU8>,          // 0=blue, 1=green
    counters: Vec<[Counter; 2]>,    // [blue, green]
}

impl BlueGreenState {
    pub fn new(apps: Vec<ResolvedBgApp>) -> Self {
        let active = apps.iter().map(|a| AtomicU8::new(a.default_active.idx() as u8)).collect();
        let counters = apps.iter().map(|_| [Counter::new(), Counter::new()]).collect();
        Self { apps, active, counters }
    }
    pub fn app_count(&self) -> usize { self.apps.len() }
    pub fn app(&self, id: usize) -> &ResolvedBgApp { &self.apps[id] }
    pub fn active(&self, id: usize) -> Color {
        if self.active[id].load(Ordering::Relaxed) == 0 { Color::Blue } else { Color::Green }
    }
    pub fn set_active(&self, id: usize, c: Color) { self.active[id].store(c.idx() as u8, Ordering::Relaxed); }
    pub fn backend(&self, id: usize) -> String { self.apps[id].backend(self.active(id)).to_string() }
    pub fn record(&self, id: usize, c: Color, ok: bool) {
        let ctr = &self.counters[id][c.idx()];
        ctr.total.fetch_add(1, Ordering::Relaxed);
        if !ok { ctr.fail.fetch_add(1, Ordering::Relaxed); }
    }
    pub fn reset(&self, id: usize, c: Color) {
        let ctr = &self.counters[id][c.idx()];
        ctr.total.store(0, Ordering::Relaxed); ctr.fail.store(0, Ordering::Relaxed);
    }
    pub fn snapshot(&self, id: usize, c: Color) -> (u64, u64) {
        let ctr = &self.counters[id][c.idx()];
        (ctr.total.load(Ordering::Relaxed), ctr.fail.load(Ordering::Relaxed))
    }
}

pub fn is_failure(status: Option<u16>, proxy_error: bool, failure_status_from: u16) -> bool {
    if proxy_error { return true; }
    matches!(status, Some(s) if s >= failure_status_from)
}

#[cfg(test)]
mod tests {
    use super::*;
    fn app() -> ResolvedBgApp {
        ResolvedBgApp { name: "web".into(), prefix: "/".into(), namespace: "stock".into(),
            blue_dep: "web-blue".into(), green_dep: "web-green".into(),
            blue_backend: "http://blue:3000".into(), green_backend: "http://green:3000".into(),
            default_active: Color::Green }
    }

    #[test]
    fn default_active_and_flip() {
        let s = BlueGreenState::new(vec![app()]);
        assert_eq!(s.active(0), Color::Green);
        assert_eq!(s.backend(0), "http://green:3000");
        s.set_active(0, Color::Blue);
        assert_eq!(s.active(0), Color::Blue);
        assert_eq!(s.backend(0), "http://blue:3000");
    }

    #[test]
    fn counters_record_and_reset() {
        let s = BlueGreenState::new(vec![app()]);
        s.record(0, Color::Blue, true);
        s.record(0, Color::Blue, false);
        assert_eq!(s.snapshot(0, Color::Blue), (2, 1));
        assert_eq!(s.snapshot(0, Color::Green), (0, 0));
        s.reset(0, Color::Blue);
        assert_eq!(s.snapshot(0, Color::Blue), (0, 0));
    }

    #[test]
    fn failure_classification() {
        assert!(is_failure(Some(500), false, 500));
        assert!(is_failure(Some(503), false, 500));
        assert!(is_failure(None, true, 500));      // proxy error
        assert!(!is_failure(Some(200), false, 500));
        assert!(!is_failure(Some(404), false, 500)); // 4xx = success
    }
}
```

- [ ] **Step 2: Register module** — trong `gateway-svc/src/bluegreen/mod.rs` thêm `pub mod state;`

- [ ] **Step 3: Run tests**

Run: `cd gateway-svc && cargo test bluegreen::state`
Expected: PASS (3 tests).

- [ ] **Step 4: Commit**

```bash
git add gateway-svc/src/bluegreen/state.rs gateway-svc/src/bluegreen/mod.rs
git commit -m "feat(gateway): bluegreen state (active color, counters, failure classify)"
```

---

## Task 3: Verdict decision (pure function)

**Files:**
- Create: `gateway-svc/src/bluegreen/analysis.rs`
- Modify: `gateway-svc/src/bluegreen/mod.rs` (`pub mod analysis;`)

**Interfaces:**
- Consumes: `AnalysisConfig` (Task 1).
- Produces: `enum Verdict { Continue, Promote, Rollback, Extend }`; `fn decide(total: u64, fail: u64, elapsed_secs: u64, extends_used: u32, a: &AnalysisConfig) -> Verdict`.

Logic:
- Rollback SỚM: `total >= min_requests` và `fail as f64 / total as f64 > (1.0 - success_threshold)`.
- Chưa hết window (`elapsed_secs < window_seconds`): `Continue`.
- Hết window, `total >= min_requests`, success ≥ threshold: `Promote`.
- Hết window, `total < min_requests`, còn quota extend (`extends_used < max_window_multiplier - 1`): `Extend`.
- Hết window, `total < min_requests`, hết quota extend: `Promote` (low-confidence — không thấy lỗi).

- [ ] **Step 1: Write the failing test** (`gateway-svc/src/bluegreen/analysis.rs`)

```rust
use crate::bluegreen::config::AnalysisConfig;

#[derive(Debug, PartialEq, Eq)]
pub enum Verdict { Continue, Promote, Rollback, Extend }

pub fn decide(total: u64, fail: u64, elapsed_secs: u64, extends_used: u32, a: &AnalysisConfig) -> Verdict {
    let fail_rate = if total > 0 { fail as f64 / total as f64 } else { 0.0 };
    let max_fail = 1.0 - a.success_threshold;

    // Early rollback: đủ mẫu và fail rate vượt ngưỡng — không đợi hết window.
    if total >= a.min_requests && fail_rate > max_fail {
        return Verdict::Rollback;
    }
    if elapsed_secs < a.window_seconds {
        return Verdict::Continue;
    }
    // Hết window:
    if total >= a.min_requests {
        return Verdict::Promote; // success ≥ threshold (đã loại rollback ở trên)
    }
    // Traffic thấp:
    if extends_used < a.max_window_multiplier.saturating_sub(1) {
        Verdict::Extend
    } else {
        Verdict::Promote // low-confidence, không thấy lỗi
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn cfg() -> AnalysisConfig {
        AnalysisConfig { window_seconds: 60, success_threshold: 0.99, min_requests: 20,
            max_window_multiplier: 3, failure_status_from: 500 }
    }

    #[test]
    fn early_rollback_on_high_fail() {
        // 100 req, 5 fail = 5% > 1% ngưỡng, đủ mẫu, dù chưa hết window
        assert_eq!(decide(100, 5, 10, 0, &cfg()), Verdict::Rollback);
    }
    #[test]
    fn continue_within_window() {
        assert_eq!(decide(100, 0, 10, 0, &cfg()), Verdict::Continue);
    }
    #[test]
    fn promote_after_window_healthy() {
        assert_eq!(decide(100, 0, 60, 0, &cfg()), Verdict::Promote);
        assert_eq!(decide(100, 1, 60, 0, &cfg()), Verdict::Promote); // 1% == ngưỡng, không vượt
    }
    #[test]
    fn extend_when_low_traffic() {
        assert_eq!(decide(5, 0, 60, 0, &cfg()), Verdict::Extend);
    }
    #[test]
    fn low_confidence_promote_after_extends() {
        // hết quota extend (max_mult-1 = 2) mà vẫn ít mẫu, không lỗi
        assert_eq!(decide(5, 0, 60, 2, &cfg()), Verdict::Promote);
    }
    #[test]
    fn rollback_beats_low_traffic() {
        // ít mẫu nhưng fail cao + đủ min? min=20 nên total=5 không rollback → Extend
        assert_eq!(decide(5, 5, 60, 0, &cfg()), Verdict::Extend);
    }
}
```

- [ ] **Step 2: Register module** — `gateway-svc/src/bluegreen/mod.rs` thêm `pub mod analysis;`

- [ ] **Step 3: Run tests**

Run: `cd gateway-svc && cargo test bluegreen::analysis`
Expected: PASS (6 tests).

- [ ] **Step 4: Commit**

```bash
git add gateway-svc/src/bluegreen/analysis.rs gateway-svc/src/bluegreen/mod.rs
git commit -m "feat(gateway): bluegreen verdict decision function"
```

---

## Task 4: Router BlueGreen action

**Files:**
- Modify: `gateway-svc/src/router/mod.rs`

**Interfaces:**
- Consumes: `ResolvedBgApp` (Task 2).
- Produces: `RouteAction::BlueGreen { app_id: usize }`; `PathRouter::from_config(routes: &[RouteConfig], bg_apps: &[ResolvedBgApp]) -> PathRouter`.

- [ ] **Step 1: Modify RouteAction + from_config**

Trong `gateway-svc/src/router/mod.rs`:
```rust
use crate::bluegreen::state::ResolvedBgApp;

pub enum RouteAction {
    Block,
    Proxy { backend: String, security_headers: bool },
    BlueGreen { app_id: usize },
}

impl PathRouter {
    pub fn from_config(routes: &[RouteConfig], bg_apps: &[ResolvedBgApp]) -> Self {
        let mut pairs: Vec<(String, RouteAction)> = routes
            .iter()
            .map(|r| {
                let action = if r.action == "block" {
                    RouteAction::Block
                } else {
                    RouteAction::Proxy {
                        backend: r.backend.clone().expect("proxy route requires backend field"),
                        security_headers: r.security_headers,
                    }
                };
                (r.prefix.clone(), action)
            })
            .collect();

        // Bluegreen-managed prefixes thay thế route tĩnh cùng prefix (bg thắng).
        for (i, app) in bg_apps.iter().enumerate() {
            pairs.retain(|(p, _)| p != &app.prefix);
            pairs.push((app.prefix.clone(), RouteAction::BlueGreen { app_id: i }));
        }

        pairs.sort_by(|a, b| b.0.len().cmp(&a.0.len()));
        Self { routes: pairs }
    }
    // route() giữ nguyên
}
```

- [ ] **Step 2: Write the failing test** (thêm vào `mod tests` sẵn có)

```rust
    fn bg(prefix: &str) -> ResolvedBgApp {
        ResolvedBgApp { name: "x".into(), prefix: prefix.into(), namespace: "stock".into(),
            blue_dep: "b".into(), green_dep: "g".into(),
            blue_backend: "http://b".into(), green_backend: "http://g".into(),
            default_active: crate::bluegreen::state::Color::Green }
    }

    #[test]
    fn bluegreen_prefix_wins_and_indexed() {
        let router = PathRouter::from_config(
            &make_routes(&[("/swagger", "block", None, false)]),
            &[bg("/"), bg("/api")],
        );
        assert!(matches!(router.route("/api/gold"), Some(RouteAction::BlueGreen { app_id: 1 })));
        assert!(matches!(router.route("/dashboard"), Some(RouteAction::BlueGreen { app_id: 0 })));
        assert!(matches!(router.route("/swagger/x"), Some(RouteAction::Block)));
    }
```

- [ ] **Step 3: Fix existing callers** — mọi chỗ gọi `PathRouter::from_config(&config.routes)` giờ cần tham số thứ 2. Trong các test cũ của router truyền `&[]`. (Ví dụ `test_api_wins_over_root` → `PathRouter::from_config(&make_routes(...), &[])`.) Sẽ sửa `main.rs` ở Task 8.

- [ ] **Step 4: Run tests**

Run: `cd gateway-svc && cargo test router`
Expected: PASS (bao gồm test mới + các test cũ đã thêm `&[]`).

- [ ] **Step 5: Commit**

```bash
git add gateway-svc/src/router/mod.rs
git commit -m "feat(gateway): router BlueGreen action + prefix precedence"
```

---

## Task 5: proxy_handler — resolve active backend + record metric

**Files:**
- Modify: `gateway-svc/src/routes/proxy.rs`
- Modify: `gateway-svc/src/routes/mod.rs`

**Interfaces:**
- Consumes: `BlueGreenState` (Task 2), `RouteAction::BlueGreen` (Task 4).
- Produces: `AppState { proxy_client, router, bg: Option<Arc<BlueGreenState>> }`; `create_routes(proxy_client, router, config, bg)`.

- [ ] **Step 1: Extend AppState + handle BlueGreen** (`gateway-svc/src/routes/proxy.rs`)

Thêm field và nhánh xử lý. `forward_request` trả `Result<Response, ProxyError>`; ta phân loại outcome từ status/lỗi rồi `record`.
```rust
use crate::bluegreen::state::{BlueGreenState, is_failure};

pub struct AppState {
    pub proxy_client: Arc<ProxyClient>,
    pub router: Arc<PathRouter>,
    pub bg: Option<Arc<BlueGreenState>>,
}

// trong proxy_handler, thêm nhánh sau Proxy{...}:
        Some(RouteAction::BlueGreen { app_id }) => {
            let bg = match &state.bg { Some(b) => b, None => return StatusCode::BAD_GATEWAY.into_response() };
            let app_id = *app_id;
            let color = bg.active(app_id);
            let backend = bg.backend(app_id);
            let ff = 500u16; // failure_status_from — controller cũng dùng default; đọc từ config ở Task 8 nếu cần khác
            match state.proxy_client
                .forward_request(method, &full_path, &headers, body, &backend).await
            {
                Ok(response) => {
                    let status = response.status().as_u16();
                    bg.record(app_id, color, !is_failure(Some(status), false, ff));
                    response
                }
                Err(e) => {
                    bg.record(app_id, color, !is_failure(None, true, ff));
                    e.into_response()
                }
            }
        }
```
Ghi chú: `failure_status_from` để hằng 500 ở data-path cho đơn giản; controller (Task 7) mới cần đọc từ config để quyết verdict. Nếu muốn cấu hình được cả data-path, truyền `ff` vào `BlueGreenState` lúc `new()` (tùy chọn, không bắt buộc cho MVP).

- [ ] **Step 2: Thread `bg` qua create_routes** (`gateway-svc/src/routes/mod.rs`)

Thêm tham số `bg: Option<Arc<BlueGreenState>>` vào `create_routes(...)`, đưa vào `AppState { proxy_client, router, bg }`.

- [ ] **Step 3: Verify build**

Run: `cd gateway-svc && cargo build`
Expected: lỗi ở `main.rs` (chưa truyền `bg`) — sẽ sửa ở Task 8. `cargo build` phần này chỉ để chắc proxy.rs/mod.rs đúng cú pháp; nếu muốn xanh hoàn toàn, làm Task 8 trước khi build. (Không commit lẻ nếu build đỏ — gộp commit với Task 8, hoặc tạm `create_routes` để `bg=None` ở main.)

- [ ] **Step 4: Commit** (cùng Task 8 nếu cần build xanh)

```bash
git add gateway-svc/src/routes/proxy.rs gateway-svc/src/routes/mod.rs
git commit -m "feat(gateway): proxy_handler bluegreen routing + metric recording"
```

---

## Task 6: kube-rs client helpers + deps

**Files:**
- Modify: `gateway-svc/Cargo.toml`
- Create: `gateway-svc/src/bluegreen/k8s.rs`
- Modify: `gateway-svc/src/bluegreen/mod.rs` (`pub mod k8s;`)

**Interfaces:**
- Produces (async, `anyhow::Result`):
  - `async fn client() -> Result<kube::Client>`
  - `async fn deployment_image(c: &Client, ns: &str, name: &str) -> Result<Option<String>>`
  - `async fn deployment_ready(c: &Client, ns: &str, name: &str) -> Result<bool>`
  - `async fn patch_deployment_image(c: &Client, ns: &str, name: &str, image: &str) -> Result<()>`
  - `async fn read_state(c: &Client, ns: &str) -> HashMap<String, String>`
  - `async fn write_state(c: &Client, ns: &str, key: &str, val: &str) -> Result<()>`
- Const: `STATE_CM: &str = "gateway-bluegreen-state"`.

- [ ] **Step 1: Add deps** (`gateway-svc/Cargo.toml`, mục `[dependencies]`)

```toml
kube = { version = "0.95", features = ["client", "runtime"] }
k8s-openapi = { version = "0.23", features = ["v1_30"] }
```

- [ ] **Step 2: Implement k8s.rs**

```rust
use std::collections::HashMap;
use anyhow::{Context, Result};
use kube::{Client, Api};
use kube::api::{Patch, PatchParams};
use k8s_openapi::api::apps::v1::Deployment;
use k8s_openapi::api::core::v1::ConfigMap;

pub const STATE_CM: &str = "gateway-bluegreen-state";

pub async fn client() -> Result<Client> {
    Client::try_default().await.context("k8s client")
}

pub async fn deployment_image(c: &Client, ns: &str, name: &str) -> Result<Option<String>> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let d = api.get(name).await.with_context(|| format!("get deploy {name}"))?;
    let img = d.spec.and_then(|s| s.template.spec)
        .and_then(|ps| ps.containers.into_iter().next())
        .and_then(|c| c.image);
    Ok(img)
}

pub async fn deployment_ready(c: &Client, ns: &str, name: &str) -> Result<bool> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let d = api.get(name).await?;
    let status = d.status.unwrap_or_default();
    let desired = d.spec.and_then(|s| s.replicas).unwrap_or(1);
    let available = status.available_replicas.unwrap_or(0);
    let updated = status.updated_replicas.unwrap_or(0);
    Ok(available >= desired && updated >= desired)
}

pub async fn patch_deployment_image(c: &Client, ns: &str, name: &str, image: &str) -> Result<()> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    // Container name = deployment name không chắc; patch container[0].image bằng strategic merge
    // theo tên container. Ta giả định container name khớp app color deployment's container.
    // An toàn hơn: strategic merge theo container name lấy từ get(). Ở đây patch mảng theo index-0.
    let patch = serde_json::json!({
        "spec": { "template": { "spec": { "containers": [ { "name": container_name(c, ns, name).await?, "image": image } ] } } }
    });
    api.patch(name, &PatchParams::apply("gateway-bluegreen").force(), &Patch::Apply(patch)).await
        .with_context(|| format!("patch image {name}"))?;
    Ok(())
}

async fn container_name(c: &Client, ns: &str, name: &str) -> Result<String> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let d = api.get(name).await?;
    let cn = d.spec.and_then(|s| s.template.spec)
        .and_then(|ps| ps.containers.into_iter().next())
        .map(|c| c.name).unwrap_or_else(|| name.to_string());
    Ok(cn)
}

pub async fn read_state(c: &Client, ns: &str) -> HashMap<String, String> {
    let api: Api<ConfigMap> = Api::namespaced(c.clone(), ns);
    match api.get(STATE_CM).await {
        Ok(cm) => cm.data.unwrap_or_default(),
        Err(_) => HashMap::new(),
    }
}

pub async fn write_state(c: &Client, ns: &str, key: &str, val: &str) -> Result<()> {
    let api: Api<ConfigMap> = Api::namespaced(c.clone(), ns);
    let patch = serde_json::json!({
        "apiVersion": "v1", "kind": "ConfigMap",
        "metadata": { "name": STATE_CM },
        "data": { key: val }
    });
    api.patch(STATE_CM, &PatchParams::apply("gateway-bluegreen").force(), &Patch::Apply(patch)).await
        .with_context(|| "patch state cm")?;
    Ok(())
}
```
> Lưu ý thực thi: kube 0.95 API surface có thể cần chỉnh nhỏ (import, `PatchParams::apply`). `cargo build` sẽ chỉ ra; sửa tại chỗ. `patch_deployment_image` dùng server-side apply với field manager riêng — an toàn, không đụng field khác.

- [ ] **Step 3: Register module + build**

`gateway-svc/src/bluegreen/mod.rs` thêm `pub mod k8s;`
Run: `cd gateway-svc && cargo build`
Expected: build xanh (module k8s compile). Sửa import theo lỗi cargo nếu có.

- [ ] **Step 4: Commit**

```bash
git add gateway-svc/Cargo.toml gateway-svc/Cargo.lock gateway-svc/src/bluegreen/k8s.rs gateway-svc/src/bluegreen/mod.rs
git commit -m "feat(gateway): kube-rs client helpers for bluegreen"
```

---

## Task 7: Rollout controller (state machine)

**Files:**
- Create: `gateway-svc/src/bluegreen/controller.rs`
- Modify: `gateway-svc/src/bluegreen/mod.rs` (`pub mod controller;` + `init()`)

**Interfaces:**
- Consumes: `BlueGreenState` (T2), `decide/Verdict` (T3), `k8s` helpers (T6), `BlueGreenConfig` (T1).
- Produces: `async fn run(client, state: Arc<BlueGreenState>, cfg: Arc<BlueGreenConfig>)`; `fn init(cfg, ...) -> Option<(Arc<BlueGreenState>, impl FnOnce future)>` (chi tiết ở mod.rs).

- [ ] **Step 1: Implement controller loop** (`gateway-svc/src/bluegreen/controller.rs`)

Thuật toán (poll-based, đơn giản & đủ; watch nâng cao để tương lai):
```rust
use std::sync::Arc;
use std::time::{Duration, Instant};
use std::collections::HashMap;
use tracing::{info, warn, error};
use kube::Client;
use crate::bluegreen::config::BlueGreenConfig;
use crate::bluegreen::state::{BlueGreenState, Color};
use crate::bluegreen::analysis::{decide, Verdict};
use crate::bluegreen::k8s;

pub async fn run(client: Client, state: Arc<BlueGreenState>, cfg: Arc<BlueGreenConfig>) {
    // Seed active từ ConfigMap state (nếu có) — override default_active.
    for id in 0..state.app_count() {
        let ns = state.app(id).namespace.clone();
        let saved = k8s::read_state(&client, &ns).await;
        if let Some(v) = saved.get(&state.app(id).name) {
            state.set_active(id, Color::parse(v));
        }
    }
    // last_seen image per (app, color) — seed để edge-trigger không kích giả khi boot.
    let mut last_seen: HashMap<(usize, Color), Option<String>> = HashMap::new();
    for id in 0..state.app_count() {
        let ns = &state.app(id).namespace;
        for color in [Color::Blue, Color::Green] {
            let dep = state.app(id).deployment(color).to_string();
            last_seen.insert((id, color), k8s::deployment_image(&client, ns, &dep).await.ok().flatten());
        }
        // Boot inconsistency check
        let a = state.active(id);
        let ai = last_seen.get(&(id, a)).cloned().flatten();
        let ii = last_seen.get(&(id, a.other())).cloned().flatten();
        if ai.is_some() && ai != ii {
            warn!(app = %state.app(id).name, "bluegreen.inconsistent_on_boot");
        }
    }

    let poll = Duration::from_secs(2);
    loop {
        for id in 0..state.app_count() {
            if let Err(e) = tick(&client, &state, &cfg, id, &mut last_seen).await {
                warn!(app = %state.app(id).name, error = %e, "bluegreen.tick_error");
            }
        }
        tokio::time::sleep(poll).await;
    }
}

// Trạng thái rollout đang chạy giữ trong closure-local map (theo id). Đơn giản hoá:
// dùng static per-loop struct. Ở đây minh hoạ theo id qua HashMap ngoài vòng.
async fn tick(
    client: &Client, state: &Arc<BlueGreenState>, cfg: &Arc<BlueGreenConfig>,
    id: usize, last_seen: &mut HashMap<(usize, Color), Option<String>>,
) -> anyhow::Result<()> {
    let app = state.app(id).clone();
    let ns = &app.namespace;
    let active = state.active(id);
    let idle = active.other();
    let idle_dep = app.deployment(idle).to_string();

    let cur_idle_img = k8s::deployment_image(client, ns, &idle_dep).await?;
    let prev = last_seen.get(&(id, idle)).cloned().flatten();

    // Edge-trigger: idle image vừa đổi (khác last_seen) và idle Ready → rollout.
    if cur_idle_img != prev {
        last_seen.insert((id, idle), cur_idle_img.clone());
        if cur_idle_img.is_some() && k8s::deployment_ready(client, ns, &idle_dep).await? {
            info!(app = %app.name, from = active.as_str(), to = idle.as_str(),
                image = ?cur_idle_img, "bluegreen.rollout.start");
            do_rollout(client, state, cfg, id, active, idle).await?;
        }
        return Ok(());
    }
    // Guard: active image đổi ngoài luồng
    let cur_active_img = k8s::deployment_image(client, ns, app.deployment(active)).await?;
    let prev_active = last_seen.get(&(id, active)).cloned().flatten();
    if cur_active_img != prev_active {
        last_seen.insert((id, active), cur_active_img);
        warn!(app = %app.name, "bluegreen.out_of_band_change");
    }
    Ok(())
}

async fn do_rollout(
    client: &Client, state: &Arc<BlueGreenState>, cfg: &Arc<BlueGreenConfig>,
    id: usize, old: Color, candidate: Color,
) -> anyhow::Result<()> {
    let app = state.app(id).clone();
    let a = &cfg.analysis;
    // Flip 100% sang candidate + reset counter + persist.
    state.reset(id, candidate);
    state.set_active(id, candidate);
    let _ = k8s::write_state(client, &app.namespace, &app.name, candidate.as_str()).await;
    info!(app = %app.name, color = candidate.as_str(), "bluegreen.flip");

    // Analyze loop
    let start = Instant::now();
    let mut extends: u32 = 0;
    loop {
        tokio::time::sleep(Duration::from_secs(2)).await;
        let (total, fail) = state.snapshot(id, candidate);
        let elapsed = start.elapsed().as_secs().saturating_sub(extends as u64 * a.window_seconds);
        match decide(total, fail, elapsed, extends, a) {
            Verdict::Continue => continue,
            Verdict::Extend => { extends += 1; info!(app=%app.name, "bluegreen.extend"); continue; }
            Verdict::Rollback => {
                state.set_active(id, old);
                let _ = k8s::write_state(client, &app.namespace, &app.name, old.as_str()).await;
                warn!(app=%app.name, total, fail, "bluegreen.rollback");
                return Ok(());
            }
            Verdict::Promote => {
                // patch màu cũ (giờ idle) = image của candidate
                let img = k8s::deployment_image(client, &app.namespace, app.deployment(candidate)).await?;
                if let Some(img) = img {
                    k8s::patch_deployment_image(client, &app.namespace, app.deployment(old), &img).await?;
                }
                info!(app=%app.name, total, fail, "bluegreen.promote");
                return Ok(());
            }
        }
    }
}
```
> Ghi chú thực thi: `elapsed` trừ phần extend để `decide` so với `window_seconds` mỗi vòng gia hạn. `state` cần `Clone` cho `ResolvedBgApp` (đã `#[derive(Clone)]`). Nếu borrow-checker phàn nàn, clone các String cần thiết. `cargo build` sẽ chỉ ra.

- [ ] **Step 2: `init()` trong mod.rs**

```rust
// gateway-svc/src/bluegreen/mod.rs
pub mod config; pub mod state; pub mod analysis; pub mod k8s; pub mod controller;
use std::sync::Arc;
use crate::bluegreen::config::BlueGreenConfig;
use crate::bluegreen::state::{BlueGreenState, ResolvedBgApp};

/// Trả (state, apps) nếu bật; None nếu tắt/không cấu hình.
pub fn build_state(cfg: &Option<BlueGreenConfig>) -> Option<(Arc<BlueGreenState>, Vec<ResolvedBgApp>)> {
    let c = cfg.as_ref()?;
    if !c.enabled || c.apps.is_empty() { return None; }
    let apps: Vec<ResolvedBgApp> = c.apps.iter().map(ResolvedBgApp::from_cfg).collect();
    let state = Arc::new(BlueGreenState::new(apps.clone()));
    Some((state, apps))
}
```

- [ ] **Step 3: Build**

Run: `cd gateway-svc && cargo build`
Expected: build xanh (trừ `main.rs` chưa gọi — Task 8). Sửa lỗi mượn/type theo cargo.

- [ ] **Step 4: Commit**

```bash
git add gateway-svc/src/bluegreen/controller.rs gateway-svc/src/bluegreen/mod.rs
git commit -m "feat(gateway): bluegreen rollout controller (flip/analyze/promote/rollback)"
```

---

## Task 8: Wire into main.rs + config.yaml

**Files:**
- Modify: `gateway-svc/src/main.rs`
- Modify: `gateway-svc/config.yaml`
- Modify: `gateway-svc/src/routes/mod.rs` (create_routes signature — đã ở T5)

**Interfaces:**
- Consumes: `bluegreen::build_state` (T7), `controller::run` (T7), `PathRouter::from_config(routes, bg_apps)` (T4), `create_routes(..., bg)` (T5).

- [ ] **Step 1: main.rs — build state, router, spawn controller**

Sau khi có `config: Arc<AppConfig>`:
```rust
    // Blue/green
    let (bg_state, bg_apps) = match bluegreen::build_state(&config.bluegreen) {
        Some((s, a)) => (Some(s), a),
        None => (None, Vec::new()),
    };
    let path_router = Arc::new(PathRouter::from_config(&config.routes, &bg_apps));

    if let (Some(state), Some(bgcfg)) = (bg_state.clone(), config.bluegreen.clone()) {
        match bluegreen::k8s::client().await {
            Ok(k) => {
                let st = state.clone();
                let cfg = Arc::new(bgcfg);
                tokio::spawn(async move { bluegreen::controller::run(k, st, cfg).await; });
                info!("bluegreen controller started ({} app(s))", state.app_count());
            }
            Err(e) => error!("bluegreen: k8s client init failed, controller disabled: {}", e),
        }
    }
```
Đổi `create_routes(proxy_client, path_router, Arc::clone(&config))` → `create_routes(proxy_client, path_router, Arc::clone(&config), bg_state)`.

- [ ] **Step 2: config.yaml — thêm khối bluegreen**

```yaml
bluegreen:
  enabled: true
  analysis: { window_seconds: 60, success_threshold: 0.99, min_requests: 20, max_window_multiplier: 3, failure_status_from: 500 }
  apps:
    - name: web-svc
      prefix: "/"
      namespace: stock
      blue:  { deployment: web-svc-blue,  backend: "http://web-svc-blue:3000" }
      green: { deployment: web-svc-green, backend: "http://web-svc-green:3000" }
      default_active: green
    - name: api-svc
      prefix: "/api"
      namespace: stock
      blue:  { deployment: api-svc-blue,  backend: "http://api-svc-blue:8118" }
      green: { deployment: api-svc-green, backend: "http://api-svc-green:8118" }
      default_active: green
```
Và XÓA (hoặc giữ) route tĩnh `/` và `/api` trong `routes:` — bluegreen sẽ đè, nhưng gọn nhất là bỏ 2 route đó khỏi `routes:` (giữ `/swagger` block, `/health`).

- [ ] **Step 3: Build + all unit tests**

Run: `cd gateway-svc && cargo build && cargo test`
Expected: build xanh; tất cả unit test PASS.

- [ ] **Step 4: Commit**

```bash
git add gateway-svc/src/main.rs gateway-svc/config.yaml gateway-svc/src/routes/mod.rs gateway-svc/src/routes/proxy.rs
git commit -m "feat(gateway): wire bluegreen controller + config into gateway"
```

---

## Task 9: k8s manifests (blue/green + RBAC)

**Files:**
- Create: `deploy/k8s/web-svc/bluegreen.yaml`
- Create: `deploy/k8s/api-svc/bluegreen.yaml`
- Create: `deploy/k8s/gateway-svc/rbac.yaml`
- Modify: `deploy/k8s/gateway-svc/deployment.yaml` (thêm `serviceAccountName: gateway-svc`)

- [ ] **Step 1: web-svc blue/green** (`deploy/k8s/web-svc/bluegreen.yaml`)

2 Deployment (`web-svc-blue` image web-svc:dev, `web-svc-green` image web-svc:v2) + 2 Service (`web-svc-blue`/`web-svc-green` :3000), mỗi Deployment `replicas: 2`, selector `{app: web-svc, color: <c>}`, readinessProbe `/ :3000`, preStop sleep 5. (Tái dùng cấu trúc `day_2/web-bluegreen.yaml` đã có — bổ sung 2 Service theo màu.)

- [ ] **Step 2: api-svc blue/green** (`deploy/k8s/api-svc/bluegreen.yaml`)

Tương tự: `api-svc-blue`/`api-svc-green` (image api-svc:dev) + Service :8118, selector `{app: api-svc, color: <c>}`, envFrom api-config + api-secret, readinessProbe `/health/ready`, preStop.

- [ ] **Step 3: RBAC + state ConfigMap** (`deploy/k8s/gateway-svc/rbac.yaml`)

```yaml
apiVersion: v1
kind: ServiceAccount
metadata: { name: gateway-svc, namespace: stock }
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: { name: gateway-bluegreen, namespace: stock }
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "patch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "create", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: { name: gateway-bluegreen, namespace: stock }
subjects: [ { kind: ServiceAccount, name: gateway-svc, namespace: stock } ]
roleRef: { kind: Role, name: gateway-bluegreen, apiGroup: rbac.authorization.k8s.io }
---
apiVersion: v1
kind: ConfigMap
metadata: { name: gateway-bluegreen-state, namespace: stock }
data: { web-svc: green, api-svc: green }
```

- [ ] **Step 4: gateway deployment SA** — thêm `serviceAccountName: gateway-svc` vào `deploy/k8s/gateway-svc/deployment.yaml` (spec.template.spec).

- [ ] **Step 5: Validate**

Run: `for f in deploy/k8s/web-svc/bluegreen.yaml deploy/k8s/api-svc/bluegreen.yaml deploy/k8s/gateway-svc/rbac.yaml deploy/k8s/gateway-svc/deployment.yaml; do kubectl apply --dry-run=client -f $f >/dev/null && echo "OK $f"; done`
Expected: OK cả 4.

- [ ] **Step 6: Commit**

```bash
git add deploy/k8s/web-svc/bluegreen.yaml deploy/k8s/api-svc/bluegreen.yaml deploy/k8s/gateway-svc/rbac.yaml deploy/k8s/gateway-svc/deployment.yaml
git commit -m "feat(k8s): bluegreen deployments/services + gateway RBAC"
```

---

## Task 10: Build image + integration test on cluster ckad

**Files:** none (verification).

- [ ] **Step 1: Build + load gateway image**

```bash
cd /home/chronical/Projects/private/go-stock-prediction
docker build -t gateway-svc:dev -f deploy/gateway-svc.Dockerfile gateway-svc
kind load docker-image gateway-svc:dev --name ckad
```
Expected: image build xanh (cargo build trong Docker pass — nếu lỗi kube-rs, sửa Task 6/7).

- [ ] **Step 2: Apply RBAC + blue/green + state, restart gateway**

```bash
kubectl apply -f deploy/k8s/gateway-svc/rbac.yaml
kubectl apply -f deploy/k8s/web-svc/bluegreen.yaml
kubectl apply -f deploy/k8s/api-svc/bluegreen.yaml
kubectl apply -f deploy/k8s/gateway-svc/deployment.yaml
kubectl rollout status deploy/gateway-svc -n stock
kubectl logs -n stock deploy/gateway-svc | grep bluegreen
```
Expected: log `bluegreen controller started (2 app(s))`.

- [ ] **Step 3: Trigger rollout (happy path)**

```bash
# đổi image màu idle (green) của web-svc -> bản mới
kubectl set image deploy/web-svc-green web-svc=web-svc:v2 -n stock
# quan sát controller: detect -> flip -> analyzing -> promote
kubectl logs -n stock deploy/gateway-svc -f | grep -E 'bluegreen.(rollout|flip|promote|rollback)'
```
Expected (trong ~60-90s): `rollout.start` → `flip` → `promote`. Sau promote, cả 2 màu web-svc cùng image; `gateway-bluegreen-state` cm `web-svc` = màu mới active.

- [ ] **Step 4: Trigger rollback (bad image)**

```bash
kubectl set image deploy/web-svc-<idle-color> web-svc=web-svc:nope -n stock
```
Expected: idle không Ready (ImagePullBackOff) → controller KHÔNG flip (guard Ready). Hoặc dùng image chạy được nhưng trả 500 để thấy `bluegreen.rollback`. Ghi lại kết quả vào `deploy/k8s/ckad-labs/day_2/lab.md` + index.html (mục blue/green automation).

- [ ] **Step 5: Commit kết quả doc**

```bash
git add deploy/k8s/ckad-labs/
git commit -m "docs(ckad): gateway bluegreen automation integration results"
```

---

## Notes cho người thực thi

- **kube-rs**: API surface 0.95 có thể lệch nhẹ so với code mẫu (import, `Patch::Apply`, `PatchParams::apply`). `cargo build` là nguồn sự thật — sửa tại chỗ, giữ nguyên interface đã khai báo ở Interfaces.
- **Rust borrow/clone**: `ResolvedBgApp` đã `Clone`; nếu controller vướng borrow, clone String cần thiết trước await.
- **api-svc idle chạy backup scheduler**: chấp nhận (insert-if-not-exists, cron 3AM). Tối ưu (disable scheduler ở idle) để sau.
- **Không xoá Service `web-svc`/`api-svc` đơn** cho tới khi chắc không còn ai tham chiếu (gateway giờ trỏ Service theo màu). Kiểm tra trước khi dọn.
