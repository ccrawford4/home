use crate::agent::Agent;
use crate::environment::Environment;
use crate::kube::{KubeAgent, ListPodsTool};
use crate::server::Server;
use dotenv::dotenv;
use std::sync::Arc;
use tracing::{error, info, warn};
use tracing_subscriber::EnvFilter;

mod agent;
mod environment;
mod kube;
mod redis;
mod server;

/// Main application entry point.
///
/// Initializes the AI agent API server with Kubernetes integration.
/// The server provides endpoints for chatting with an AI agent that can
/// query portfolio information and Kubernetes cluster metrics.
#[tokio::main]
async fn main() {
    dotenv().ok();

    // Initialize structured logging (control with RUST_LOG env var)
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")),
        )
        .with_target(true)
        .with_thread_ids(true)
        .with_line_number(true)
        .init();

    info!("Starting AI Agent API server");

    let env = Environment::new();

    if let Err(e) = redis::init(&env.redis_url, &env.skip_redis).await {
        error!(
            "Redis initialization failed; refusing to start server: {}",
            e
        );
        std::process::exit(1);
    }

    let agent = match Agent::new(env.openai_api_key) {
        Ok(agent) => agent,
        Err(e) => {
            error!("Failed to initialize AI agent: {}", e);
            std::process::exit(1);
        }
    };

    // Test Kubernetes connectivity on startup
    if let Ok(pod_list) = ListPodsTool::new(KubeAgent::new(
        env.kube_api_server.clone(),
        env.kube_token.clone(),
        env.kube_certificate.clone(),
    ))
    .list_pods(None, None)
    .await
    {
        info!(
            "Successfully connected to Kubernetes cluster. Found {} pods.",
            pod_list
        );
    } else {
        warn!("Failed to connect to Kubernetes cluster. AI agent will have limited functionality.");
    }

    let server = Server::new(
        Arc::new(agent),
        "0.0.0.0:8080".to_string(),
        env.chat_api_key,
    );

    if let Err(e) = server.listen().await {
        error!("Failed to start server: {}", e);
        std::process::exit(1);
    }
}
