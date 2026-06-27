use axum::{http::StatusCode, response::IntoResponse, Json};
use serde_json::json;

pub async fn health_check() -> impl IntoResponse {
    let v = crate::version::current();
    (
        StatusCode::OK,
        Json(json!({
            "status": "ok",
            "service": "gateway-svc",
            "git_sha": v.git_sha,
            "build_time": v.build_time,
            "dirty": v.dirty,
        })),
    )
}

pub async fn readiness_check() -> impl IntoResponse {
    (
        StatusCode::OK,
        Json(json!({ "status": "ready", "service": "gateway" })),
    )
}
