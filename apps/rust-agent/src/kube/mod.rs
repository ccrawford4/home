pub mod error;
pub mod tools;
pub mod types;

pub use error::KubeAgentError;
pub use tools::{ListNamespacesTool, ListPodsTool, NodeMetricsTool};

use tracing::*;

/// Client for interacting with the Kubernetes API.
///
/// Handles authentication via bearer tokens and optional CA certificate validation.
/// Supports both production (with certificates) and development (self-signed certs) modes.
#[derive(Clone)]
pub struct KubeAgent {
    kube_api_server: String,
    token: String,
    certificate: Option<reqwest::Certificate>,
}

impl KubeAgent {
    pub fn new(
        kube_api_server: String,
        token: String,
        certificate: Option<reqwest::Certificate>,
    ) -> Self {
        KubeAgent {
            kube_api_server,
            token,
            certificate,
        }
    }

    /// Makes an HTTP GET request to a Kubernetes API endpoint.
    ///
    /// Automatically handles bearer token authentication and certificate validation.
    /// In development mode (no certificate), accepts self-signed certificates.
    ///
    /// # Arguments
    /// * `endpoint` - The API endpoint path (e.g., "/api/v1/pods")
    ///
    /// # Returns
    /// The response body as a string, or a KubeAgentError on failure.
    pub async fn make_request(&self, endpoint: String) -> Result<String, KubeAgentError> {
        debug!(
            "Making Kubernetes API request to {}{}",
            self.kube_api_server, endpoint
        );

        let mut headers = reqwest::header::HeaderMap::new();
        headers.insert(
            "Authorization",
            format!("Bearer {}", self.token).parse().unwrap(),
        );

        // Build HTTP client with appropriate certificate handling
        let client = if let Some(cert) = &self.certificate {
            debug!("Using CA certificate for secure connection");
            reqwest::Client::builder()
                .default_headers(headers)
                .add_root_certificate(cert.clone())
                .build()
                .unwrap()
        } else {
            warn!(
                "No CA certificate provided, accepting self-signed certificates (development only)"
            );
            reqwest::Client::builder()
                .default_headers(headers)
                .danger_accept_invalid_certs(true)
                .build()
                .unwrap()
        };

        let request = client
            .get(format!("{}{}", self.kube_api_server, endpoint))
            .send()
            .await;

        match request {
            Ok(resp) => {
                let text = resp.text().await;
                match text {
                    Ok(body) => {
                        debug!("Successfully received response from Kubernetes API");
                        Ok(body)
                    }
                    Err(err) => {
                        error!("Failed to read response body: {}", err);
                        Err(KubeAgentError::from(err))
                    }
                }
            }
            Err(err) => {
                error!("Failed to send request to Kubernetes API: {}", err);
                Err(KubeAgentError::from(err))
            }
        }
    }
}
