# AI Agent API Server

A RESTful API server built in Rust that provides AI-powered insights about my [portfolio website](https://about.calum.sh) and its underlying Kubernetes infrastructure. 

## Features

- **AI-Powered Chat**: Natural language interface for querying portfolio information and infrastructure metrics
- **Kubernetes Integration**: Real-time access to pod listings, namespaces, and node metrics
- **Portfolio Scraping**: Fetches content from portfolio website sections (About, Work, Projects, Contact)
- **Secure Authentication**: API key-based request authentication

## Prerequisites

- **Rust** (1.70+): Install from [rustup.rs](https://rustup.rs/)
- **OpenAI API Key**: Get one from [OpenAI](https://platform.openai.com/)
- **Redis**: Required at startup for tool call tracking. Install from [redis.io](https://redis.io/)
- **Kubernetes Cluster** (optional): Required for infrastructure monitoring features

## Quick Start

### Local Development

1. **Clone the repository**
   ```bash
   git clone https://github.com/ccrawford4/rust-agent.git
   cd rust-agent
   ```

2. **Create a secure key to access the chat API**
   ```bash
   openssl rand -base64 32
   ```

3. **Create a `.env` file** in the project root:
   ```env
   # Required
   OPENAI_API_KEY=your_openai_api_key_here
   CHAT_API_KEY=your_secure_api_key_for_authentication_from_step_2

   # Optional
   PRODUCTION_MODE=false
   KUBE_API_SERVER=https://localhost:6443
   KUBE_TOKEN=your_kubernetes_token_here

   # Optional Redis config
   REDIS_HOST=127.0.0.1
   REDIS_PORT=6379
   # REDIS_PASSWORD=your_redis_password_here

   # Backward-compatible alternative
   # REDIS_URL=redis://127.0.0.1:6379

   # Optional
   RUST_LOG=info
   ```

4. **Build and run**
   ```bash
   cargo build --release
   cargo run --release
   ```

   The server verifies Redis during startup and exits immediately if it cannot connect.

5. **Test the server**
   ```bash
   # Health check
   curl -X GET http://127.0.0.1:8080/ \
     -H "X-API-Key: your_secure_api_key_for_authentication"

   # Chat request (basic)
   curl -X POST http://127.0.0.1:8080/chat \
     -H "Content-Type: application/json" \
     -H "X-API-Key: your_secure_api_key_for_authentication" \
     -d '{
       "request_id": "550e8400-e29b-41d4-a716-446655440000",
       "prompt": "What is on Calum'\''s About page?",
       "chat_history": []
     }'

   # Chat request (with conversation history)
   curl -X POST http://127.0.0.1:8080/chat \
     -H "Content-Type: application/json" \
     -H "X-API-Key: your_secure_api_key_for_authentication" \
     -d '{
       "request_id": "550e8400-e29b-41d4-a716-446655440001",
       "prompt": "What technologies are mentioned?",
       "chat_history": [
         {"role": "user", "content": "Tell me about Calum'\''s projects"},
         {"role": "assistant", "content": "Calum has several projects involving Rust, Kubernetes..."}
       ]
     }'
   ```

### Production Deployment (Kubernetes)

The server is designed to run inside a Kubernetes cluster as a pod with appropriate RBAC permissions.

1. **Build Docker image**
   ```bash
   docker build -t ai-agent-api:latest .
   ```

2. **Apply Kubernetes manifests**
   ```bash
   # Apply RBAC permissions
   kubectl apply -f kubernetes/permissions.yaml

   # Deploy the application
   kubectl apply -f kubernetes/agent-deployment.yaml
   ```

3. **Configure environment**

   The production deployment uses:
   - **Service Account Tokens**: Automatically mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token`
   - **CA Certificates**: Mounted at `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`
   - **Environment Variables**: Set via ConfigMap/Secret

   Required environment variables for production:
   ```yaml
   env:
     - name: PRODUCTION_MODE
       value: "true"
     - name: OPENAI_API_KEY
       valueFrom:
         secretKeyRef:
           name: ai-agent-secrets
           key: openai-api-key
     - name: CHAT_API_KEY
       valueFrom:
         secretKeyRef:
           name: ai-agent-secrets
           key: chat-api-key
     - name: KUBE_API_SERVER
       value: "https://kubernetes.default.svc"
     - name: REDIS_HOST
       value: "ai-agent-api-redis"
     - name: REDIS_PORT
       value: "6379"
     - name: REDIS_PASSWORD
       valueFrom:
         secretKeyRef:
           name: ai-agent-api-secrets
           key: redis-password
   ```

   The application will not start unless the configured Redis instance is reachable at boot.

## API Documentation

### Endpoints

#### `GET /`
Health check endpoint.

**Response**
```json
{
  "healthy": true
}
```

#### `POST /chat`
Main chat endpoint for AI interactions.

Add `?async=true` to queue LLM processing in the background and return `200 OK` immediately. The request is marked as pending in Redis under `chat_response_{request_id}`, then overwritten with the completed or failed result when background processing finishes.

**Request Headers**
- `Content-Type: application/json`
- `X-API-Key: <your-api-key>`

**Request Body**
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "prompt": "Your question here",
  "chat_history": [
    {
      "role": "user",
      "content": "Previous user message"
    },
    {
      "role": "assistant",
      "content": "Previous assistant response"
    }
  ]
}
```

**Fields**
- `request_id` (string, required): Unique identifier for this request. Used for tracking tool calls in Redis. Typically a UUID (e.g., `550e8400-e29b-41d4-a716-446655440000`).
- `prompt` (string, required): The user's question or prompt
- `chat_history` (array, optional): Previous conversation messages for context. Each message must have `role` ("user" or "assistant") and `content` (string) fields.

**Response**
```
String response from the AI agent
```

When using `POST /chat?async=true`, the HTTP response returns immediately with an empty body. Poll for completion with:

```bash
curl "http://127.0.0.1:8080/chat/response?request_id=550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: your-chat-api-key"
```

**Status Codes**
- `200 OK`: Successful response
- `400 Bad Request`: Invalid JSON or malformed request (e.g., missing/empty `request_id`)
- `401 Unauthorized`: Missing API key
- `403 Forbidden`: Invalid API key
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: AI agent failure

#### `GET /chat/response`
Polls for the result of an async `POST /chat?async=true` request.

**Query Parameters**
- `request_id` (string, required): Request identifier originally sent to `POST /chat`

**Response**
```json
{
  "status": "pending",
  "timestamp": "2026-04-20T01:23:45Z"
}
```

```json
{
  "status": "completed",
  "response": "{\"response\":\"Your answer here\"}",
  "timestamp": "2026-04-20T01:23:52Z"
}
```

```json
{
  "status": "failed",
  "error": "Failed to generate response",
  "details": "provider error details",
  "timestamp": "2026-04-20T01:23:52Z"
}
```

**Status Codes**
- `200 OK`: Async response completed
- `202 Accepted`: Async request is still processing
- `400 Bad Request`: Missing or empty `request_id`
- `401 Unauthorized`: Missing API key
- `403 Forbidden`: Invalid API key
- `404 Not Found`: No async response exists for that `request_id`
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Async request failed or Redis read failed

#### `GET /api/tools`

Returns the Redis-backed tool call log for a given response identifier.

**Query Parameters**
- `response_id` (string, required): Identifier used to look up the Redis key for tool calls. This currently maps to the same value sent as `request_id` on `POST /chat`.

**Example**
```bash
curl "http://127.0.0.1:8080/api/tools?response_id=550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: your-chat-api-key"
```

**Response**
```json
{
  "response_id": "550e8400-e29b-41d4-a716-446655440000",
  "tools": [
    {
      "name": "web_search",
      "args": {
        "endpoint": "about"
      },
      "timestamp": "2025-02-15T10:30:45Z"
    }
  ]
}
```

**Status Codes**
- `200 OK`: Successful response
- `400 Bad Request`: Missing or empty `response_id`
- `401 Unauthorized`: Missing API key
- `403 Forbidden`: Invalid API key
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Redis read failure

## Tool Call Tracking (Redis)

Wrapped tool calls made by the AI agent are written to Redis in real-time. This allows you to monitor which wrapped tools are being invoked and with what arguments for each request.

**How it works:**
1. Each API request includes a `request_id` in the body
2. As the agent executes, wrapped tools write tool calls to Redis with the key format: `request:{request_id}:tool_calls`
3. Each tool call is stored as a JSON record containing the tool name, arguments, and timestamp
4. Tool calls are stored as a Redis list via `RPUSH`, where each list item is a JSON string containing `name`, `args`, and `timestamp`

**Example Redis data structure:**
```
Key: request:550e8400-e29b-41d4-a716-446655440000:tool_calls
Value: [
  {"name": "web_search", "args": {"endpoint": "about"}, "timestamp": "2025-02-15T10:30:45Z"}
]
```

**Retrieving tool calls:**
```bash
# Get all tool calls for a request
redis-cli LRANGE "request:550e8400-e29b-41d4-a716-446655440000:tool_calls" 0 -1
```

Or via the HTTP API:
```bash
curl "http://127.0.0.1:8080/api/tools?response_id=550e8400-e29b-41d4-a716-446655440000" \
  -H "X-API-Key: your-chat-api-key"
```

**Configuration:**
- Set `REDIS_HOST`, `REDIS_PORT`, and `REDIS_PASSWORD` to have the app build the Redis URL automatically
- `REDIS_URL` remains supported as a backward-compatible fallback
- The server validates Redis connectivity during startup with a connection check and `PING`
- If Redis is unavailable or authentication fails, the process exits and the server does not start
- At present, `web_search` is wrapped and logged; Kubernetes tools are not

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OPENAI_API_KEY` | Yes | - | OpenAI API key for GPT-5.1 model |
| `CHAT_API_KEY` | Yes | - | API key for authenticating requests to this server |
| `PRODUCTION_MODE` | No | `false` | Enables production mode (uses mounted K8s credentials) |
| `KUBE_API_SERVER` | No | `https://localhost:6443` | Kubernetes API server URL |
| `KUBE_TOKEN` | No (dev only) | - | Kubernetes bearer token (development mode only) |
| `REDIS_HOST` | No | - | Redis host used to build the connection URL when set with `REDIS_PORT` and `REDIS_PASSWORD` |
| `REDIS_PORT` | No | - | Redis port used to build the connection URL when set with `REDIS_HOST` and `REDIS_PASSWORD` |
| `REDIS_PASSWORD` | No | - | Redis password used to build the connection URL when set with `REDIS_HOST` and `REDIS_PORT` |
| `REDIS_URL` | No | `redis://127.0.0.1:6379` | Backward-compatible Redis connection URL fallback |
| `RUST_LOG` | No | `info` | Log level (`error`, `warn`, `info`, `debug`, `trace`) |

### Logging

The server uses structured logging with the `tracing` framework. Control verbosity with the `RUST_LOG` environment variable:

```bash
# Show only errors and warnings
RUST_LOG=warn cargo run

# Show detailed debug information
RUST_LOG=debug cargo run

# Show everything including trace logs
RUST_LOG=trace cargo run

# Filter by module
RUST_LOG=sql_agent::agent=debug,sql_agent::kube=trace cargo run
```

**Log Levels by Component**
- `error`: Critical failures (server startup, OpenAI client errors, K8s connection failures)
- `warn`: Non-critical issues (missing API keys, invalid requests, self-signed certificates)
- `info`: Important events (server start, successful requests, tool invocations)
- `debug`: Detailed flow (request parsing, API responses, data transformations)

## Architecture

### Project Structure

```
src/
├── main.rs              # Application entry point
├── environment.rs       # Configuration management
├── server/              # HTTP server implementation
│   ├── mod.rs          # TCP-based HTTP/1.1 server
│   └── types.rs        # Request/Response types
├── agent/               # AI agent module
│   ├── mod.rs          # Agent initialization and chat handler
│   └── tools/          # Portfolio API tools
│       ├── mod.rs
│       ├── portfolio_api_search.rs
│       └── wrapped.rs
└── kube/                # Kubernetes integration
    ├── mod.rs          # KubeAgent HTTP client
    ├── error.rs        # Custom error types
    ├── types/          # Kubernetes API response types
    │   ├── mod.rs
    │   ├── pod.rs
    │   ├── metrics.rs
    │   └── namespaces.rs
    └── tools/          # Kubernetes tools for AI agent
        ├── mod.rs
        ├── pods.rs     # ListPodsTool
        ├── namespaces.rs # ListNamespacesTool
        └── metrics.rs  # NodeMetricsTool
```

### How It Works

1. **Server** receives HTTP requests on port 8080
2. **Request parsing** extracts method, path, API key, and body
3. **Authentication** validates the API key from the `X-API-Key` header
4. **Routing** directs to appropriate handler (`/` or `/chat`)
5. **AI Agent** processes the chat request:
   - Receives user prompt and chat history
   - Decides which tools to invoke (portfolio API, Kubernetes queries)
   - Makes up to 2 rounds of tool calls
   - Generates natural language response
6. **Response** is sent back to client

### Tools Available to AI Agent

1. **PortfolioAPISearch**: Fetches structured JSON from `about.calum.sh`
   - Supports: `/api/about`, `/api/work`, `/api/projects`, `/api/contact`
   - Returns API responses directly from the portfolio host

2. **ListPodsTool**: Queries Kubernetes pods
   - Optional namespace filtering
   - Configurable result limit

3. **ListNamespacesTool**: Lists all cluster namespaces

4. **NodeMetricsTool**: Gets node CPU and memory metrics
   - Requires metrics-server addon
   - Calculates usage percentages
   - Fetches data from both core API and metrics API in parallel

## Development

### Running Tests
```bash
cargo test
```

### Building for Release
```bash
cargo build --release
# Binary located at: target/release/sql-agent
```

### Code Formatting
```bash
cargo fmt
```

### Linting
```bash
cargo clippy
```

## Security Considerations

- **API Key Authentication**: All requests must include a valid `X-API-Key` header
- **Certificate Validation**: Production mode uses CA certificates for secure K8s communication
- **Development Mode**: Accepts self-signed certificates (never use in production)
- **Secrets Management**: Use Kubernetes Secrets for sensitive environment variables
- **RBAC Permissions**: Ensure the service account has minimal required permissions

## Troubleshooting

### Server won't start
- Check that port 8080 is not already in use
- Verify `OPENAI_API_KEY` is set correctly
- Check logs with `RUST_LOG=debug` for detailed error messages

### Kubernetes connection failed
- Verify `KUBE_API_SERVER` URL is correct
- Ensure `KUBE_TOKEN` is valid (development mode)
- Check that service account has proper RBAC permissions (production mode)
- Confirm metrics-server is installed for node metrics

### AI agent errors
- Verify OpenAI API key is valid and has credits
- Check network connectivity to OpenAI API
- Review error logs for specific OpenAI error messages

### 401/403 responses
- Ensure `X-API-Key` header is included in request
- Verify the API key matches `CHAT_API_KEY` environment variable
