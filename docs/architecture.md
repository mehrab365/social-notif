# Architecture

## System Overview

The WhatsApp webhook messaging service provides a REST API for external systems to request outbound WhatsApp messages. The API accepts and validates requests, persists message state in PostgreSQL, and enqueues delivery work into Redis. Background workers consume queued jobs and send messages through the Meta WhatsApp Cloud API.

The system is designed around asynchronous processing. API callers receive a fast acknowledgement when a request is accepted, while message delivery, retries, and provider communication happen outside the request path.

Primary goals:

- Decouple external webhook latency from WhatsApp provider latency.
- Preserve durable message state for auditability and recovery.
- Support retries for transient provider and infrastructure failures.
- Scale API and worker workloads independently.
- Keep provider-specific behavior isolated behind interfaces.

## High-Level Architecture

```text
External Systems
      |
      v
REST Webhook API
      |
      v
Message Service
      |
      +---------> PostgreSQL
      |
      +---------> Redis Queue
                    |
                    v
              Asynq Workers
                    |
                    v
          Meta WhatsApp Cloud API
```

## Request Lifecycle

1. An external system sends a message request to the REST webhook API.
2. API middleware authenticates the request using an API key.
3. Request timeout and rate limiting protections are applied.
4. The HTTP handler binds the request body into an API DTO.
5. The handler validates required fields, message type, recipient format, and payload shape.
6. The handler calls the message service with `context.Context`.
7. The service applies business rules and creates a message record in PostgreSQL.
8. The service enqueues a delivery task in Redis through Asynq.
9. The API returns a `queued` response with the internal message ID.
10. A worker consumes the queued task.
11. The worker loads message state from PostgreSQL.
12. The worker calls the WhatsApp provider interface.
13. The Meta WhatsApp Cloud API client sends the message.
14. The worker updates delivery status, provider response metadata, and attempt count.
15. Failed retryable deliveries are retried according to queue policy.

## Component Responsibilities

### REST API

- Exposes webhook endpoints for external systems.
- Handles authentication, rate limiting, request timeout, and request validation.
- Converts HTTP DTOs into service input models.
- Returns appropriate HTTP status codes and response DTOs.
- Does not contain business logic or persistence logic.

### Message Service

- Owns message business logic and workflow orchestration.
- Validates business-level invariants.
- Coordinates persistence and queue dispatch.
- Ensures message records are created before delivery jobs are queued.
- Defines idempotency and duplicate-handling behavior.

### Repository Layer

- Encapsulates PostgreSQL access through GORM.
- Reads and writes message records, delivery attempts, and provider metadata.
- Does not contain business decisions.
- Accepts `context.Context` for all database operations.

### Queue Layer

- Defines Asynq task names and payload schemas.
- Enqueues delivery jobs.
- Implements worker handlers.
- Configures retry behavior, timeout behavior, and queue priority where required.

### Worker Processes

- Consume message delivery tasks from Redis.
- Load current message state before delivery.
- Call the provider interface.
- Persist delivery outcomes.
- Return retryable errors for transient failures and terminal errors for permanent failures.

### WhatsApp Provider Integration

- Implements the Meta WhatsApp Cloud API client.
- Stays behind a provider interface used by services and workers.
- Handles request construction, provider responses, provider error mapping, and provider timeouts.
- Does not leak provider-specific response structures into API DTOs.

## Folder Structure Explanation

Recommended layout:

```text
.
├── cmd
│   ├── api
│   ├── worker
│   └── migrate
├── internal
│   ├── config
│   ├── database
│   ├── http
│   ├── messaging
│   ├── queue
│   ├── repository
│   └── whatsapp
├── migrations
├── docs
├── docker-compose.yml
├── Dockerfile
└── README.md
```

Directory responsibilities:

- `cmd/api`: starts the Gin HTTP server and wires dependencies.
- `cmd/worker`: starts Asynq workers and wires worker dependencies.
- `cmd/migrate`: runs database migrations.
- `internal/config`: loads and validates environment-based configuration.
- `internal/database`: manages PostgreSQL connections and migration helpers.
- `internal/http`: contains routes, handlers, middleware, API DTOs, and request validation.
- `internal/messaging`: contains message use cases, domain models, and service interfaces.
- `internal/queue`: contains Asynq task definitions, enqueueing logic, and worker handlers.
- `internal/repository`: contains GORM persistence models and repository implementations.
- `internal/whatsapp`: contains the Meta WhatsApp Cloud API client implementation.
- `migrations`: contains schema migrations.
- `docs`: contains architecture and operational documentation.

## Database Design

PostgreSQL is the system of record for message state, delivery attempts, and provider metadata.

Recommended core tables:

### `messages`

Stores the canonical message request and current lifecycle state.

| Column | Purpose |
| --- | --- |
| `id` | Internal message identifier |
| `recipient` | WhatsApp recipient phone number |
| `message_type` | Template, text, media, or other supported type |
| `payload` | Normalized message payload as JSON |
| `status` | Current lifecycle state |
| `source` | Calling system or integration name |
| `correlation_id` | External correlation or idempotency identifier |
| `provider_message_id` | Message ID returned by Meta, when available |
| `provider_response` | Sanitized provider response metadata |
| `last_error` | Last sanitized error message |
| `created_at` | Creation timestamp |
| `updated_at` | Last update timestamp |

Recommended statuses:

- `queued`
- `processing`
- `sent`
- `failed_retryable`
- `failed_permanent`
- `cancelled`

### `message_attempts`

