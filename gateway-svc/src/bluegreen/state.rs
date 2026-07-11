use crate::bluegreen::config::BgAppConfig;
use std::sync::atomic::{AtomicU64, AtomicU8, Ordering};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Color {
    Blue,
    Green,
}

impl Color {
    pub fn other(self) -> Color {
        match self {
            Color::Blue => Color::Green,
            Color::Green => Color::Blue,
        }
    }
    pub fn as_str(&self) -> &'static str {
        match self {
            Color::Blue => "blue",
            Color::Green => "green",
        }
    }
    pub fn parse(s: &str) -> Color {
        if s.eq_ignore_ascii_case("blue") {
            Color::Blue
        } else {
            Color::Green
        }
    }
    fn idx(self) -> usize {
        match self {
            Color::Blue => 0,
            Color::Green => 1,
        }
    }
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
            name: c.name.clone(),
            prefix: c.prefix.clone(),
            namespace: c.namespace.clone(),
            blue_dep: c.blue.deployment.clone(),
            green_dep: c.green.deployment.clone(),
            blue_backend: c.blue.backend.clone(),
            green_backend: c.green.backend.clone(),
            default_active: Color::parse(&c.default_active),
        }
    }
    pub fn deployment(&self, color: Color) -> &str {
        match color {
            Color::Blue => &self.blue_dep,
            Color::Green => &self.green_dep,
        }
    }
    pub fn backend(&self, color: Color) -> &str {
        match color {
            Color::Blue => &self.blue_backend,
            Color::Green => &self.green_backend,
        }
    }
}

struct Counter {
    total: AtomicU64,
    fail: AtomicU64,
}
impl Counter {
    fn new() -> Self {
        Self {
            total: AtomicU64::new(0),
            fail: AtomicU64::new(0),
        }
    }
}

pub struct BlueGreenState {
    apps: Vec<ResolvedBgApp>,
    active: Vec<AtomicU8>, // 0=blue, 1=green
    counters: Vec<[Counter; 2]>, // [blue, green]
}

impl BlueGreenState {
    pub fn new(apps: Vec<ResolvedBgApp>) -> Self {
        let active = apps
            .iter()
            .map(|a| AtomicU8::new(a.default_active.idx() as u8))
            .collect();
        let counters = apps.iter().map(|_| [Counter::new(), Counter::new()]).collect();
        Self { apps, active, counters }
    }
    pub fn app_count(&self) -> usize {
        self.apps.len()
    }
    pub fn app(&self, id: usize) -> &ResolvedBgApp {
        &self.apps[id]
    }
    pub fn active(&self, id: usize) -> Color {
        if self.active[id].load(Ordering::Relaxed) == 0 {
            Color::Blue
        } else {
            Color::Green
        }
    }
    pub fn set_active(&self, id: usize, c: Color) {
        self.active[id].store(c.idx() as u8, Ordering::Relaxed);
    }
    pub fn backend(&self, id: usize) -> String {
        self.apps[id].backend(self.active(id)).to_string()
    }
    pub fn record(&self, id: usize, c: Color, ok: bool) {
        let ctr = &self.counters[id][c.idx()];
        ctr.total.fetch_add(1, Ordering::Relaxed);
        if !ok {
            ctr.fail.fetch_add(1, Ordering::Relaxed);
        }
    }
    pub fn reset(&self, id: usize, c: Color) {
        let ctr = &self.counters[id][c.idx()];
        ctr.total.store(0, Ordering::Relaxed);
        ctr.fail.store(0, Ordering::Relaxed);
    }
    pub fn snapshot(&self, id: usize, c: Color) -> (u64, u64) {
        let ctr = &self.counters[id][c.idx()];
        (
            ctr.total.load(Ordering::Relaxed),
            ctr.fail.load(Ordering::Relaxed),
        )
    }
}

pub fn is_failure(status: Option<u16>, proxy_error: bool, failure_status_from: u16) -> bool {
    if proxy_error {
        return true;
    }
    matches!(status, Some(s) if s >= failure_status_from)
}

#[cfg(test)]
mod tests {
    use super::*;
    fn app() -> ResolvedBgApp {
        ResolvedBgApp {
            name: "web".into(),
            prefix: "/".into(),
            namespace: "stock".into(),
            blue_dep: "web-blue".into(),
            green_dep: "web-green".into(),
            blue_backend: "http://blue:3000".into(),
            green_backend: "http://green:3000".into(),
            default_active: Color::Green,
        }
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
        assert!(is_failure(None, true, 500)); // proxy error
        assert!(!is_failure(Some(200), false, 500));
        assert!(!is_failure(Some(404), false, 500)); // 4xx = success
    }
}
