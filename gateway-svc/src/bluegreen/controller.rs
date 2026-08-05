use crate::bluegreen::analysis::{decide, Verdict};
use crate::bluegreen::config::BlueGreenConfig;
use crate::bluegreen::k8s;
use crate::bluegreen::state::{BlueGreenState, Color};
use kube::Client;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tracing::{info, warn};

/// Poll interval for detecting deployment image changes and readiness.
const POLL: Duration = Duration::from_secs(2);

/// Controller entry point: seed state, then poll each managed app forever.
pub async fn run(client: Client, state: Arc<BlueGreenState>, cfg: Arc<BlueGreenConfig>) {
    info!("bluegreen.controller.seed.begin");
    // Seed active color from the state ConfigMap (overrides default_active).
    for id in 0..state.app_count() {
        let ns = state.app(id).namespace.clone();
        let name = state.app(id).name.clone();
        let saved = k8s::read_state(&client, &ns).await;
        if let Some(v) = saved.get(&name) {
            state.set_active(id, Color::parse(v));
        }
    }

    // Seed last_seen images so a restart does not spuriously trigger a rollout.
    let mut last_seen: HashMap<(usize, Color), Option<String>> = HashMap::new();
    for id in 0..state.app_count() {
        let app = state.app(id).clone();
        for color in [Color::Blue, Color::Green] {
            let img = match k8s::deployment_image(&client, &app.namespace, app.deployment(color)).await
            {
                Ok(v) => v,
                Err(e) => {
                    warn!(app = %app.name, color = color.as_str(), error = %e, "bluegreen.seed.image_error");
                    None
                }
            };
            last_seen.insert((id, color), img);
        }
        // Boot inconsistency: active image differs from idle image => a rollout was
        // likely interrupted. Leave as-is (no loop), just warn.
        let a = state.active(id);
        let ai = last_seen.get(&(id, a)).cloned().flatten();
        let ii = last_seen.get(&(id, a.other())).cloned().flatten();
        if ai.is_some() && ai != ii {
            warn!(app = %app.name, "bluegreen.inconsistent_on_boot");
        }
    }

    for id in 0..state.app_count() {
        info!(app = %state.app(id).name, active = state.active(id).as_str(),
            blue = ?last_seen.get(&(id, Color::Blue)).cloned().flatten(),
            green = ?last_seen.get(&(id, Color::Green)).cloned().flatten(),
            "bluegreen.controller.ready");
    }

    loop {
        for id in 0..state.app_count() {
            if let Err(e) = tick(&client, &state, &cfg, id, &mut last_seen).await {
                warn!(app = %state.app(id).name, error = %e, "bluegreen.tick_error");
            }
        }
        tokio::time::sleep(POLL).await;
    }
}

/// One poll iteration for a single app: detect an image change on the idle color
/// (edge-triggered) and, if the idle deployment is ready, run a rollout.
async fn tick(
    client: &Client,
    state: &Arc<BlueGreenState>,
    cfg: &Arc<BlueGreenConfig>,
    id: usize,
    last_seen: &mut HashMap<(usize, Color), Option<String>>,
) -> anyhow::Result<()> {
    let app = state.app(id).clone();
    let ns = &app.namespace;
    let active = state.active(id);
    let idle = active.other();

    let cur_idle = k8s::deployment_image(client, ns, app.deployment(idle)).await?;
    let prev_idle = last_seen.get(&(id, idle)).cloned().flatten();

    if cur_idle != prev_idle {
        // Image changed on the idle color. Only CONSUME the edge (update last_seen)
        // once the idle deployment is Ready with the new image — otherwise we'd lose
        // the trigger while it's still mid-rollout (not yet Ready) and never fire.
        if cur_idle.is_some() && k8s::deployment_ready(client, ns, app.deployment(idle)).await? {
            last_seen.insert((id, idle), cur_idle.clone());
            info!(app = %app.name, from = active.as_str(), to = idle.as_str(),
                image = ?cur_idle, "bluegreen.rollout.start");
            do_rollout(client, state, cfg, id, active, idle, last_seen).await?;
        }
        // Not ready yet: leave last_seen unchanged; re-check next tick.
        return Ok(());
    }

    // Guard: image of the active color changed out of band (not a rollout).
    let cur_active = k8s::deployment_image(client, ns, app.deployment(active)).await?;
    let prev_active = last_seen.get(&(id, active)).cloned().flatten();
    if cur_active != prev_active {
        last_seen.insert((id, active), cur_active);
        warn!(app = %app.name, "bluegreen.out_of_band_change");
    }
    Ok(())
}

/// Flip traffic to `candidate`, watch its metrics, then promote or roll back.
async fn do_rollout(
    client: &Client,
    state: &Arc<BlueGreenState>,
    cfg: &Arc<BlueGreenConfig>,
    id: usize,
    old: Color,
    candidate: Color,
    last_seen: &mut HashMap<(usize, Color), Option<String>>,
) -> anyhow::Result<()> {
    let app = state.app(id).clone();
    let a = &cfg.analysis;

    // Flip 100% to candidate, reset its counters, persist.
    state.reset(id, candidate);
    state.set_active(id, candidate);
    let _ = k8s::write_state(client, &app.namespace, &app.name, candidate.as_str()).await;
    info!(app = %app.name, color = candidate.as_str(), "bluegreen.flip");

    let mut extends: u32 = 0;
    let mut window_start = Instant::now();
    loop {
        tokio::time::sleep(POLL).await;
        let (total, fail) = state.snapshot(id, candidate);
        let elapsed = window_start.elapsed().as_secs();
        match decide(total, fail, elapsed, extends, a) {
            Verdict::Continue => continue,
            Verdict::Extend => {
                extends += 1;
                window_start = Instant::now();
                info!(app = %app.name, total, "bluegreen.extend");
                continue;
            }
            Verdict::Rollback => {
                state.set_active(id, old);
                let _ = k8s::write_state(client, &app.namespace, &app.name, old.as_str()).await;
                warn!(app = %app.name, total, fail, "bluegreen.rollback");
                return Ok(());
            }
            Verdict::Promote => {
                // Sync the now-idle (old) color to the candidate's image. Also update
                // last_seen[old] so this self-inflicted patch is NOT re-detected as a
                // new candidate next tick (which would ping-pong the rollout).
                if let Some(img) =
                    k8s::deployment_image(client, &app.namespace, app.deployment(candidate)).await?
                {
                    k8s::patch_deployment_image(client, &app.namespace, app.deployment(old), &img)
                        .await?;
                    last_seen.insert((id, old), Some(img));
                }
                info!(app = %app.name, total, fail, "bluegreen.promote");
                return Ok(());
            }
        }
    }
}
