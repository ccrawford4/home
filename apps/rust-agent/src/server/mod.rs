pub mod types;

use crate::agent::Agent;
use crate::redis::{ChatDetails, write_completed_chat_response, write_failed_chat_response};
use rig::{OneOrMany, completion::Message};
use rig::completion::message::UserContent;
use std::collections::HashMap;
use std::io::{self, prelude::*};
use std::net::{TcpListener, TcpStream};
use std::sync::Arc;
use tracing::{debug, error, info, warn};
use types::{ChatRequest, Method, Path, Request};

/// HTTP server that handles AI chat requests.
///
/// Implements a custom TCP-based HTTP/1.1 server without using a web framework.
/// Provides endpoints for health checks and AI-powered chat interactions.
pub struct Server {
    agent: Arc<Agent>,
    host: String,
    api_key: String,
}

#[derive(Clone)]
pub struct ChatRequestState {
    pub request_id: String,
}

tokio::task_local! {
    pub static CHAT_REQUEST_STATE: ChatRequestState;
}

impl Server {
    pub fn new(agent: Arc<Agent>, host: String, api_key: String) -> Self {
        Server {
            agent,
            host,
            api_key,
        }
    }

    /// Starts the server and listens for incoming connections.
    ///
    /// Blocks indefinitely, handling requests synchronously (one at a time).
    /// Each connection is processed completely before accepting the next one.
    pub async fn listen(&self) -> io::Result<()> {
        let listener = TcpListener::bind(&self.host)?;
        info!("Server listening on {}", self.host);

        for stream in listener.incoming() {
            match stream {
                Ok(stream) => {
                    debug!("Accepted connection from {:?}", stream.peer_addr());
                    if let Err(e) = self.handle_client(stream).await {
                        error!("Error handling client: {}", e);
                    }
                }
                Err(e) => {
                    warn!("Failed to accept connection: {}", e);
                }
            }
        }

        Ok(())
    }

    /// Handles a single client connection.
    ///
    /// Reads the HTTP request, validates the API key, routes to appropriate handler,
    /// and sends the response.
    async fn handle_client(&self, mut stream: TcpStream) -> Result<(), std::io::Error> {
        let mut buffer = [0; 100000]; // 100KB buffer for request
        let bytes_read = stream.read(&mut buffer)?;
        let request_str = String::from_utf8_lossy(&buffer[..bytes_read]);

        match Request::parse(&request_str) {
            Some(request) => {
                debug!(
                    "Parsed request: method={:?}, path={:?}",
                    request.method, request.path
                );

                // Validate API key
                if let Some(api_key) = &request.api_key {
                    if *api_key != self.api_key {
                        warn!("Invalid API key attempt");
                        return Self::send_response(
                            &mut stream,
                            "403 Forbidden",
                            "Invalid API key",
                        );
                    }
                    debug!("API key validated successfully");
                } else {
                    warn!("Request missing API key");
                    return Self::send_response(&mut stream, "401 Unauthorized", "Missing API key");
                }

                match request.path {
                    Path::Chat => {
                        self.chat_handler(&mut stream, request.method, request.body)
                            .await
                    }
                    Path::ChatResponse => {
                        self.chat_response_handler(
                            &mut stream,
                            request.method,
                            request.query_params,
                        )
                        .await
                    }
                    Path::Tools => {
                        self.tools_handler(&mut stream, request.method, request.query_params)
                            .await
                    }
                    Path::Root => self.root_handler(&mut stream),
                    Path::Favicon => {
                        debug!("Favicon request received, returning 404");
                        Self::send_response(&mut stream, "404 Not Found", "Favicon not found")
                    }
                }
            }
            None => {
                warn!("Received malformed request, returning 400");
                debug!("Request string: {}", request_str);
                Self::send_response(&mut stream, "400 Bad Request", "Invalid request")
            }
        }
    }

    /// Sends an HTTP response to the client.
    fn send_response(stream: &mut TcpStream, status: &str, body: &str) -> io::Result<()> {
        debug!("Sending response: {}", status);
        let response = format!(
            "HTTP/1.1 {}\r\nContent-Length: {}\r\n\r\n{}",
            status,
            body.len(),
            body
        );
        stream.write_all(response.as_bytes())?;
        stream.flush()
    }

