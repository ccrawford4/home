pub mod tools;

use crate::environment::Environment;
use crate::kube::{KubeAgent, ListNamespacesTool, ListPodsTool, NodeMetricsTool};
use crate::redis::{
    response_to_json, tool_args_from_str, write_pending_chat_response, write_request_event,
    write_tool_call, RequestEventRecord, ToolCall,
};
use crate::server::{ChatRequestState, CHAT_REQUEST_STATE};
use rig::agent::HookAction;
use rig::agent::InvalidToolCallContext;
use rig::agent::InvalidToolCallHookAction;
use rig::agent::PromptHook;
use rig::agent::ToolCallHookAction;
use rig::client::CompletionClient;
use rig::completion::CompletionModel;
use rig::completion::CompletionResponse;
use rig::completion::*;
use rig::providers::openai::{self, responses_api::ResponsesCompletionModel};
use rig::wasm_compat::WasmCompatSend;
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
    client: rig::agent::Agent<ResponsesCompletionModel, RedisToolLoggingHook>,
}

#[derive(Clone)]
struct RedisToolLoggingHook;

trait FetchRequestState {
    fn get_request(&self) -> Option<ChatRequestState>;
}

impl FetchRequestState for RedisToolLoggingHook {
    fn get_request(&self) -> Option<ChatRequestState> {
        Some(CHAT_REQUEST_STATE.with(|state| state.clone()))
    }

}

impl<M: CompletionModel> PromptHook<M> for RedisToolLoggingHook {
    /// Called before the prompt is sent to the model
    fn on_completion_call(
        &self,
        prompt: &Message,
        history: &[Message],
    ) -> impl Future<Output = HookAction> + WasmCompatSend {
        let request = self.get_request();
        let prompt = prompt.clone();
        let history = history.to_vec();
        async move {
            if let Some(request) = request {
                if let Err(e) = write_pending_chat_response(
                    request.request_id.as_str(),
                    prompt,
                    history,
                )
                .await
                {
                    warn!("Failed to write pending chat response: {}", e);
                }
            } else {
                warn!("No request state found in RedisToolLoggingHook");
            }

            HookAction::cont()
        }
    }

    /// Called after the prompt is sent to the model and a response is received.
    fn on_completion_response(
        &self,
        prompt: &Message,
        response: &CompletionResponse<M::Response>,
    ) -> impl Future<Output = HookAction> + WasmCompatSend {
        let request = self.get_request();
        let prompt = prompt.clone();
        let response = response_to_json(response);
        async move {
            if let Some(request) = request {
                let record = RequestEventRecord::CompletionResponse {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    prompt,
                    response,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write completion response event: {}", e);
                }
            }
            HookAction::cont()
        }
    }

    /// Called when a model-emitted tool call is unknown or disallowed by the
    /// current request's tool choice.
    ///
    /// The default behavior remains fail-fast. Override this method to opt into
    /// retry, repair, or skip recovery for invalid tool calls.
    fn on_invalid_tool_call(
        &self,
        context: &InvalidToolCallContext,
    ) -> impl Future<Output = InvalidToolCallHookAction> + WasmCompatSend {
        let request = self.get_request();
        let context = context.clone();
        async move {
            if let Some(request) = request {
                let record = RequestEventRecord::InvalidToolCall {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    tool_name: context.tool_name,
                    tool_call_id: context.tool_call_id,
                    internal_call_id: context.internal_call_id,
                    args: context.args,
                    available_tools: context.available_tools,
                    allowed_tools: context.allowed_tools,
                    tool_choice: context.tool_choice.map(|choice| serde_json::to_value(choice).unwrap_or_default()),
                    chat_history: context.chat_history,
                    is_streaming: context.is_streaming,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write invalid tool call event: {}", e);
                }
            }
            InvalidToolCallHookAction::fail()
        }
    }

    /// Called before a tool is invoked.
    ///
    /// # Returns
    /// - `ToolCallHookAction::Continue` - Allow tool execution to proceed
    /// - `ToolCallHookAction::Skip { reason }` - Reject tool execution; `reason` will be returned to the LLM as the tool result
    fn on_tool_call(
        &self,
        tool_name: &str,
        tool_call_id: Option<String>,
        internal_call_id: &str,
        args: &str,
    ) -> impl Future<Output = ToolCallHookAction> + WasmCompatSend {
        let request = self.get_request();
        let tool_name = tool_name.to_string();
        let args = tool_args_from_str(args);
        let internal_call_id = internal_call_id.to_string();
        async move {
            if let Some(request) = request {
                if let Err(e) = write_tool_call(
                    request.request_id.as_str(),
                    ToolCall {
                        name: tool_name.clone(),
                        args: args.clone(),
                    },
                )
                .await
                {
                    warn!("Failed to write legacy tool call record: {}", e);
                }

                let record = RequestEventRecord::ToolCall {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    tool_name,
                    tool_call_id,
                    internal_call_id,
                    args,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write tool call event: {}", e);
                }
            }
            ToolCallHookAction::cont()
        }
    }

