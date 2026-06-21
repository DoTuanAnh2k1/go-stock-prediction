//! service-mgt registry client for the gateway.
//!
//! Two responsibilities, both best-effort and gated by `SERVICE_MGT_ENABLED`:
//!   * register `gateway-svc` and keep its lease alive (so peers can discover it);
//!   * resolve backend hosts at startup so the proxy targets registry-provided
//!     endpoints, falling back to the static config backend on any failure.
//!
//! The request hot-path is never touched: discovery happens once at boot to
//! pick backends, so a dead registry can never add latency or break proxying.

use std::time::Duration;

use tracing::{info, warn};

pub mod pb {
    tonic::include_proto!("registry");
}

use pb::registry_client::RegistryClient;
use pb::{DiscoverRequest, HeartbeatRequest, RegisterRequest};

/// Resolve `service` to "host:port" via the registry. None on any error or when
/// no UP instance exists (caller then keeps its static backend).
pub async fn discover(target: &str, service: &str) -> Option<String> {
    let mut client = RegistryClient::connect(format!("http://{target}")).await.ok()?;
    let resp = client
        .discover(DiscoverRequest { service_name: service.to_string() })
        .await
        .ok()?;
    let inst = resp.into_inner().instances.into_iter().next()?;
    Some(format!("{}:{}", inst.address, inst.port))
}

/// Spawn a background task that registers `gateway-svc` and heartbeats every
/// 10s, re-registering if the lease is lost. Never blocks startup.
pub fn spawn_register(target: String, service: String, address: String, port: i32) {
    tokio::spawn(async move {
        let mut instance_id: Option<String> = None;
        loop {
            match RegistryClient::connect(format!("http://{target}")).await {
                Ok(mut client) => {
                    if instance_id.is_none() {
                        match client
                            .register(RegisterRequest {
                                service_name: service.clone(),
                                instance_id: String::new(),
                                address: address.clone(),
                                port,
                                metadata: Default::default(),
                                ttl_seconds: 30,
                            })
                            .await
                        {
                            Ok(r) => {
                                instance_id = Some(r.into_inner().instance_id);
                                info!("registered gateway-svc with service-mgt");
                            }
                            Err(e) => warn!("register failed: {}", e),
                        }
                    }
                    if let Some(id) = instance_id.clone() {
                        if let Err(e) =
                            client.heartbeat(HeartbeatRequest { instance_id: id }).await
                        {
                            if e.code() == tonic::Code::NotFound {
                                instance_id = None; // re-register next tick
                            }
                            warn!("heartbeat failed: {}", e);
                        }
                    }
                }
                Err(e) => warn!("registry connect failed: {}", e),
            }
            tokio::time::sleep(Duration::from_secs(10)).await;
        }
    });
}

/// Extract the host from a backend URL like "http://api-svc:8118" → "api-svc".
pub fn host_of(backend: &str) -> Option<String> {
    let no_scheme = backend
        .trim_start_matches("http://")
        .trim_start_matches("https://");
    let host = no_scheme.split(['/', ':']).next()?;
    if host.is_empty() {
        None
    } else {
        Some(host.to_string())
    }
}
