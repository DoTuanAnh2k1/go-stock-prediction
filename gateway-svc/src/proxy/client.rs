use anyhow::Result;
use axum::{
    body::Body,
    http::{HeaderMap, Method, StatusCode},
    response::Response,
};
use bytes::Bytes;
use reqwest::Client;
use std::time::Duration;
use tracing::{error, info, warn};

use crate::config::AppConfig;

#[derive(Clone)]
pub struct ProxyClient {
    client: Client,
    max_retries: u32,
    retry_delay: Duration,
}

impl ProxyClient {
    pub async fn new(config: &AppConfig) -> Result<Self> {
        info!("Initializing ProxyClient");

        let client = Client::builder()
            .timeout(Duration::from_secs(config.http.request.timeout))
            .pool_max_idle_per_host(config.http.pool.max_idle_per_host)
            .pool_idle_timeout(Duration::from_secs(config.http.pool.idle_timeout))
            .connect_timeout(Duration::from_secs(config.http.pool.connection_timeout))
            .build()?;

        Ok(Self {
            client,
            max_retries: config.http.request.max_retries,
            retry_delay: Duration::from_secs(config.http.request.retry_delay),
        })
    }

    /// Forward a request to `backend` + `path`.
    /// `backend` is e.g. "http://api:8118" (no trailing slash).
    pub async fn forward_request(
        &self,
        method: Method,
        path: &str,
        headers: &HeaderMap,
        body: Body,
        backend: &str,
    ) -> Result<Response, ProxyError> {
        let url = format!("{}{}", backend.trim_end_matches('/'), path);

        let body_bytes = axum::body::to_bytes(body, usize::MAX)
            .await
            .map_err(|e| ProxyError::RequestFailed(format!("Failed to read body: {}", e)))?;

        for attempt in 0..=self.max_retries {
            if attempt > 0 {
                warn!("Retrying request attempt {}/{}", attempt, self.max_retries);
                tokio::time::sleep(self.retry_delay).await;
            }

            match self.do_forward(&method, &url, headers, body_bytes.clone()).await {
                Ok(response) => {
                    info!(status = %response.status(), url = %url, "Forwarded");
                    return Ok(response);
                }
                Err(e) if attempt < self.max_retries => {
                    warn!(error = %e, "Request failed, retrying");
                }
                Err(e) => {
                    error!(error = %e, url = %url, "Request failed after retries");
                    return Err(e);
                }
            }
        }

        Err(ProxyError::RequestFailed("All retries exhausted".to_string()))
    }

    async fn do_forward(
        &self,
        method: &Method,
        url: &str,
        headers: &HeaderMap,
        body_bytes: Bytes,
    ) -> Result<Response, ProxyError> {
        let mut builder = match method.as_str() {
            "GET" => self.client.get(url),
            "POST" => self.client.post(url),
            "PUT" => self.client.put(url),
            "DELETE" => self.client.delete(url),
            "PATCH" => self.client.patch(url),
            "HEAD" => self.client.head(url),
            "OPTIONS" => self.client.request(reqwest::Method::OPTIONS, url),
            _ => {
                return Err(ProxyError::RequestFailed(format!(
                    "Unsupported method: {}",
                    method
                )))
            }
        };

        for (key, value) in headers.iter() {
            let name = key.as_str().to_lowercase();
            if matches!(
                name.as_str(),
                "host" | "connection" | "content-length" | "transfer-encoding"
                // W3C trace headers are (re)written from our own span context
                // below, so drop any inbound copies to avoid duplicates.
                    | "traceparent" | "tracestate"
            ) {
                continue;
            }
            if let Ok(v) = value.to_str() {
                builder = builder.header(key.as_str(), v);
            }
        }

        // EDGE service: inject the current span's W3C trace context so api-svc
        // joins the same trace (gateway → api-svc → auth/prediction).
        for (k, v) in crate::telemetry::traceparent_headers() {
            builder = builder.header(k, v);
        }

        if !body_bytes.is_empty() {
            builder = builder.body(body_bytes.to_vec());
        }

        // SSE connections are long-lived; the client-level timeout would kill
        // them mid-stream, so lift it for requests that ask for an event stream.
        let wants_sse = headers
            .get("accept")
            .and_then(|v| v.to_str().ok())
            .map(|v| v.contains("text/event-stream"))
            .unwrap_or(false);
        if wants_sse {
            builder = builder.timeout(Duration::from_secs(24 * 3600));
        }

        let response = builder
            .send()
            .await
            .map_err(|e| ProxyError::RequestFailed(format!("Request failed: {}", e)))?;

        self.convert_response(response).await
    }

    async fn convert_response(
        &self,
        response: reqwest::Response,
    ) -> Result<Response, ProxyError> {
        let status = response.status();
        let resp_headers = response.headers().clone();

        let is_sse = resp_headers
            .get("content-type")
            .and_then(|v| v.to_str().ok())
            .map(|v| v.starts_with("text/event-stream"))
            .unwrap_or(false);

        let mut builder = Response::builder().status(status);

        for (key, value) in resp_headers.iter() {
            let name = key.as_str().to_lowercase();
            if matches!(name.as_str(), "transfer-encoding" | "content-encoding") {
                continue;
            }
            builder = builder.header(key, value);
        }

        // SSE bodies are unbounded — forward chunks as they arrive instead of
        // buffering to completion (which would stall the stream until timeout).
        let body = if is_sse {
            Body::from_stream(response.bytes_stream())
        } else {
            let body_bytes = response.bytes().await.map_err(|e| {
                ProxyError::RequestFailed(format!("Failed to read response: {}", e))
            })?;
            Body::from(body_bytes)
        };

        builder
            .body(body)
            .map_err(|e| ProxyError::RequestFailed(format!("Failed to build response: {}", e)))
    }
}

#[derive(Debug)]
pub enum ProxyError {
    RequestFailed(String),
}

impl std::fmt::Display for ProxyError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ProxyError::RequestFailed(msg) => write!(f, "Proxy error: {}", msg),
        }
    }
}

impl std::error::Error for ProxyError {}

use axum::response::IntoResponse;

impl IntoResponse for ProxyError {
    fn into_response(self) -> Response {
        let (status, message) = match self {
            ProxyError::RequestFailed(msg) => (StatusCode::BAD_GATEWAY, msg),
        };

        let body = serde_json::json!({ "error": message, "status": status.as_u16() });
        (status, axum::Json(body)).into_response()
    }
}
