use axum::http::{HeaderMap, HeaderValue};

/// Add OWASP security headers to a response.
/// HSTS is always included — browsers ignore it over plain HTTP so it's harmless.
pub fn add_security_headers(headers: &mut HeaderMap) {
    headers.insert(
        "x-frame-options",
        HeaderValue::from_static("DENY"),
    );
    headers.insert(
        "x-content-type-options",
        HeaderValue::from_static("nosniff"),
    );
    headers.insert(
        "referrer-policy",
        HeaderValue::from_static("strict-origin-when-cross-origin"),
    );
    headers.insert(
        "strict-transport-security",
        HeaderValue::from_static("max-age=31536000; includeSubDomains"),
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_adds_required_headers() {
        let mut headers = HeaderMap::new();
        add_security_headers(&mut headers);

        assert_eq!(headers.get("x-frame-options").unwrap(), "DENY");
        assert_eq!(headers.get("x-content-type-options").unwrap(), "nosniff");
        assert!(headers.get("referrer-policy").is_some());
        assert!(headers.get("strict-transport-security").is_some());
    }
}