    /// Handles POST /chat requests by processing the prompt through the AI agent.
    async fn chat_handler(
        &self,
        stream: &mut TcpStream,
        method: Method,
        body: Option<String>,
    ) -> io::Result<()> {
        match method {
            Method::POST => {
                let body_str = match body {
                    Some(b) => b,
                    None => {
                        warn!("Chat request missing body");
                        return Self::send_response(
                            stream,
                            "400 Bad Request",
                            "Missing request body",
                        );
                    }
                };

                match serde_json::from_str::<ChatRequest>(&body_str) {
                    Ok(chat_req) => {
                        info!(
                            "Processing chat request ({} chars) with request_id: {}",
                            chat_req.prompt.len(),
                            chat_req.request_id
                        );

                        if chat_req.request_id.is_empty() {
                            warn!("Chat request has empty request_id");
                            return Self::send_response(
                                stream,
                                "400 Bad Request",
                                "request_id cannot be empty",
                            );
                        }

                        // Convert chat history to internal message format
                        let mut chat_history: Vec<Message> = Vec::new();
                        if let Some(history) = chat_req.chat_history {
                            debug!("Including {} historical messages", history.len());
                            let mut converted_history = Vec::new();
                            for msg in history {
                                match msg.try_into() {
                                    Ok(m) => converted_history.push(m),
                                    Err(e) => {
                                        warn!("Invalid message role in chat history: {}", e);
                                        return Self::send_response(
                                            stream,
                                            "400 Bad Request",
                                            "Invalid message role in chat history",
                                        );
                                    }
                                }
                            }
                            chat_history = converted_history;
                        }

                        let prompt = chat_req.prompt;
                        let agent = Arc::clone(&self.agent);

                        // Set the local state
                        let state = ChatRequestState {
                            request_id: chat_req.request_id.clone(),
                        };

                        tokio::spawn(async move {
                            CHAT_REQUEST_STATE
                                .scope(state, async move {
                                    // Create a local copy of the request_id so the redis hooks
                                    // will have access to it even if the original state is
                                    // dropped.
                                    let request_id =
                                        CHAT_REQUEST_STATE.with(|s| s.request_id.clone());
                                    let prompt_for_redis = prompt.clone();
                                    let history_for_redis = chat_history.clone();
                                    let chat_result = match agent
                                        .chat(prompt, chat_history, request_id.clone())
                                        .await
                                    {
                                        Ok(resp) => Ok(resp),
                                        Err(e) => Err(e.to_string()),
                                    };

                                    match chat_result {
                                        Ok(resp) => {
                                            if let Err(e) =
                                                write_completed_chat_response(&request_id, &resp)
                                                    .await
                                            {
                                                error!(
                                                    "Failed to write completed chat response for request_id={}: {}",
                                                    request_id, e
                                                );
                                            }
                                            info!(
                                                "Background chat completed for request_id={} ({} chars)",
                                                request_id,
                                                resp.len()
                                            );
                                        }
                                        Err(error_message) => {
                                            if let Err(e) = write_failed_chat_response(
                                                &request_id,
                                                &error_message,
                                                ChatDetails {
                                                    prompt: Message::User {
                                                        content: OneOrMany::one(
                                                            UserContent::Text(
                                                                prompt_for_redis.into(),
                                                            ),
                                                        ),
                                                    },
                                                    history: history_for_redis,
                                                },
                                            )
                                            .await
                                            {
                                                error!(
                                                    "Failed to write failed chat response for request_id={}: {}",
                                                    request_id, e
                                                );
                                            }
                                            error!(
                                                "Background chat failed for request_id={}: {}",
                                                request_id, error_message
                                            );
                                        }
                                    }
                                })
                                .await;
                        });

                        // Send the immediate response to the client indicating that the request is
                        // being processed asynchronously.
                        Self::send_response(stream, "200 OK", "")
                    }
                    Err(e) => {
                        warn!("Failed to parse chat request body: {}", e);
                        Self::send_response(
                            stream,
                            "400 Bad Request",
                            "Invalid JSON in request body",
                        )
                    }
                }
            }
            Method::GET => {
                warn!("Received GET request on /chat endpoint, which is not allowed");
                Self::send_response(stream, "405 Method Not Allowed", "Invalid method for /chat")
            }
        }
    }

