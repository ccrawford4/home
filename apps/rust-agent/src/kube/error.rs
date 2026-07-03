use std::fmt;

/// Errors that can occur when interacting with the Kubernetes API.
#[derive(Debug)]
pub enum KubeAgentError {
    /// HTTP request failure (network, timeout, etc.)
    HttpError(reqwest::Error),
    /// Failed to parse JSON response from Kubernetes API
    JsonParseError(serde_json::Error),
    /// General parsing or data validation error
    ParseError(String),
}

impl fmt::Display for KubeAgentError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            KubeAgentError::HttpError(err) => write!(f, "HTTP request error: {}", err),
            KubeAgentError::JsonParseError(err) => write!(f, "JSON parsing error: {}", err),
            KubeAgentError::ParseError(err) => write!(f, "Parse error: {}", err),
        }
    }
}

impl std::error::Error for KubeAgentError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            KubeAgentError::HttpError(err) => Some(err),
            KubeAgentError::JsonParseError(err) => Some(err),
            KubeAgentError::ParseError(_) => None,
        }
    }
}

impl From<reqwest::Error> for KubeAgentError {
    fn from(err: reqwest::Error) -> Self {
        KubeAgentError::HttpError(err)
    }
}

impl From<serde_json::Error> for KubeAgentError {
    fn from(err: serde_json::Error) -> Self {
        KubeAgentError::JsonParseError(err)
    }
}
