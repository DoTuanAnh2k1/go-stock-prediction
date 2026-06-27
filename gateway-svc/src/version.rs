use chrono::Local;
use serde_json::json;
use std::fs;
use std::sync::OnceLock;
use tracing::{info, warn};

static VERSION: OnceLock<VersionInfo> = OnceLock::new();

/// Global accessor for the stamped version info (falls back to env if stamp() not yet called).
pub fn current() -> VersionInfo {
    VERSION.get().cloned().unwrap_or_else(VersionInfo::from_env)
}

/// Build/version info read from runtime environment variables.
/// The values are injected into the final runtime stage of the Docker image
/// (GIT_SHA, BUILD_TIME, GIT_DIRTY) and read at RUNTIME via std::env::var.
#[derive(Debug, Clone)]
pub struct VersionInfo {
    pub git_sha: String,
    pub build_time: String,
    pub dirty: String,
}

impl VersionInfo {
    pub fn from_env() -> Self {
        VersionInfo {
            git_sha: env_or_unknown("GIT_SHA"),
            build_time: env_or_unknown("BUILD_TIME"),
            dirty: env_or_unknown("GIT_DIRTY"),
        }
    }
}

fn env_or_unknown(key: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| "unknown".to_string())
}

/// Read version info from env, write the version JSON file, and log one startup line.
/// Returns the VersionInfo so callers (e.g. /healthz) can reuse the values.
pub fn stamp() -> VersionInfo {
    let info = VersionInfo::from_env();

    // (b) exactly one startup line.
    info!(
        "version git_sha={} build_time={} dirty={}",
        info.git_sha, info.build_time, info.dirty
    );

    // (a) write JSON to /versions/gateway-svc.json — never panic on failure.
    let started_at = Local::now().to_rfc3339();
    let payload = json!({
        "service": "gateway-svc",
        "git_sha": info.git_sha,
        "build_time": info.build_time,
        "dirty": info.dirty,
        "started_at": started_at,
    });

    if let Err(e) = fs::create_dir_all("/versions") {
        warn!("could not create /versions dir: {}", e);
    } else if let Err(e) = fs::write("/versions/gateway-svc.json", payload.to_string()) {
        warn!("could not write /versions/gateway-svc.json: {}", e);
    }

    let _ = VERSION.set(info.clone());
    info
}
