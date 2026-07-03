use rig::completion::Message;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// HTTP methods supported by the server
#[derive(Debug)]
pub enum Method {
    GET,
    POST,
}

impl Method {
    pub fn from_str(s: &str) -> Option<Self> {
        match s {
            "GET" => Some(Method::GET),
            "POST" => Some(Method::POST),
            _ => None,
        }
    }
}

/// HTTP paths (routes) supported by the server
#[derive(Debug)]
pub enum Path {
    /// POST /chat - Main chat endpoint for AI interactions
    Chat,
    /// GET /chat/response - Poll for an async chat response by request id
    ChatResponse,
    /// GET /api/tools - Fetch logged tool calls for a response/request id
    Tools,
    /// GET / - Health check endpoint
    Root,
    /// GET /favicon.ico - Favicon request (returns 404)
    Favicon,
}

impl Path {
    pub fn from_str(s: &str) -> Option<Self> {
        match s {
            "/chat" => Some(Path::Chat),
            "/chat/response" => Some(Path::ChatResponse),
            "/api/tools" => Some(Path::Tools),
            "/" => Some(Path::Root),
            "/favicon.ico" => Some(Path::Favicon),
            _ => None,
        }
    }
}

/// Parsed HTTP request with relevant fields extracted
#[derive(Debug)]
pub struct Request {
    pub method: Method,
    pub path: Path,
    pub query_params: HashMap<String, String>,
    pub api_key: Option<String>,
    pub body: Option<String>,
}

impl Request {
    /// Parses an HTTP/1.1 request string into a Request struct.
    ///
    /// Extracts:
    /// - HTTP method and path from the request line
    /// - X-API-Key header for authentication
    /// - Request body based on Content-Length header
    ///
    /// Returns None if the request is malformed or uses unsupported method/path.
    pub fn parse(request_str: &str) -> Option<Self> {
        let mut lines = request_str.lines();
        let first_line = lines.next()?;
        let mut parts = first_line.split_whitespace();

        let method = parts.next().and_then(Method::from_str)?;
        let raw_path = parts.next()?;
        let (path_str, query_params) = parse_path_and_query(raw_path);
        let path = Path::from_str(path_str)?;

        let mut content_length = 0;
        let mut api_key = None;

        // Parse headers
        for line in lines.by_ref() {
            if line.is_empty() {
                break;
            }
            if line.to_lowercase().starts_with("x-api-key:") {
                if let Some(key_str) = line.split(':').nth(1) {
                    api_key = Some(key_str.trim().to_string());
                }
            }
            if line.to_lowercase().starts_with("content-length:") {
                if let Some(len_str) = line.split(':').nth(1) {
                    content_length = len_str.trim().parse().unwrap_or(0);
                }
            }
        }

        // Extract body if present
        let body = if content_length > 0 {
            let body_str: String = lines.collect::<Vec<_>>().join("\n");
            Some(body_str)
        } else {
            None
        };

        Some(Request {
            method,
            path,
            query_params,
            body,
            api_key,
        })
    }
}

fn parse_path_and_query(raw_path: &str) -> (&str, HashMap<String, String>) {
    let Some((path, query)) = raw_path.split_once('?') else {
        return (raw_path, HashMap::new());
    };

    let mut query_params = HashMap::new();
    for pair in query.split('&') {
        if pair.is_empty() {
            continue;
        }

        let (key, value) = match pair.split_once('=') {
            Some((key, value)) => (key, value),
            None => (pair, ""),
        };

        query_params.insert(key.to_string(), value.to_string());
    }

    (path, query_params)
}

/// Request payload for the /chat endpoint
#[derive(Debug, Deserialize, Serialize)]
pub struct ChatRequest {
    /// Unique request identifier for tracking tool calls in Redis
    pub request_id: String,
    /// The user's prompt/question
    pub prompt: String,
    /// Optional conversation history for context
    pub chat_history: Option<Vec<HttpMessage>>,
}

/// A single message in a chat conversation
#[derive(Debug, Deserialize, Serialize)]
pub struct HttpMessage {
    /// Message role: "user" or "assistant"
    pub role: String,
    /// Message content/text
    pub content: String,
}

impl TryFrom<HttpMessage> for Message {
    type Error = &'static str;

    fn try_from(value: HttpMessage) -> Result<Self, Self::Error> {
        match value.role.as_str() {
            "user" => Ok(Message::user(value.content)),
            "assistant" => Ok(Message::assistant(value.content)),
            _ => Err("Invalid role in HttpMessage"),
        }
    }
}
