use once_cell::sync::OnceCell;
use redis::Client;
use serde::{Deserialize, Serialize};
use tracing::{debug, error, info, warn};

static REDIS_CLIENT: OnceCell<Client> = OnceCell::new();

pub async fn init(redis_url: &str) -> Result<(), redis::RedisError> {
    warn!(
        "Initializing Redis client from configured URL ({} chars)",
        redis_url.len()
    );

    let client = match REDIS_CLIENT.get_or_try_init(|| Client::open(redis_url)) {
        Ok(client) => {
            warn!("Redis client created successfully");
            client
        }
        Err(e) => {
            warn!("Redis client creation failed: {}", e);
            error!("Failed to create Redis client: {}", e);
            return Err(e);
        }
    };

    warn!("Verifying Redis connectivity with startup connection check");
    let mut conn = match client.get_multiplexed_async_connection().await {
        Ok(conn) => {
            warn!("Redis startup connection acquired successfully");
            conn
        }
        Err(e) => {
            warn!("Redis startup connection failed: {}", e);
            error!("Failed to connect to Redis during startup: {}", e);
            return Err(e);
        }
    };

    match redis::cmd("PING").query_async::<_, String>(&mut conn).await {
        Ok(response) => {
            warn!("Redis startup PING succeeded with response={}", response);
            Ok(())
        }
        Err(e) => {
            warn!("Redis startup PING failed: {}", e);
            error!("Failed to verify Redis during startup: {}", e);
            Err(e)
        }
    }
}

pub struct ToolCall {
    pub name: String,
    pub args: serde_json::Value,
}

