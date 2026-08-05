use config::{Config, ConfigError, File};
use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AppConfig {
    pub server: ServerConfig,
    pub http: HttpConfig,
    pub routes: Vec<RouteConfig>,
    pub middleware: MiddlewareConfig,
    pub logging: LoggingConfig,
    #[serde(default)]
    pub bluegreen: Option<crate::bluegreen::config::BlueGreenConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    pub host: String,
    #[serde(default = "default_http_port")]
    pub http_port: u16,
    #[serde(default = "default_https_port")]
    pub https_port: u16,
    pub tls: TlsConfig,
    #[serde(default = "default_shutdown_timeout")]
    pub shutdown_timeout: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TlsConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default = "default_cert_path")]
    pub cert_path: String,
    #[serde(default = "default_key_path")]
    pub key_path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RouteConfig {
    pub prefix: String,
    /// "proxy" or "block"
    #[serde(default = "default_action")]
    pub action: String,
    #[serde(default)]
    pub backend: Option<String>,
    #[serde(default)]
    pub security_headers: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HttpConfig {
    #[serde(default = "default_http_version")]
    pub version: String,
    pub pool: ConnectionPoolConfig,
    pub request: RequestConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConnectionPoolConfig {
    #[serde(default = "default_max_idle_per_host")]
    pub max_idle_per_host: usize,
    #[serde(default = "default_idle_timeout")]
    pub idle_timeout: u64,
    #[serde(default = "default_connection_timeout")]
    pub connection_timeout: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RequestConfig {
    #[serde(default = "default_timeout")]
    pub timeout: u64,
    #[serde(default)]
    pub max_retries: u32,
    #[serde(default = "default_retry_delay")]
    pub retry_delay: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MiddlewareConfig {
    pub rate_limit: RateLimitConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RateLimitConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default = "default_rate_limit")]
    pub max_requests: u32,
    #[serde(default = "default_rate_window")]
    pub window_secs: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoggingConfig {
    #[serde(default = "default_log_level")]
    pub level: String,
    #[serde(default = "default_log_format")]
    pub format: String,
    #[serde(default = "default_true")]
    pub access_log: bool,
}

fn default_http_port() -> u16 { 80 }
fn default_https_port() -> u16 { 443 }
fn default_shutdown_timeout() -> u64 { 30 }
fn default_cert_path() -> String { "/etc/gateway/certs/cert.pem".to_string() }
fn default_key_path() -> String { "/etc/gateway/certs/key.pem".to_string() }
fn default_action() -> String { "proxy".to_string() }
fn default_http_version() -> String { "auto".to_string() }
fn default_max_idle_per_host() -> usize { 20 }
fn default_idle_timeout() -> u64 { 90 }
fn default_connection_timeout() -> u64 { 10 }
fn default_timeout() -> u64 { 600 }
fn default_retry_delay() -> u64 { 1 }
fn default_rate_limit() -> u32 { 1000 }
fn default_rate_window() -> u64 { 60 }
fn default_log_level() -> String { "info".to_string() }
fn default_log_format() -> String { "json".to_string() }
fn default_true() -> bool { true }

impl AppConfig {
    pub fn load() -> Result<Self, ConfigError> {
        let paths = vec![
            "config.yaml",
            "/etc/gateway/config.yaml",
        ];
        for path in paths {
            if Path::new(path).exists() {
                return Self::load_from(path);
            }
        }
        Err(ConfigError::Message(
            "config.yaml not found. Tried: config.yaml, /etc/gateway/config.yaml".to_string()
        ))
    }

    pub fn load_from<P: AsRef<Path>>(path: P) -> Result<Self, ConfigError> {
        Config::builder()
            .add_source(File::with_name(path.as_ref().to_str().unwrap()))
            .add_source(config::Environment::with_prefix("GATEWAY").separator("_"))
            .build()?
            .try_deserialize()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_route_config_defaults() {
        let r = RouteConfig {
            prefix: "/api".to_string(),
            action: "proxy".to_string(),
            backend: Some("http://api:8118".to_string()),
            security_headers: false,
        };
        assert_eq!(r.prefix, "/api");
        assert!(!r.security_headers);
    }

    #[test]
    fn test_tls_config_defaults() {
        let tls = TlsConfig {
            enabled: false,
            cert_path: "/etc/gateway/certs/cert.pem".to_string(),
            key_path: "/etc/gateway/certs/key.pem".to_string(),
        };
        assert!(!tls.enabled);
    }
}
