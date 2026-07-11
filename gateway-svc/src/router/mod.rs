use crate::bluegreen::state::ResolvedBgApp;
use crate::config::RouteConfig;

pub enum RouteAction {
    Block,
    Proxy {
        backend: String,
        security_headers: bool,
    },
    BlueGreen {
        app_id: usize,
    },
}

pub struct PathRouter {
    // sorted longest prefix first for correct matching
    routes: Vec<(String, RouteAction)>,
}

impl PathRouter {
    pub fn from_config(routes: &[RouteConfig], bg_apps: &[ResolvedBgApp]) -> Self {
        let mut pairs: Vec<(String, RouteAction)> = routes
            .iter()
            .map(|r| {
                let action = if r.action == "block" {
                    RouteAction::Block
                } else {
                    RouteAction::Proxy {
                        backend: r
                            .backend
                            .clone()
                            .expect("proxy route requires backend field"),
                        security_headers: r.security_headers,
                    }
                };
                (r.prefix.clone(), action)
            })
            .collect();

        // Bluegreen-managed prefixes replace any static route with the same prefix
        // (blue/green wins), then take part in the same longest-prefix matching.
        for (i, app) in bg_apps.iter().enumerate() {
            pairs.retain(|(p, _)| p != &app.prefix);
            pairs.push((app.prefix.clone(), RouteAction::BlueGreen { app_id: i }));
        }

        // longest prefix first so /api wins over / for path /api/gold
        pairs.sort_by(|a, b| b.0.len().cmp(&a.0.len()));

        Self { routes: pairs }
    }

    pub fn route(&self, path: &str) -> Option<&RouteAction> {
        self.routes
            .iter()
            .find(|(prefix, _)| path.starts_with(prefix.as_str()))
            .map(|(_, action)| action)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::bluegreen::state::Color;

    fn make_routes(items: &[(&str, &str, Option<&str>, bool)]) -> Vec<RouteConfig> {
        items
            .iter()
            .map(|(prefix, action, backend, sec)| RouteConfig {
                prefix: prefix.to_string(),
                action: action.to_string(),
                backend: backend.map(|s| s.to_string()),
                security_headers: *sec,
            })
            .collect()
    }

    fn bg(prefix: &str) -> ResolvedBgApp {
        ResolvedBgApp {
            name: "x".into(),
            prefix: prefix.into(),
            namespace: "stock".into(),
            blue_dep: "b".into(),
            green_dep: "g".into(),
            blue_backend: "http://b".into(),
            green_backend: "http://g".into(),
            default_active: Color::Green,
        }
    }

    #[test]
    fn test_api_wins_over_root() {
        let router = PathRouter::from_config(
            &make_routes(&[
                ("/", "proxy", Some("http://frontend:3000"), false),
                ("/api", "proxy", Some("http://api:8118"), true),
            ]),
            &[],
        );

        match router.route("/api/gold/latest") {
            Some(RouteAction::Proxy { backend, .. }) => {
                assert_eq!(backend, "http://api:8118");
            }
            _ => panic!("expected proxy to api"),
        }
    }

    #[test]
    fn test_root_catches_frontend() {
        let router = PathRouter::from_config(
            &make_routes(&[
                ("/api", "proxy", Some("http://api:8118"), true),
                ("/", "proxy", Some("http://frontend:3000"), false),
            ]),
            &[],
        );

        match router.route("/dashboard") {
            Some(RouteAction::Proxy { backend, .. }) => {
                assert_eq!(backend, "http://frontend:3000");
            }
            _ => panic!("expected proxy to frontend"),
        }
    }

    #[test]
    fn test_block_returns_block() {
        let router = PathRouter::from_config(
            &make_routes(&[
                ("/swagger", "block", None, false),
                ("/", "proxy", Some("http://frontend:3000"), false),
            ]),
            &[],
        );

        assert!(matches!(
            router.route("/swagger/index.html"),
            Some(RouteAction::Block)
        ));
    }

    #[test]
    fn test_security_headers_flag() {
        let router = PathRouter::from_config(
            &make_routes(&[
                ("/api", "proxy", Some("http://api:8118"), true),
                ("/", "proxy", Some("http://frontend:3000"), false),
            ]),
            &[],
        );

        match router.route("/api/auth/login") {
            Some(RouteAction::Proxy { security_headers, .. }) => assert!(*security_headers),
            _ => panic!("expected proxy"),
        }

        match router.route("/") {
            Some(RouteAction::Proxy { security_headers, .. }) => assert!(!*security_headers),
            _ => panic!("expected proxy"),
        }
    }

    #[test]
    fn test_no_routes_none() {
        let router = PathRouter::from_config(&[], &[]);
        assert!(router.route("/anything").is_none());
    }

    #[test]
    fn bluegreen_prefix_wins_and_indexed() {
        let router = PathRouter::from_config(
            &make_routes(&[("/swagger", "block", None, false)]),
            &[bg("/"), bg("/api")],
        );
        assert!(matches!(
            router.route("/api/gold"),
            Some(RouteAction::BlueGreen { app_id: 1 })
        ));
        assert!(matches!(
            router.route("/dashboard"),
            Some(RouteAction::BlueGreen { app_id: 0 })
        ));
        assert!(matches!(router.route("/swagger/x"), Some(RouteAction::Block)));
    }
}
