use anyhow::{Context, Result};
use k8s_openapi::api::apps::v1::Deployment;
use k8s_openapi::api::core::v1::ConfigMap;
use kube::api::{Patch, PatchParams};
use kube::{Api, Client};
use std::collections::BTreeMap;

/// Name of the ConfigMap that persists which color is active per app.
pub const STATE_CM: &str = "gateway-bluegreen-state";

pub async fn client() -> Result<Client> {
    Client::try_default().await.context("k8s client")
}

/// Current image of the first container of a Deployment (None if not found / no image).
pub async fn deployment_image(c: &Client, ns: &str, name: &str) -> Result<Option<String>> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let d = api
        .get(name)
        .await
        .with_context(|| format!("get deployment {name}"))?;
    let img = d
        .spec
        .and_then(|s| s.template.spec)
        .and_then(|ps| ps.containers.into_iter().next())
        .and_then(|ct| ct.image);
    Ok(img)
}

/// True when the Deployment has all desired replicas available and updated.
pub async fn deployment_ready(c: &Client, ns: &str, name: &str) -> Result<bool> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let d = api.get(name).await?;
    let desired = d.spec.as_ref().and_then(|s| s.replicas).unwrap_or(1);
    let status = d.status.unwrap_or_default();
    let available = status.available_replicas.unwrap_or(0);
    let updated = status.updated_replicas.unwrap_or(0);
    Ok(available >= desired && updated >= desired)
}

/// Name of the first container in a Deployment (fallback: deployment name).
async fn container_name(c: &Client, ns: &str, name: &str) -> Result<String> {
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let d = api.get(name).await?;
    let cn = d
        .spec
        .and_then(|s| s.template.spec)
        .and_then(|ps| ps.containers.into_iter().next())
        .map(|ct| ct.name)
        .unwrap_or_else(|| name.to_string());
    Ok(cn)
}

/// Set the image of a Deployment's first container (strategic merge, like `kubectl set image`).
pub async fn patch_deployment_image(c: &Client, ns: &str, name: &str, image: &str) -> Result<()> {
    let cn = container_name(c, ns, name).await?;
    let api: Api<Deployment> = Api::namespaced(c.clone(), ns);
    let patch = serde_json::json!({
        "spec": { "template": { "spec": { "containers": [ { "name": cn, "image": image } ] } } }
    });
    api.patch(name, &PatchParams::default(), &Patch::Strategic(patch))
        .await
        .with_context(|| format!("patch image {name}"))?;
    Ok(())
}

/// Read the state ConfigMap's data (empty map if absent).
pub async fn read_state(c: &Client, ns: &str) -> BTreeMap<String, String> {
    let api: Api<ConfigMap> = Api::namespaced(c.clone(), ns);
    match api.get(STATE_CM).await {
        Ok(cm) => cm.data.unwrap_or_default(),
        Err(_) => BTreeMap::new(),
    }
}

/// Set data[key]=val in the state ConfigMap (JSON merge patch; CM must exist).
pub async fn write_state(c: &Client, ns: &str, key: &str, val: &str) -> Result<()> {
    let api: Api<ConfigMap> = Api::namespaced(c.clone(), ns);
    let patch = serde_json::json!({ "data": { key: val } });
    api.patch(STATE_CM, &PatchParams::default(), &Patch::Merge(patch))
        .await
        .context("patch state configmap")?;
    Ok(())
}
