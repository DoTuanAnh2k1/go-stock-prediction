pub mod analysis;
pub mod config;
pub mod controller;
pub mod k8s;
pub mod state;

use crate::bluegreen::config::BlueGreenConfig;
use crate::bluegreen::state::{BlueGreenState, ResolvedBgApp};
use std::sync::Arc;

/// Build the shared bluegreen state + resolved apps from config, or None if
/// disabled / no apps configured.
pub fn build_state(
    cfg: &Option<BlueGreenConfig>,
) -> Option<(Arc<BlueGreenState>, Vec<ResolvedBgApp>)> {
    let c = cfg.as_ref()?;
    if !c.enabled || c.apps.is_empty() {
        return None;
    }
    let apps: Vec<ResolvedBgApp> = c.apps.iter().map(ResolvedBgApp::from_cfg).collect();
    let state = Arc::new(BlueGreenState::new(apps.clone()));
    Some((state, apps))
}