Stores delivery attempt history for auditing and debugging.

| Column | Purpose |
| --- | --- |
| `id` | Attempt identifier |
| `message_id` | Associated message |
| `attempt_number` | Delivery attempt count |
| `status` | Attempt result |
| `provider_status_code` | HTTP status from Meta, when available |
| `provider_error_code` | Provider error code, when available |
| `error_message` | Sanitized error message |
| `started_at` | Attempt start timestamp |
| `completed_at` | Attempt completion timestamp |

Database design considerations:

- Use indexes on `status`, `correlation_id`, `created_at`, and `provider_message_id`.
- Enforce uniqueness on idempotency keys when required.
- Store provider responses in sanitized form only.
- Avoid storing access tokens or raw authorization headers.

## Queue Design

Redis is used as the Asynq backend for asynchronous delivery jobs.

Recommended task:

```text
whatsapp:deliver_message
```

Recommended task payload:

```json
{
  "message_id": "msg_01J8Z9Y7M6R4T8K2BQ9P1X3H5N",
  "correlation_id": "01J8Z9V6W8B4K2QG7H6N3E2R1T"
}
```

Queue design principles:

- Keep task payloads small.
- Store authoritative message data in PostgreSQL, not only in Redis.
- Make worker execution idempotent where possible.
- Use task timeouts to prevent stuck delivery attempts.
- Use queue priority if different message classes require different service levels.
- Use dead-letter or archived failed tasks for operational review.

## Retry Strategy

Retries should be applied only when the failure is likely transient.

Retryable examples:

- Meta API `5xx` responses.
- Network timeouts.
- Temporary DNS or connection failures.
- Redis or database transient errors.
- Provider rate limits, when retry-after behavior is respected.

Non-retryable examples:

- Invalid recipient format.
- Invalid or unapproved template.
- Authentication or authorization failure caused by invalid credentials.
- Payload validation failure.
- Permanent provider rejection.

Recommended policy:

- Use exponential backoff with jitter.
- Cap maximum retry attempts.
- Persist every attempt in `message_attempts`.
- Mark messages as `failed_permanent` after retry exhaustion.
- Keep retry classification explicit in provider error mapping.
- Use idempotency keys or message state checks to avoid duplicate delivery where possible.

## Logging Strategy

Use structured JSON logging across API and worker processes.

Required log fields where available:

- `request_id`
- `correlation_id`
- `message_id`
- `task_id`
- `recipient_hash`
- `provider_message_id`
- `status`
- `attempt`
- `duration_ms`
- `error`

Logging requirements:

- Do not log secrets, API keys, access tokens, or full authorization headers.
- Avoid logging full phone numbers; prefer masked values or hashes.
- Log validation failures at an appropriate level without exposing sensitive payloads.
- Log retryable provider failures with provider error classification.
- Log permanent failures with enough context for operational triage.
- Ensure logs are consistent between API and workers.

## Security Considerations

The service accepts requests from external systems and sends messages through a privileged provider account. Security controls are required at every boundary.

Required controls:

- API key authentication for webhook endpoints.
- Rate limiting for inbound API traffic.
- Request size limits.
- Request timeout middleware.
- Strict input validation.
- Environment-based configuration.
- Secret management outside source control.
- TLS termination at the edge or load balancer.
- Principle-of-least-privilege credentials for PostgreSQL and Redis.
- Sanitized logging.

Additional recommendations:

- Rotate API keys and Meta access tokens regularly.
- Use separate credentials per environment.
- Restrict inbound traffic by network policy where possible.
- Store only necessary provider response metadata.
- Consider request signing for high-trust integrations.

## Deployment Overview

The service should be deployed as separate API and worker processes using the same application image with different entrypoints.

Typical production deployment:

```text
Load Balancer
      |
      v
API Containers
      |
      +------ PostgreSQL
      |
      +------ Redis
                 |
                 v
          Worker Containers
                 |
                 v
       Meta WhatsApp Cloud API
```

Deployment expectations:

- API and worker containers are independently scalable.
- Configuration is injected through environment variables.
- Secrets are provided through a secret manager.
- Database migrations are run as a controlled deployment step.
- Health checks are exposed for API and worker readiness.
- Graceful shutdown is implemented for HTTP servers and workers.
- Logs are shipped to centralized logging infrastructure.
- Metrics and alerts cover API latency, queue depth, worker failures, retry counts, and provider error rates.

## Scalability Considerations

The architecture supports horizontal scaling by separating request intake from message delivery.

API scalability:

- Run multiple API replicas behind a load balancer.
- Keep handlers stateless.
- Use PostgreSQL and Redis as shared backing services.
- Apply rate limits per API key or source system.

Worker scalability:

- Scale worker replicas based on queue depth and delivery latency.
- Tune Asynq concurrency per worker instance.
- Respect Meta WhatsApp Cloud API rate limits.
- Use backpressure when provider limits or downstream failures increase.

Database scalability:

- Index common query patterns.
- Keep message payloads reasonably sized.
- Archive old message and attempt records according to retention policy.
- Use read replicas only for read-heavy operational views, not delivery state decisions.

Queue scalability:

- Monitor Redis memory usage and queue depth.
- Keep task payloads compact.
- Use queue priorities for differentiated workloads.
- Configure dead-letter handling for failed jobs.

Provider scalability:

- Centralize provider rate-limit handling.
- Classify provider errors consistently.
- Avoid uncontrolled retry storms.
- Track throughput, rejection rates, and latency by provider response code.
