use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct ErrorResponse {
    pub error: String,
    pub status: u16,
}

impl ErrorResponse {
    pub fn new(error: impl Into<String>, status: u16) -> Self {
        Self {
            error: error.into(),
            status,
        }
    }
}
