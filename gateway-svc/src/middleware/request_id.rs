use axum::{body::Body, extract::Request, http::HeaderValue, middleware::Next, response::Response};
use uuid::Uuid;

pub const REQUEST_ID_HEADER: &str = "x-request-id";

pub struct RequestIdLayer;

pub async fn request_id_middleware(mut request: Request, next: Next) -> Response {
    let request_id = request
        .headers()
        .get(REQUEST_ID_HEADER)
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string())
        .unwrap_or_else(|| Uuid::new_v4().to_string());

    request.extensions_mut().insert(request_id.clone());

    let mut response = next.run(request).await;

    if let Ok(header_value) = HeaderValue::from_str(&request_id) {
        response
            .headers_mut()
            .insert(REQUEST_ID_HEADER, header_value);
    }

    response
}

pub trait RequestIdExt {
    fn request_id(&self) -> Option<String>;
}

impl RequestIdExt for Request {
    fn request_id(&self) -> Option<String> {
        self.extensions().get::<String>().cloned()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::{body::Body, http::Request};

    #[test]
    fn test_request_id_ext() {
        let mut request = Request::new(Body::empty());
        let test_id = "test-id-123".to_string();
        request.extensions_mut().insert(test_id.clone());
        assert_eq!(request.request_id(), Some(test_id));
    }
}
