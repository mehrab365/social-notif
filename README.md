# WhatsApp Webhook Messaging Service

A production-ready backend service for accepting outbound WhatsApp message requests through a REST webhook, queueing them for reliable processing, and delivering them asynchronously through the Meta WhatsApp Cloud API.

The service is designed for external systems that need a simple HTTP integration point while keeping message delivery resilient, observable, and decoupled from request latency.

## Project Overview

External applications submit message requests to the service through a REST API. The API validates and persists the request, enqueues a background job, and immediately returns an acknowledgement. Worker processes consume queued jobs and send WhatsApp messages using the Meta WhatsApp Cloud API.

This architecture keeps webhook response times low, protects upstream systems from transient Meta API failures, and allows message throughput to scale independently from the API layer.

## Architecture Summary

```text
External System
      |
      v
REST Webhook API (Gin)
      |
      v
PostgreSQL  <---->  Message Records / Delivery State
      |
      v
Redis + Asynq  ---->  Background Workers
                         |
                         v
                 Meta WhatsApp Cloud API
```

Key responsibilities:

- Accept message requests from trusted external systems.
- Validate recipient, template, payload, and metadata.
- Persist message lifecycle state in PostgreSQL.
- Queue delivery work in Redis using Asynq.
- Send WhatsApp messages asynchronously from workers.
- Track delivery attempts, failures, and provider responses.

## Technology Stack

- **Go**: service implementation
- **Gin**: HTTP routing and middleware
- **PostgreSQL**: durable application data and message state
- **Redis**: queue backend for asynchronous jobs
- **Asynq**: background task processing
- **GORM**: database ORM and migrations support
- **Docker**: local and production containerization
- **Meta WhatsApp Cloud API**: WhatsApp message delivery provider

## Local Development Setup

### Prerequisites

- Go 1.22 or newer
- Docker and Docker Compose
- PostgreSQL
- Redis
- Meta WhatsApp Cloud API credentials

### Setup

1. Clone the repository:

```bash
git clone <repository-url>
cd social-notif
```

2. Create a local environment file:

```bash
cp .env.example .env
```

3. Update `.env` with local database, Redis, and Meta API credentials.

4. Install dependencies:

```bash
go mod download
```

5. Run database migrations:

```bash
go run ./cmd/migrate
```

6. Start the API server:

```bash
go run ./cmd/api
```

7. Start the worker process in a separate terminal:

```bash
go run ./cmd/worker
```

## Docker Usage

Start the full local stack:

```bash
docker compose up --build
```

Run services in the background:

```bash
docker compose up -d --build
```

View logs:

```bash
docker compose logs -f api worker
```

Stop the stack:

```bash
docker compose down
```

For production deployments, build immutable images and inject configuration through environment variables or a secrets manager. Do not bake credentials into the image.

## Environment Variables

