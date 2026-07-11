use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BlueGreenConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub analysis: AnalysisConfig,
    #[serde(default)]
    pub apps: Vec<BgAppConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AnalysisConfig {
    #[serde(default = "d_window")]
    pub window_seconds: u64,
    #[serde(default = "d_threshold")]
    pub success_threshold: f64,
    #[serde(default = "d_min_req")]
    pub min_requests: u64,
    #[serde(default = "d_max_mult")]
    pub max_window_multiplier: u32,
    #[serde(default = "d_fail_from")]
    pub failure_status_from: u16,
}

impl Default for AnalysisConfig {
    fn default() -> Self {
        Self {
            window_seconds: d_window(),
            success_threshold: d_threshold(),
            min_requests: d_min_req(),
            max_window_multiplier: d_max_mult(),
            failure_status_from: d_fail_from(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BgAppConfig {
    pub name: String,
    pub prefix: String,
    #[serde(default = "d_ns")]
    pub namespace: String,
    pub blue: BgColorConfig,
    pub green: BgColorConfig,
    #[serde(default = "d_active")]
    pub default_active: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BgColorConfig {
    pub deployment: String,
    pub backend: String,
}

fn d_window() -> u64 { 60 }
fn d_threshold() -> f64 { 0.99 }
fn d_min_req() -> u64 { 20 }
fn d_max_mult() -> u32 { 3 }
fn d_fail_from() -> u16 { 500 }
fn d_ns() -> String { "stock".to_string() }
fn d_active() -> String { "green".to_string() }

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_minimal_app() {
        let yaml = r#"
enabled: true
analysis: { window_seconds: 30 }
apps:
  - name: web-svc
    prefix: "/"
    namespace: stock
    blue:  { deployment: web-svc-blue,  backend: "http://web-svc-blue:3000" }
    green: { deployment: web-svc-green, backend: "http://web-svc-green:3000" }
    default_active: green
"#;
        let c: BlueGreenConfig = serde_yaml::from_str(yaml).unwrap();
        assert!(c.enabled);
        assert_eq!(c.analysis.window_seconds, 30);
        assert_eq!(c.analysis.success_threshold, 0.99); // default kept
        assert_eq!(c.apps.len(), 1);
        assert_eq!(c.apps[0].blue.deployment, "web-svc-blue");
        assert_eq!(c.apps[0].default_active, "green");
    }
}