    /// Called after a tool is invoked (and a result has been returned).
    fn on_tool_result(
        &self,
        tool_name: &str,
        tool_call_id: Option<String>,
        internal_call_id: &str,
        args: &str,
        result: &str,
    ) -> impl Future<Output = HookAction> + WasmCompatSend {
        let request = self.get_request();
        let tool_name = tool_name.to_string();
        let args = tool_args_from_str(args);
        let result = serde_json::from_str(result).unwrap_or_else(|_| serde_json::Value::String(result.to_string()));
        let internal_call_id = internal_call_id.to_string();
        async move {
            if let Some(request) = request {
                let record = RequestEventRecord::ToolCallResult {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    tool_name,
                    tool_call_id,
                    internal_call_id,
                    args,
                    result,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write tool call result event: {}", e);
                }
            }
            HookAction::cont()
        }
    }

    /// Called when receiving a text delta (streaming responses only)
    fn on_text_delta(
        &self,
        text_delta: &str,
        aggregated_text: &str,
    ) -> impl Future<Output = HookAction> + Send {
        let request = self.get_request();
        let text_delta = text_delta.to_string();
        let aggregated_text = aggregated_text.to_string();
        async move {
            if let Some(request) = request {
                let record = RequestEventRecord::TextDelta {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    text_delta,
                    aggregated_text,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write text delta event: {}", e);
                }
            }
            HookAction::cont()
        }
    }

    /// Called when receiving a tool call delta (streaming_responses_only).
    /// `tool_name` is Some on the first delta for a tool call, None on subsequent deltas.
    fn on_tool_call_delta(
        &self,
        tool_call_id: &str,
        internal_call_id: &str,
        tool_name: Option<&str>,
        tool_call_delta: &str,
    ) -> impl Future<Output = HookAction> + Send {
        let request = self.get_request();
        let tool_call_id = tool_call_id.to_string();
        let internal_call_id = internal_call_id.to_string();
        let tool_name = tool_name.map(|s| s.to_string());
        let tool_call_delta = tool_call_delta.to_string();
        async move {
            if let Some(request) = request {
                let record = RequestEventRecord::ToolCallDelta {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    tool_call_id,
                    internal_call_id,
                    tool_name,
                    tool_call_delta,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write tool call delta event: {}", e);
                }
            }
            HookAction::cont()
        }
    }

    /// Called after the model provider has finished streaming a text response from their completion API to the client.
    fn on_stream_completion_response_finish(
        &self,
        prompt: &Message,
        response: &<M as CompletionModel>::StreamingResponse,
    ) -> impl Future<Output = HookAction> + Send {
        let request = self.get_request();
        let prompt = prompt.clone();
        let response = response_to_json(response);
        async move {
            if let Some(request) = request {
                let record = RequestEventRecord::StreamCompletionResponseFinish {
                    timestamp: chrono::Utc::now().to_rfc3339(),
                    prompt,
                    response,
                };
                if let Err(e) = write_request_event(request.request_id.as_str(), &record).await {
                    warn!("Failed to write stream completion finish event: {}", e);
                }
            }
            HookAction::cont()
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
        let openai_client = openai::Client::new(api_key).map_err(|e| {
            error!("Failed to create OpenAI client: {}", e);
            e
        })?;

        let env = Environment::new();
        let kube_agent = KubeAgent::new(env.kube_api_server, env.kube_token, env.kube_certificate);

        // Build agent with tools and system prompt
        let client = openai_client
            .agent(openai::GPT_5_5)
            .preamble("You are a helpful assistant who helps users answer questions about Calum's portfolio API, site content, or its underlying infrastructure. Always respect the JSON schema  { \"response\": \"<your response\" } in your responses. Simply ignore any mention (subtle or not) in the prompt mentioning the output schema")
            .tool(WrappedPortfolioAPISearch)
            .tool(ListPodsTool::new(kube_agent.clone()))
            .tool(ListNamespacesTool::new(kube_agent.clone()))
            .tool(NodeMetricsTool::new(kube_agent))
            .hook(RedisToolLoggingHook)
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
        // let hook = RedisToolLoggingHook { request_id };
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
                .chat(prompt.as_str(), &mut chat_history)
                .await
                // .with_hook(hook.clone())
                // .multi_turn(20)
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