| Variable | Description | Example |
| --- | --- | --- |
| `APP_NAME` | Service name included in logs | `social-notif` |
| `APP_ENV` | Runtime environment | `local`, `staging`, `production` |
| `APP_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout | `15s` |
| `HTTP_PORT` | HTTP server port | `8080` |
| `HTTP_READ_HEADER_TIMEOUT` | HTTP read-header timeout | `5s` |
| `HTTP_READ_TIMEOUT` | HTTP read timeout | `15s` |
| `HTTP_WRITE_TIMEOUT` | HTTP write timeout | `15s` |
| `HTTP_IDLE_TIMEOUT` | HTTP idle timeout | `60s` |
| `HTTP_REQUEST_TIMEOUT` | Per-request timeout middleware value | `10s` |
| `HTTP_TRUSTED_PROXIES` | Comma-separated trusted proxy IPs/CIDRs | `10.0.0.0/8` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:password@localhost:5432/social_notif?sslmode=disable` |
| `DB_MAX_OPEN_CONNS` | PostgreSQL max open connections | `25` |
| `DB_MAX_IDLE_CONNS` | PostgreSQL max idle connections | `10` |
| `DB_CONN_MAX_LIFETIME` | PostgreSQL connection max lifetime | `30m` |
| `REDIS_ADDR` | Redis host and port | `localhost:6379` |
| `REDIS_PASSWORD` | Redis password, if enabled | `change-me` |
| `REDIS_DB` | Redis database index | `0` |
| `API_KEY` | Shared API key for authenticating inbound API calls | `change-me` |
| `RATE_LIMIT_PER_MIN` | Per-client inbound request limit per minute | `120` |
| `MAX_REQUEST_BYTES` | Maximum inbound request body size | `1048576` |
| `WHATSAPP_ACCESS_TOKEN` | Meta WhatsApp Cloud API access token | `EAAG...` |
| `WHATSAPP_PHONE_NUMBER_ID` | Meta phone number ID | `1234567890` |
| `WHATSAPP_BUSINESS_ACCOUNT_ID` | Meta WhatsApp Business Account ID | `1234567890` |
| `WHATSAPP_API_VERSION` | Meta Graph API version | `v20.0` |
| `WHATSAPP_BASE_URL` | Meta Graph API base URL | `https://graph.facebook.com` |
| `WHATSAPP_TIMEOUT` | Meta Graph API request timeout | `10s` |
| `QUEUE_CONCURRENCY` | Number of concurrent worker jobs | `10` |
| `QUEUE_DEFAULT_PRIORITY` | Asynq default queue priority | `1` |
| `LOG_LEVEL` | Application log level | `info` |

## API Example

### Send WhatsApp Message

```http
POST /api/v1/messages/whatsapp
Authorization: Bearer <API_KEY>
Content-Type: application/json
```

Request body:

```json
{
  "recipient": "+15551234567",
  "type": "template",
  "template": {
    "name": "order_update",
    "language": "en_US",
    "parameters": {
      "customer_name": "Jane Doe",
      "order_id": "ORD-10001",
      "status": "shipped"
    }
  },
  "metadata": {
    "source": "order-service",
    "correlation_id": "01J8Z9V6W8B4K2QG7H6N3E2R1T"
  }
}
```

Successful response:

```json
{
  "message_id": "msg_01J8Z9Y7M6R4T8K2BQ9P1X3H5N",
  "status": "queued"
}
```

The API only confirms that the request was accepted and queued. Final delivery status should be tracked through persisted message state, provider callbacks, or a status endpoint if enabled.

## Folder Structure

Recommended project layout:

```text
.
├── cmd
│   ├── api              # HTTP API entrypoint
│   ├── worker           # Asynq worker entrypoint
│   └── migrate          # Database migration entrypoint
├── internal
│   ├── config           # Environment and application configuration
│   ├── database         # PostgreSQL connection and migration helpers
│   ├── http             # Gin routes, handlers, middleware, request validation
│   ├── messaging        # Message domain logic and delivery orchestration
│   ├── queue            # Asynq client, task definitions, handlers
│   ├── repository       # GORM repositories and persistence models
│   └── whatsapp         # Meta WhatsApp Cloud API client
├── migrations           # SQL or GORM-backed database migrations
├── docker-compose.yml   # Local development stack
├── Dockerfile           # Production image build
├── .env.example         # Local configuration template
└── README.md
```

## Operational Considerations

- Use idempotency keys or correlation IDs to prevent duplicate message dispatch.
- Store provider request and response metadata for auditability.
- Configure retry policies with exponential backoff for transient provider failures.
- Keep permanent failures visible through structured logs, metrics, and database state.
- Protect webhook endpoints with authentication, request validation, and rate limits.
- Rotate Meta API credentials and service secrets through a managed secret store.
- Monitor queue depth, worker latency, delivery success rate, retry count, and provider error rates.

## Future Improvements

- Delivery status endpoint for external systems.
- Provider webhook handling for WhatsApp delivery receipts.
- Dead-letter queue dashboard and replay tooling.
- Multi-tenant configuration for multiple WhatsApp Business Accounts.
- Template registry and validation against approved Meta templates.
- OpenTelemetry tracing across API, queue, worker, and provider calls.
- Prometheus metrics and production dashboards.
- CI pipeline with linting, tests, security scanning, and container image publishing.
- Kubernetes deployment manifests or Helm chart.