    async fn chat_response_handler(
        &self,
        stream: &mut TcpStream,
        method: Method,
        query_params: HashMap<String, String>,
    ) -> io::Result<()> {
        match method {
            Method::GET => {
                let Some(request_id) = query_params.get("request_id") else {
                    warn!("Chat response request missing request_id query parameter");
                    return Self::send_response(
                        stream,
                        "400 Bad Request",
                        "{\"error\":\"Missing request_id query parameter\"}",
                    );
                };

                if request_id.is_empty() {
                    warn!("Chat response request has empty request_id query parameter");
                    return Self::send_response(
                        stream,
                        "400 Bad Request",
                        "{\"error\":\"request_id cannot be empty\"}",
                    );
                }

                match crate::redis::read_chat_response(request_id).await {
                    Ok(Some(record)) if record.status == "pending" => {
                        let body = serde_json::to_string(&record).map_err(io::Error::other)?;
                        Self::send_response(stream, "202 Accepted", &body)
                    }
                    Ok(Some(record)) if record.status == "completed" => {
                        let body = serde_json::to_string(&record).map_err(io::Error::other)?;
                        Self::send_response(stream, "200 OK", &body)
                    }
                    Ok(Some(record)) if record.status == "failed" => {
                        let body = serde_json::to_string(&record).map_err(io::Error::other)?;
                        Self::send_response(stream, "500 Internal Server Error", &body)
                    }
                    Ok(Some(record)) => {
                        warn!(
                            "Chat response record has unknown status={} for request_id={}",
                            record.status, request_id
                        );
                        let body = serde_json::to_string(&record).map_err(io::Error::other)?;
                        Self::send_response(stream, "500 Internal Server Error", &body)
                    }
                    Ok(None) => Self::send_response(
                        stream,
                        "404 Not Found",
                        "{\"error\":\"No async chat response found for request_id\"}",
                    ),
                    Err(e) => {
                        error!(
                            "Failed to read async chat response from Redis for request_id={}: {}",
                            request_id, e
                        );
                        Self::send_response(
                            stream,
                            "500 Internal Server Error",
                            "{\"error\":\"Failed to read async chat response\"}",
                        )
                    }
                }
            }
            _ => Self::send_response(
                stream,
                "405 Method Not Allowed",
                "Invalid method for /chat/response",
            ),
        }
    }

    /// Handles GET /api/tools requests by returning logged tool calls from Redis.
    async fn tools_handler(
        &self,
        stream: &mut TcpStream,
        method: Method,
        query_params: std::collections::HashMap<String, String>,
    ) -> io::Result<()> {
        match method {
            Method::GET => {
                let Some(response_id) = query_params.get("response_id") else {
                    warn!("Tools request missing response_id query parameter");
                    return Self::send_response(
                        stream,
                        "400 Bad Request",
                        "{\"error\":\"Missing response_id query parameter\"}",
                    );
                };

                if response_id.is_empty() {
                    warn!("Tools request has empty response_id query parameter");
                    return Self::send_response(
                        stream,
                        "400 Bad Request",
                        "{\"error\":\"response_id cannot be empty\"}",
                    );
                }

                match (
                    crate::redis::read_tool_calls(response_id).await,
                    crate::redis::read_request_events(response_id).await,
                ) {
                    (Ok(tool_calls), Ok(events)) => {
                        let body = serde_json::json!({
                            "response_id": response_id,
                            "tools": tool_calls,
                            "events": events,
                        })
                        .to_string();
                        Self::send_response(stream, "200 OK", &body)
                    }
                    (Err(e), _) | (_, Err(e)) => {
                        error!(
                            "Failed to read request data for response_id={}: {}",
                            response_id, e
                        );
                        Self::send_response(
                            stream,
                            "500 Internal Server Error",
                            "{\"error\":\"Failed to fetch tool calls\"}",
                        )
                    }
                }
            }
            _ => {
                warn!("Invalid HTTP method for /api/tools endpoint");
                Self::send_response(
                    stream,
                    "405 Method Not Allowed",
                    "Invalid method for /api/tools",
                )
            }
        }
    }

    /// Handles GET / requests (health check endpoint).
    fn root_handler(&self, stream: &mut TcpStream) -> io::Result<()> {
        debug!("Health check requested");
        Self::send_response(stream, "200 OK", "{\"healthy\": true}")
    }
}
