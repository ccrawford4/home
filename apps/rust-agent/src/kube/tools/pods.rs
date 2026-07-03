use crate::kube::error::KubeAgentError;
use crate::kube::types::PodListResponse;
use crate::kube::KubeAgent;
use rig::completion::ToolDefinition;
use rig::tool::Tool;
use serde::{Deserialize, Serialize};
use serde_json::json;
use tracing::*;

/// Tool for listing pods in a Kubernetes cluster namespace.
///
/// Supports filtering by namespace and limiting the number of results.
pub struct ListPodsTool {
    kube_agent: KubeAgent,
}

impl ListPodsTool {
    pub fn new(kube_agent: KubeAgent) -> Self {
        ListPodsTool { kube_agent }
    }

    pub async fn list_pods(
        &self,
        namespace: Option<String>,
        limit: Option<u32>,
    ) -> Result<String, KubeAgentError> {
        let mut namespace_path = String::from("default");
        let mut limit_query: u32 = 500;

        if let Some(ns) = namespace {
            namespace_path = ns;
        }
        if let Some(lim) = limit {
            limit_query = lim;
        }

        let endpoint = format!(
            "/api/v1/namespaces/{}/pods?limit={}",
            namespace_path, limit_query
        );

        let response = self.kube_agent.make_request(endpoint).await?;

        debug!("Kubernetes API response: {}", response);

        let pod_list: PodListResponse = serde_json::from_str(&response).map_err(|e| {
            error!("Error parsing JSON response: {}", e);
            KubeAgentError::from(e)
        })?;

        Ok(pod_list.as_string())
    }
}

#[derive(Serialize, Deserialize)]
pub struct ListPodsToolArgs {
    pub namespace: Option<String>,
    pub limit: Option<u32>,
}

impl Tool for ListPodsTool {
    const NAME: &'static str = "list_pods";
    type Args = ListPodsToolArgs;
    type Output = String;
    type Error = KubeAgentError;

    async fn definition(&self, _prompt: String) -> ToolDefinition {
        serde_json::from_value(json!({
            "name": "list_pods",
            "description": "List pods in a Kubernetes cluster namespace",
            "parameters": {
                "type": "object",
                "properties": {
                    "namespace": {
                        "type": "string",
                        "description": "The namespace to list pods from (default is 'default')"
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of pods to return (default is 500)"
                    }
                },
                "required": []
            }
        }))
        .unwrap()
    }

    async fn call(&self, args: Self::Args) -> Result<Self::Output, Self::Error> {
        self.list_pods(args.namespace, args.limit).await
    }
}
