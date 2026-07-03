use crate::kube::error::KubeAgentError;
use serde::{Deserialize, Serialize};

// Node API Response (/api/v1/nodes)
#[derive(Debug, Serialize, Deserialize)]
pub struct NodeListResponse {
    pub items: Vec<Node>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Node {
    pub metadata: NodeMetadata,
    pub status: NodeStatus,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NodeMetadata {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NodeStatus {
    pub capacity: NodeCapacity,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NodeCapacity {
    pub cpu: String,
    pub memory: String,
}

// NodeMetrics API Response (/apis/metrics.k8s.io/v1beta1/nodes)
#[derive(Debug, Serialize, Deserialize)]
pub struct NodeMetricsListResponse {
    pub items: Vec<NodeMetrics>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NodeMetrics {
    pub metadata: NodeMetricsMetadata,
    pub usage: NodeUsage,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NodeMetricsMetadata {
    pub name: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct NodeUsage {
    pub cpu: String,    // e.g., "160635734n" (nanoseconds)
    pub memory: String, // e.g., "1879200Ki"
}

// Combined struct with calculated percentages
#[derive(Debug, Serialize, Deserialize)]
pub struct NodeMetricsInfo {
    pub name: String,
    pub cpu_cores: f64,      // CPU usage in cores (e.g., 0.161)
    pub cpu_percent: f64,    // CPU usage percentage
    pub memory_bytes: u64,   // Memory usage in bytes
    pub memory_percent: f64, // Memory usage percentage
}

// Combined response with node metrics and usage percentages
#[derive(Debug, Serialize, Deserialize)]
pub struct NodeMetricsWithUsageResponse {
    pub items: Vec<NodeMetricsInfo>,
}

impl NodeMetricsInfo {
    pub fn from_node_and_metrics(node: &Node, metrics: &NodeMetrics) -> Result<Self, String> {
        // Parse CPU capacity (e.g., "2" cores)
        let cpu_capacity: f64 =
            node.status.capacity.cpu.parse().map_err(|_| {
                format!("Failed to parse CPU capacity: {}", node.status.capacity.cpu)
            })?;

        // Parse memory capacity (e.g., "6026268Ki")
        let memory_capacity_ki = parse_memory_ki(&node.status.capacity.memory)?;

        // Parse CPU usage from nanoseconds (e.g., "160635734n")
        let cpu_usage_cores = parse_cpu_nanoseconds(&metrics.usage.cpu)?;

        // Parse memory usage (e.g., "1879200Ki")
        let memory_usage_ki = parse_memory_ki(&metrics.usage.memory)?;

        // Calculate percentages
        let cpu_percent = (cpu_usage_cores / cpu_capacity) * 100.0;
        let memory_percent = (memory_usage_ki as f64 / memory_capacity_ki as f64) * 100.0;

        Ok(NodeMetricsInfo {
            name: node.metadata.name.clone(),
            cpu_cores: cpu_usage_cores,
            cpu_percent,
            memory_bytes: memory_usage_ki * 1024, // Convert Ki to bytes
            memory_percent,
        })
    }
}

// Helper function to parse CPU from nanoseconds
fn parse_cpu_nanoseconds(cpu_str: &str) -> Result<f64, String> {
    if let Some(stripped) = cpu_str.strip_suffix('n') {
        let nanoseconds: f64 = stripped
            .parse()
            .map_err(|_| format!("Failed to parse CPU nanoseconds: {}", cpu_str))?;
        // Convert nanoseconds to cores (1 core = 1,000,000,000 nanoseconds)
        Ok(nanoseconds / 1_000_000_000.0)
    } else {
        Err(format!("Invalid CPU format: {}", cpu_str))
    }
}

// Helper function to parse memory in Ki
fn parse_memory_ki(mem_str: &str) -> Result<u64, String> {
    if let Some(stripped) = mem_str.strip_suffix("Ki") {
        stripped
            .parse()
            .map_err(|_| format!("Failed to parse memory Ki: {}", mem_str))
    } else {
        Err(format!("Invalid memory format: {}", mem_str))
    }
}

impl NodeMetricsListResponse {
    pub fn combine_with_nodes(
        &self,
        nodes: &NodeListResponse,
    ) -> Result<NodeMetricsWithUsageResponse, KubeAgentError> {
        let mut items = Vec::new();

        for metrics in &self.items {
            // Find matching node
            let node = nodes
                .items
                .iter()
                .find(|n| n.metadata.name == metrics.metadata.name)
                .ok_or_else(|| {
                    KubeAgentError::ParseError(format!(
                        "No matching node found for metrics: {}",
                        metrics.metadata.name
                    ))
                })?;

            let info = NodeMetricsInfo::from_node_and_metrics(node, metrics)
                .map_err(|e| KubeAgentError::ParseError(e))?;
            items.push(info);
        }

        Ok(NodeMetricsWithUsageResponse { items })
    }
}
