use rig::completion::ToolDefinition;
use rig::tool::Tool;
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::error::Error;
use std::fmt;
use tracing::*;

const PORTFOLIO_API_HOST: &str = "https://about.calum.sh";

/// Supported endpoints on the portfolio API.
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum PortfolioEndpoint {
    About,
    Work,
    Projects,
    Contact,
}

impl PortfolioEndpoint {
    fn as_path(&self) -> &'static str {
        match self {
            PortfolioEndpoint::About => "/api/about",
            PortfolioEndpoint::Work => "/api/work",
            PortfolioEndpoint::Projects => "/api/projects",
            PortfolioEndpoint::Contact => "/api/contact",
        }
    }

    fn as_url(&self) -> String {
        format!("{PORTFOLIO_API_HOST}{}", self.as_path())
    }
}

impl fmt::Display for PortfolioEndpoint {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.as_url())
    }
}

/// Arguments for the PortfolioAPISearch tool.
#[derive(Debug, Deserialize, Serialize)]
pub struct PortfolioAPISearchArgs {
    endpoint: PortfolioEndpoint,
}

/// Error type for tool execution failures.
#[derive(Debug)]
pub struct ModelError(String);

impl fmt::Display for ModelError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl Error for ModelError {}

/// Tool for fetching JSON from the portfolio API.
#[derive(Debug, Deserialize, Serialize)]
pub struct PortfolioAPISearch;

impl Tool for PortfolioAPISearch {
    const NAME: &'static str = "web_search";
    type Error = ModelError;
    type Args = PortfolioAPISearchArgs;
    type Output = String;

    async fn definition(&self, _prompt: String) -> ToolDefinition {
        serde_json::from_value(json!({
            "name": Self::NAME,
            "description": "Fetch structured portfolio data from Calum Crawford's portfolio API on about.calum.sh.",
            "parameters": {
                "type": "object",
                "properties": {
                    "endpoint": {
                        "type": "string",
                        "description": "Which portfolio API endpoint to query.",
                        "enum": ["about", "work", "projects", "contact"]
                    }
                },
                "required": ["endpoint"]
            }
        }))
        .unwrap_or_else(|e| {
            error!("Critical error: Failed to create tool definition: {}", e);
            panic!(
                "Invalid static tool definition - this is a programming error: {}",
                e
            );
        })
    }

    async fn call(&self, args: Self::Args) -> Result<Self::Output, Self::Error> {
        let url = args.endpoint.as_url();
        info!("Fetching portfolio API content from: {}", url);

        let response = reqwest::get(&url).await.map_err(|e| {
            error!("Error fetching portfolio API URL {}: {}", url, e);

            let mut source = e.source();
            while let Some(err) = source {
                error!("  caused by: {}", err);
                source = err.source();
            }

            ModelError(format!("Failed to fetch portfolio API endpoint: {}", e))
        })?;

        let response = response.error_for_status().map_err(|e| {
            error!("Portfolio API returned an error for {}: {}", url, e);
            ModelError(format!("Portfolio API request failed: {}", e))
        })?;

        let body = response.text().await.map_err(|e| {
            error!(
                "Error reading portfolio API response body from {}: {}",
                url, e
            );
            ModelError(format!("Failed to read portfolio API response: {}", e))
        })?;

        debug!(
            "Successfully fetched portfolio API response from {} ({} bytes)",
            url,
            body.len()
        );

        Ok(body)
    }
}