impl ToolCall {
    pub fn new(name: String, args: serde_json::Value) -> Self {
        warn!(
            "Creating ToolCall for tool={} args_type={}",
            name,
            json_type_name(&args)
        );
        Self { name, args }
    }
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ToolCallRecord {
    pub name: String,
    pub args: serde_json::Value,
    pub timestamp: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ChatResponseRecord {
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
    pub timestamp: String,
}

pub async fn write_tool_call(
    request_id: &str,
    tool_call: ToolCall,
) -> Result<(), Box<dyn std::error::Error>> {
    warn!(
        "write_tool_call invoked for request_id={} tool={}",
        request_id, tool_call.name
    );
    let Some(client) = REDIS_CLIENT.get() else {
        warn!(
            "Redis client not initialized; skipping write for request_id={} tool={}",
            request_id, tool_call.name
        );
        debug!("Redis client not initialized, skipping tool call write");
        return Ok(());
    };

    info!("Writing tool call to Redis for request_id: {}", request_id);
    warn!(
        "Opening multiplexed Redis connection for request_id={} tool={}",
        request_id, tool_call.name
    );

    let mut conn = match client.get_multiplexed_async_connection().await {
        Ok(c) => {
            warn!(
                "Redis connection acquired for request_id={} tool={}",
                request_id, tool_call.name
            );
            c
        }
        Err(e) => {
            warn!(
                "Redis connection acquisition failed for request_id={} tool={}: {}",
                request_id, tool_call.name, e
            );
            error!("Failed to get Redis connection: {}", e);
            return Err(Box::new(e));
        }
    };

    warn!(
        "Building ToolCallRecord for request_id={} tool={} args_type={}",
        request_id,
        tool_call.name,
        json_type_name(&tool_call.args)
    );
    let record = ToolCallRecord {
        name: tool_call.name,
        args: tool_call.args,
        timestamp: chrono::Utc::now().to_rfc3339(),
    };

    warn!(
        "Serializing ToolCallRecord for request_id={} tool={} timestamp={}",
        request_id, record.name, record.timestamp
    );
    let json_str = serde_json::to_string(&record)?;
    let redis_key = format!("request:{}:tool_calls", request_id);
    warn!(
        "Serialized ToolCallRecord for request_id={} tool={} payload_bytes={}",
        request_id,
        record.name,
        json_str.len()
    );
    warn!(
        "Issuing Redis RPUSH for key={} request_id={} tool={}",
        redis_key, request_id, record.name
    );

    match redis::cmd("RPUSH")
        .arg(&redis_key)
        .arg(&json_str)
        .query_async::<_, ()>(&mut conn)
        .await
    {
        Ok(_) => {
            warn!(
                "Redis RPUSH completed for key={} request_id={} tool={}",
                redis_key, request_id, record.name
            );
            info!("Redis RPUSH succeeded for request_id: {}", request_id);
            Ok(())
        }
        Err(e) => {
            warn!(
                "Redis RPUSH failed for key={} request_id={} tool={}: {}",
                redis_key, request_id, record.name, e
            );
            error!("Failed to write tool call to Redis: {}", e);
            Err(Box::new(e))
        }
    }
}

pub async fn read_tool_calls(
    request_id: &str,
) -> Result<Vec<ToolCallRecord>, Box<dyn std::error::Error>> {
    warn!("read_tool_calls invoked for request_id={}", request_id);

    let Some(client) = REDIS_CLIENT.get() else {
        error!(
            "Redis client not initialized; cannot read tool calls for request_id={}",
            request_id
        );
        return Err("Redis client not initialized".into());
    };

    let mut conn = client.get_multiplexed_async_connection().await?;
    let redis_key = format!("request:{}:tool_calls", request_id);
    warn!(
        "Issuing Redis LRANGE for key={} request_id={}",
        redis_key, request_id
    );

    let raw_records: Vec<String> = redis::cmd("LRANGE")
        .arg(&redis_key)
        .arg(0)
        .arg(-1)
        .query_async(&mut conn)
        .await?;

    let mut records = Vec::with_capacity(raw_records.len());
    for raw_record in raw_records {
        let record: ToolCallRecord = serde_json::from_str(&raw_record)?;
        records.push(record);
    }

    info!(
        "Redis LRANGE succeeded for request_id={} with {} tool calls",
        request_id,
        records.len()
    );

    Ok(records)
}

pub async fn write_pending_chat_response(
    request_id: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    write_chat_response_record(
        request_id,
        ChatResponseRecord {
            status: "pending".to_string(),
            response: None,
            error: None,
            details: None,
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
    details: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    write_chat_response_record(
        request_id,
        ChatResponseRecord {
            status: "failed".to_string(),
            response: None,
            error: Some(error_message.to_string()),
            details: Some(details.to_string()),
            timestamp: chrono::Utc::now().to_rfc3339(),
        },
    )
    .await
}

pub async fn read_chat_response(
    request_id: &str,
) -> Result<Option<ChatResponseRecord>, Box<dyn std::error::Error>> {
    warn!("read_chat_response invoked for request_id={}", request_id);

    let Some(client) = REDIS_CLIENT.get() else {
        error!(
            "Redis client not initialized; cannot read chat response for request_id={}",
            request_id
        );
        return Err("Redis client not initialized".into());
    };

    let mut conn = client.get_multiplexed_async_connection().await?;
    let redis_key = format!("chat_response_{}", request_id);
    warn!(
        "Issuing Redis GET for key={} request_id={}",
        redis_key, request_id
    );

    let raw_record: Option<String> = redis::cmd("GET")
        .arg(&redis_key)
        .query_async(&mut conn)
        .await?;
    let Some(raw_record) = raw_record else {
        info!(
            "Redis GET returned no async chat response for request_id={}",
            request_id
        );
        return Ok(None);
    };

    let record: ChatResponseRecord = serde_json::from_str(&raw_record)?;
    info!(
        "Redis GET succeeded for async chat response request_id={} status={}",
        request_id, record.status
    );
    Ok(Some(record))
}

async fn write_chat_response_record(
    request_id: &str,
    record: ChatResponseRecord,
) -> Result<(), Box<dyn std::error::Error>> {
    warn!(
        "write_chat_response_record invoked for request_id={} status={}",
        request_id, record.status
    );

    let Some(client) = REDIS_CLIENT.get() else {
        error!(
            "Redis client not initialized; cannot write chat response for request_id={}",
            request_id
        );
        return Err("Redis client not initialized".into());
    };

    let mut conn = client.get_multiplexed_async_connection().await?;
    let redis_key = format!("chat_response_{}", request_id);
    let json_str = serde_json::to_string(&record)?;
    warn!(
        "Issuing Redis SET for key={} request_id={} payload_bytes={}",
        redis_key,
        request_id,
        json_str.len()
    );

    redis::cmd("SET")
        .arg(&redis_key)
        .arg(&json_str)
        .query_async::<_, ()>(&mut conn)
        .await?;

    info!(
        "Redis SET succeeded for async chat response request_id={} status={}",
        request_id, record.status
    );
    Ok(())
}

fn json_type_name(value: &serde_json::Value) -> &'static str {
    match value {
        serde_json::Value::Null => "null",
        serde_json::Value::Bool(_) => "bool",
        serde_json::Value::Number(_) => "number",
        serde_json::Value::String(_) => "string",
        serde_json::Value::Array(_) => "array",
        serde_json::Value::Object(_) => "object",
    }
}
