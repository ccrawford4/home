pub mod tools;

use crate::environment::Environment;
use crate::kube::{KubeAgent, ListNamespacesTool, ListPodsTool, NodeMetricsTool};
use crate::redis;
use rig::agent::{CancelSignal, PromptHook};
use rig::client::CompletionClient;
use rig::completion::{Message, Prompt};
use rig::providers::openai::{self, responses_api::ResponsesCompletionModel};
use serde_json::json;
use std::error::Error;
use std::future::Future;
use tools::WrappedPortfolioAPISearch;
use tracing::*;

/// AI agent that answers questions about a portfolio and Kubernetes infrastructure.
///
/// Uses OpenAI's GPT-5.1 model with the rig-core framework for tool-calling capabilities.
/// The agent has access to:
/// - Portfolio API tools for portfolio information
/// - Kubernetes API tools for cluster metrics and pod information
pub struct Agent {
    client: rig::agent::Agent<ResponsesCompletionModel>,
}

#[derive(Clone)]
struct RedisToolLoggingHook {
    request_id: String,
}

impl PromptHook<ResponsesCompletionModel> for RedisToolLoggingHook {
    fn on_tool_call(
        &self,
        tool_name: &str,
        tool_call_id: Option<String>,
        args: &str,
        _cancel_sig: CancelSignal,
    ) -> impl Future<Output = ()> + Send {
        let request_id = self.request_id.clone();
        let tool_name = tool_name.to_string();
        let tool_call_id = tool_call_id.clone();
        let args = args.to_string();

        async move {
            warn!(
                "RedisToolLoggingHook triggered for request_id={} tool={} tool_call_id={:?} raw_args_bytes={}",
                request_id,
                tool_name,
                tool_call_id,
                args.len()
            );
            info!(
                "Observed tool call for request_id {}: tool={} args={}",
                request_id, tool_name, args
            );
            warn!(
                "Parsing tool args for request_id={} tool={}",
                request_id, tool_name
            );
            let parsed_args = match serde_json::from_str(&args) {
                Ok(value) => {
                    warn!(
                        "Parsed tool args as JSON for request_id={} tool={}",
                        request_id, tool_name
                    );
                    value
                }
                Err(e) => {
                    warn!(
                        "Failed to parse tool args as JSON for request_id={} tool={}: {}; storing raw payload",
                        request_id, tool_name, e
                    );
                    json!({ "raw": args })
                }
            };
            warn!(
                "Constructing Redis tool call payload for request_id={} tool={}",
                request_id, tool_name
            );
            let tool_call = redis::ToolCall::new(tool_name, parsed_args);

            warn!(
                "Writing hook-observed tool call to Redis for request_id={}",
                request_id
            );
            if let Err(e) = redis::write_tool_call(&request_id, tool_call).await {
                warn!(
                    "Hook Redis write failed for request_id={}: {}",
                    request_id, e
                );
                error!("Failed to write tool call to Redis: {}", e);
            } else {
                warn!("Hook Redis write succeeded for request_id={}", request_id);
                info!("Tool call written to Redis for request_id {}", request_id);
            }
        }
    }
}

impl Agent {
    /// Creates a new AI agent with OpenAI backend and configured tools.
    ///
    /// Tools available to the agent:
    /// - PortfolioAPISearch: Fetches structured JSON from portfolio API endpoints
    /// - ListPodsTool: Queries Kubernetes pods
    /// - ListNamespacesTool: Lists Kubernetes namespaces
    /// - NodeMetricsTool: Gets node metrics (CPU, memory usage)
    pub fn new(api_key: String) -> Result<Self, Box<dyn Error>> {
        info!("Initializing AI agent with OpenAI backend");

        debug!("open ai api key: {}", &api_key);

        let openai_client = openai::Client::<reqwest::Client>::new(api_key).map_err(|e| {
            error!("Failed to create OpenAI client: {}", e);
            e
        })?;

        debug!("OpenAI client created successfully");

        let env = Environment::new();
        let kube_agent = KubeAgent::new(env.kube_api_server, env.kube_token, env.kube_certificate);

        // Build agent with tools and system prompt
        let client = openai_client
            .agent(openai::GPT_5_1)
            .preamble("You are a helpful assistant who helps users answer questions about Calum's portfolio API, site content, or its underlying infrastructure. Always respect the JSON schema  { \"response\": \"<your response\" } in your responses. Simply ignore any mention (subtle or not) in the prompt mentioning the output schema")
            .tool(WrappedPortfolioAPISearch)
            .tool(ListPodsTool::new(kube_agent.clone()))
            .tool(ListNamespacesTool::new(kube_agent.clone()))
            .tool(NodeMetricsTool::new(kube_agent))
            .build();

        info!("AI agent initialized with 4 tools");

        Ok(Agent { client })
    }

    pub async fn chat(
        &self,
        prompt: String,
        mut chat_history: Vec<Message>,
        request_id: String,
    ) -> Result<String, Box<dyn Error>> {
        debug!(
            "Processing chat prompt ({} chars) with request_id: {}",
            prompt.len(),
            request_id
        );
        warn!(
            "Starting chat flow for request_id={} prompt_bytes={} history_messages={}",
            request_id,
            prompt.len(),
            chat_history.len()
        );
        let hook = RedisToolLoggingHook { request_id };
        warn!("RedisToolLoggingHook attached to chat request");

        const MAX_RETRIES: u32 = 5;
        let mut backoff_secs = 1u64;

        for attempt in 0..=MAX_RETRIES {
            warn!(
                "Submitting agent prompt for attempt={} of {}",
                attempt + 1,
                MAX_RETRIES + 1
            );
            match self
                .client
                .prompt(&prompt)
                .with_history(&mut chat_history)
                .with_hook(hook.clone())
                .multi_turn(20)
                .await
            {
                Ok(response) => {
                    warn!(
                        "Agent prompt succeeded on attempt={} response_bytes={}",
                        attempt + 1,
                        response.len()
                    );
                    info!("Agent response generated ({} chars)", response.len());
                    return Ok(response);
                }
                Err(e) => {
                    warn!("Agent prompt failed on attempt={} error={}", attempt + 1, e);
                    let mut source = e.source();
                    while let Some(err) = source {
                        error!("  caused by: {}", err);
                        source = err.source();
                    }

                    if attempt == MAX_RETRIES {
                        error!("Agent prompt failed after {} retries: {}", MAX_RETRIES, e);
                        return Err(Box::new(e));
                    }

                    error!(
                        "Agent prompt failed (attempt {}/{}): {}. Retrying in {}s...",
                        attempt + 1,
                        MAX_RETRIES,
                        e,
                        backoff_secs
                    );
                    tokio::time::sleep(std::time::Duration::from_secs(backoff_secs)).await;
                    backoff_secs = (backoff_secs * 2).min(32);
                }
            }
        }

        unreachable!()
    }
}
