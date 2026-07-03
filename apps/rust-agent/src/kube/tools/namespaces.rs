use crate::kube::types::NamespaceListResponse;
use crate::kube::{KubeAgent, KubeAgentError};
use rig::completion::ToolDefinition;
use rig::tool::Tool;
use serde::{Deserialize, Serialize};
use serde_json::json;
use tracing::*;

/// Tool for listing all namespaces in a Kubernetes cluster.
pub struct ListNamespacesTool {
    kube_agent: KubeAgent,
}

impl ListNamespacesTool {
    pub fn new(kube_agent: KubeAgent) -> Self {
        ListNamespacesTool { kube_agent }
    }

    pub async fn list_namespaces(&self) -> Result<String, KubeAgentError> {
        let endpoint = String::from("/api/v1/namespaces");
        let response = self.kube_agent.make_request(endpoint).await?;

        debug!("Kubernetes API response: {}", response);

        let namespace_list: NamespaceListResponse =
            serde_json::from_str(&response).map_err(|e| {
                error!("Error parsing JSON response: {}", e);
                KubeAgentError::from(e)
            })?;

        Ok(namespace_list.as_string())
    }
}

#[derive(Serialize, Deserialize)]
pub struct ListNamespacesToolArgs {}

impl Tool for ListNamespacesTool {
    const NAME: &'static str = "list_namespaces";
    type Args = ListNamespacesToolArgs;
    type Output = String;
    type Error = KubeAgentError;

    async fn definition(&self, _prompt: String) -> ToolDefinition {
        serde_json::from_value(json!({
            "name": Self::NAME,
            "description": "List all namespaces in the Kubernetes cluster.",
            "parameters": {
                "type": "object",
                "properties": {},
                "required": []
            }
        }))
        .unwrap()
    }

    async fn call(&self, _args: Self::Args) -> Result<Self::Output, Self::Error> {
        self.list_namespaces().await
    }
}
