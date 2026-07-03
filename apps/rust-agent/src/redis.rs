use once_cell::sync::OnceCell;
use redis::Client;
use rig::completion::Message;
use serde::{Deserialize, Serialize};
use tracing::warn;

static REDIS_CLIENT: OnceCell<Client> = OnceCell::new();

pub async fn init(redis_url: &str, skip_redis: &bool) -> Result<(), redis::RedisError> {
    if *skip_redis {
        warn!("Skipping Redis initialization due to configuration");
        return Ok(());
    }

    let client = REDIS_CLIENT.get_or_try_init(|| Client::open(redis_url))?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    redis::cmd("PING").query_async::<_, String>(&mut conn).await?;
    Ok(())
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
#[non_exhaustive]
pub enum RequestEventRecord {
    CompletionResponse {
        timestamp: String,
        prompt: Message,
        response: serde_json::Value,
    },
    InvalidToolCall {
        timestamp: String,
        tool_name: String,
        tool_call_id: Option<String>,
        internal_call_id: Option<String>,
        args: Option<String>,
        available_tools: Vec<String>,
        allowed_tools: Vec<String>,
        tool_choice: Option<serde_json::Value>,
        chat_history: Vec<Message>,
        is_streaming: bool,
    },
    ToolCall {
        timestamp: String,
        tool_name: String,
        tool_call_id: Option<String>,
        internal_call_id: String,
        args: serde_json::Value,
    },
    ToolCallResult {
        timestamp: String,
        tool_name: String,
        tool_call_id: Option<String>,
        internal_call_id: String,
        args: serde_json::Value,
        result: serde_json::Value,
    },
    TextDelta {
        timestamp: String,
        text_delta: String,
        aggregated_text: String,
    },
    ToolCallDelta {
        timestamp: String,
        tool_call_id: String,
        internal_call_id: String,
        tool_name: Option<String>,
        tool_call_delta: String,
    },
    StreamCompletionResponseFinish {
        timestamp: String,
        prompt: Message,
        response: serde_json::Value,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolCallRecord {
    pub name: String,
    pub args: serde_json::Value,
    pub timestamp: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatDetails {
    pub prompt: Message,
    pub history: Vec<Message>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatResponseRecord {
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<ChatDetails>,
    pub timestamp: String,
}

pub struct ToolCall {
    pub name: String,
    pub args: serde_json::Value,
}

async fn get_client() -> Result<&'static Client, Box<dyn std::error::Error>> {
    REDIS_CLIENT
        .get()
        .ok_or_else(|| "Redis client not initialized".into())
}

async fn write_json<T: Serialize>(key: &str, record: &T) -> Result<(), Box<dyn std::error::Error>> {
    let client = get_client().await?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    let json = serde_json::to_string(record)?;
    redis::cmd("RPUSH")
        .arg(key)
        .arg(json)
        .query_async::<_, ()>(&mut conn)
        .await?;
    Ok(())
}

pub async fn write_tool_call(
    request_id: &str,
    tool_call: ToolCall,
) -> Result<(), Box<dyn std::error::Error>> {
    let record = ToolCallRecord {
        name: tool_call.name,
        args: tool_call.args,
        timestamp: chrono::Utc::now().to_rfc3339(),
    };
    write_json(&format!("request:{}:tool_calls", request_id), &record).await
}

pub async fn write_request_event(
    request_id: &str,
    record: &RequestEventRecord,
) -> Result<(), Box<dyn std::error::Error>> {
    write_json(&format!("request:{}:events", request_id), record).await
}

pub async fn read_tool_calls(
    request_id: &str,
) -> Result<Vec<ToolCallRecord>, Box<dyn std::error::Error>> {
    let client = get_client().await?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    let raw_records: Vec<String> = redis::cmd("LRANGE")
        .arg(format!("request:{}:tool_calls", request_id))
        .arg(0)
        .arg(-1)
        .query_async(&mut conn)
        .await?;
    Ok(raw_records
        .into_iter()
        .map(|raw| serde_json::from_str::<ToolCallRecord>(&raw))
        .collect::<Result<Vec<_>, _>>()?)
}

pub async fn read_request_events(
    request_id: &str,
) -> Result<Vec<RequestEventRecord>, Box<dyn std::error::Error>> {
    let client = get_client().await?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    let raw_records: Vec<String> = redis::cmd("LRANGE")
        .arg(format!("request:{}:events", request_id))
        .arg(0)
        .arg(-1)
        .query_async(&mut conn)
        .await?;
    Ok(raw_records
        .into_iter()
        .map(|raw| serde_json::from_str::<RequestEventRecord>(&raw))
        .collect::<Result<Vec<_>, _>>()?)
}

pub async fn write_pending_chat_response(
    request_id: &str,
    prompt: Message,
    history: Vec<Message>,
) -> Result<(), Box<dyn std::error::Error>> {
    write_chat_response_record(
        request_id,
        ChatResponseRecord {
            status: "pending".to_string(),
            response: None,
            error: None,
            details: Some(ChatDetails { prompt, history }),
            timestamp: chrono::Utc::now().to_rfc3339(),
        },
    )
    .await
}

pub async fn write_completed_chat_response(
    request_id: &str,
    response: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    write_chat_response_record(
        request_id,
        ChatResponseRecord {
            status: "completed".to_string(),
            response: Some(response.to_string()),
            error: None,
            details: None,
            timestamp: chrono::Utc::now().to_rfc3339(),
        },
    )
    .await
}

pub async fn write_failed_chat_response(
    request_id: &str,
    error_message: &str,
    details: ChatDetails,
) -> Result<(), Box<dyn std::error::Error>> {
    write_chat_response_record(
        request_id,
        ChatResponseRecord {
            status: "failed".to_string(),
            response: None,
            error: Some(error_message.to_string()),
            details: Some(details),
            timestamp: chrono::Utc::now().to_rfc3339(),
        },
    )
    .await
}

pub async fn read_chat_response(
    request_id: &str,
) -> Result<Option<ChatResponseRecord>, Box<dyn std::error::Error>> {
    let client = get_client().await?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    let raw_record: Option<String> = redis::cmd("GET")
        .arg(format!("chat_response_{}", request_id))
        .query_async(&mut conn)
        .await?;
    Ok(raw_record
        .map(|raw| serde_json::from_str::<ChatResponseRecord>(&raw))
        .transpose()?)
}

async fn write_chat_response_record(
    request_id: &str,
    record: ChatResponseRecord,
) -> Result<(), Box<dyn std::error::Error>> {
    let client = get_client().await?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    let json_str = serde_json::to_string(&record)?;
    redis::cmd("SET")
        .arg(format!("chat_response_{}", request_id))
        .arg(json_str)
        .query_async::<_, ()>(&mut conn)
        .await?;
    Ok(())
}

pub fn tool_args_from_str(args: &str) -> serde_json::Value {
    serde_json::from_str(args).unwrap_or_else(|_| serde_json::Value::String(args.to_string()))
}

pub fn response_to_json<T>(_: &T) -> serde_json::Value {
    serde_json::json!({
        "type": std::any::type_name::<T>(),
    })
}
