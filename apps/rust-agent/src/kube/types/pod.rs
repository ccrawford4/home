use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct PodMetadata {
    pub name: String,
    pub namespace: String,
    pub uid: String,
    #[serde(rename = "creationTimestamp")]
    pub creation_timestamp: String,
    pub labels: Option<std::collections::HashMap<String, String>>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ContainerSpec {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PodCondition {
    #[serde(rename = "type")]
    pub type_field: String,
    pub status: String,
    #[serde(rename = "lastProbeTime")]
    pub last_probe_time: Option<String>,
    #[serde(rename = "lastTransitionTime")]
    pub last_transition_time: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PodSpecStatus {
    pub phase: String,
    pub conditions: Option<Vec<PodCondition>>,
    #[serde(rename = "startTime")]
    pub start_time: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PodSpec {
    pub containers: Vec<ContainerSpec>,
    #[serde(rename = "nodeName")]
    pub node_name: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Pod {
    pub metadata: PodMetadata,
    pub spec: Option<PodSpec>,
    pub status: Option<PodSpecStatus>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct PodListResponse {
    pub items: Vec<Pod>,
}

impl PodListResponse {
    pub fn as_string(&self) -> String {
        let mut output = String::new();
        output.push_str(&format!("Found {} pods:\n\n", self.items.len()));

        for (idx, pod) in self.items.iter().enumerate() {
            output.push_str(&format!("Pod {}:\n", idx + 1));
            output.push_str(&format!("  Name: {}\n", pod.metadata.name));
            output.push_str(&format!("  Namespace: {}\n", pod.metadata.namespace));
            output.push_str(&format!("  UID: {}\n", pod.metadata.uid));
            output.push_str(&format!("  Created: {}\n", pod.metadata.creation_timestamp));

            if let Some(labels) = &pod.metadata.labels {
                output.push_str("  Labels:\n");
                for (key, value) in labels {
                    output.push_str(&format!("    {}: {}\n", key, value));
                }
            }

            if let Some(spec) = &pod.spec {
                output.push_str(&format!(
                    "  Node: {}\n",
                    spec.node_name.as_deref().unwrap_or("N/A")
                ));
                output.push_str("  Containers:\n");
                for container in &spec.containers {
                    output.push_str(&format!("    - {}\n", container.name));
                }
            }

            if let Some(status) = &pod.status {
                output.push_str(&format!("  Phase: {}\n", status.phase));
                if let Some(start_time) = &status.start_time {
                    output.push_str(&format!("  Started: {}\n", start_time));
                }
                if let Some(conditions) = &status.conditions {
                    output.push_str("  Conditions:\n");
                    for condition in conditions {
                        output.push_str(&format!(
                            "    {}: {}\n",
                            condition.type_field, condition.status
                        ));
                    }
                }
            }

            output.push_str("\n");
        }

        output
    }
}
