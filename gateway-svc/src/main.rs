use std::net::SocketAddr;
use std::sync::Arc;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

mod config;
mod middleware;
mod models;
mod proxy;
mod router;
mod routes;

use config::AppConfig;
use proxy::ProxyClient;
use router::PathRouter;
use routes::create_routes;

#[tokio::main]
async fn main() {
    init_tracing();

    info!("Starting go-stock-prediction gateway");

    let config = match AppConfig::load() {
        Ok(c) => Arc::new(c),
        Err(e) => {
            error!("Failed to load config: {}", e);
            std::process::exit(1);
        }
    };

    let path_router = Arc::new(PathRouter::from_config(&config.routes));
    info!("Loaded {} route(s)", config.routes.len());

    let proxy_client = match ProxyClient::new(&config).await {
        Ok(c) => Arc::new(c),
        Err(e) => {
            error!("Failed to create proxy client: {}", e);
            std::process::exit(1);
        }
    };

    let app = create_routes(proxy_client, path_router, Arc::clone(&config));

    let http_addr: SocketAddr = format!("{}:{}", config.server.host, config.server.http_port)
        .parse()
        .expect("Invalid HTTP address");

    info!("HTTP  → http://{}", http_addr);
    info!("Health → http://{}/healthz", http_addr);

    if config.server.tls.enabled {
        use axum_server::tls_rustls::RustlsConfig;

        let https_addr: SocketAddr =
            format!("{}:{}", config.server.host, config.server.https_port)
                .parse()
                .expect("Invalid HTTPS address");

        let tls_config = match RustlsConfig::from_pem_file(
            &config.server.tls.cert_path,
            &config.server.tls.key_path,
        )
        .await
        {
            Ok(c) => c,
            Err(e) => {
                error!(
                    "Failed to load TLS certs ({}, {}): {}",
                    config.server.tls.cert_path, config.server.tls.key_path, e
                );
                std::process::exit(1);
            }
        };

        info!("HTTPS → https://{}", https_addr);
        info!("Gateway ready");

        let http_future =
            axum_server::bind(http_addr).serve(app.clone().into_make_service());
        let https_future =
            axum_server::bind_rustls(https_addr, tls_config).serve(app.into_make_service());

        let (http_result, https_result) = tokio::join!(http_future, https_future);
        if let Err(e) = http_result {
            error!("HTTP server error: {}", e);
        }
        if let Err(e) = https_result {
            error!("HTTPS server error: {}", e);
        }
    } else {
        let listener = match tokio::net::TcpListener::bind(http_addr).await {
            Ok(l) => l,
            Err(e) => {
                error!("Failed to bind {}: {}", http_addr, e);
                std::process::exit(1);
            }
        };
        info!("Gateway ready (HTTP only — TLS disabled)");
        if let Err(e) = axum::serve(listener, app).await {
            error!("Server error: {}", e);
        }
    }
}

fn init_tracing() {
    let log_level =
        std::env::var("RUST_LOG").unwrap_or_else(|_| "info,gateway=debug".to_string());
    let format = std::env::var("LOG_FORMAT").unwrap_or_else(|_| "json".to_string());

    if format == "json" {
        tracing_subscriber::registry()
            .with(tracing_subscriber::EnvFilter::new(log_level))
            .with(tracing_subscriber::fmt::layer().json())
            .init();
    } else {
        tracing_subscriber::registry()
            .with(tracing_subscriber::EnvFilter::new(log_level))
            .with(tracing_subscriber::fmt::layer().pretty())
            .init();
    }
}
