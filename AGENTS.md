# AGENTS.md

Guidance for AI-assisted development in this repository.

## Project Context

This project is a production-oriented WhatsApp webhook messaging service. External systems call a REST API, requests are queued, and background workers send WhatsApp messages through the Meta WhatsApp Cloud API.

Primary stack:

- Go
- Gin
- PostgreSQL
- Redis
- Asynq
- Docker
- GORM

## Architecture Standards

Use clean architecture principles throughout the codebase.

- Keep HTTP handlers thin.
- Put business logic in service packages.
- Use repositories for persistence concerns only.
- Use dependency injection for handlers, services, repositories, queue clients, and providers.
- Prefer interface-driven design at package boundaries.
- Keep external provider integrations behind interfaces.
- Avoid framework-specific types outside the HTTP boundary unless necessary.

Recommended responsibility boundaries:

- **Handlers**: authentication context, request binding, validation, DTO mapping, response formatting.
- **Services**: business rules, orchestration, transactions, queue dispatch decisions, provider workflow decisions.
- **Repositories**: database reads and writes only.
- **Queue**: task definitions, enqueueing, worker handlers, retry configuration.
- **Providers**: Meta WhatsApp Cloud API client implementation behind an interface.

## Coding Standards

- Use structured JSON logging.
- Load configuration from environment variables.
- Propagate `context.Context` through handlers, services, repositories, queue handlers, and provider clients.
- Do not hardcode secrets, credentials, tokens, DSNs, or API keys.
- Do not introduce global mutable state.
- Wrap errors with useful context using Go error wrapping.
- Return proper HTTP status codes for validation, authentication, authorization, conflict, rate limit, provider, and server errors.
- Validate all inbound requests.
- Use DTOs for API request and response payloads.
- Keep domain models separate from API DTOs.
- Use retry support for external APIs where safe and idempotent.
- Prefer small, focused interfaces owned by the consumer package.

## Security Requirements

- Protect API endpoints with API key authentication.
- Apply rate limiting to inbound webhook routes.
- Validate all input before calling services.
- Enforce request timeout protection.
- Never log secrets, access tokens, API keys, or full authorization headers.
- Treat external request payloads as untrusted.
- Prefer least-privilege database and infrastructure credentials.

## Testing Standards

- Unit tests are required for services, repositories, queue handlers, and provider clients.
- Prefer table-driven tests for business logic and validation behavior.
- Mock external providers through interfaces.
- Test success paths, validation failures, retryable failures, permanent failures, and context cancellation.
- Avoid tests that depend on real Meta API credentials.
- Use integration tests for PostgreSQL, Redis, and Asynq behavior when practical.

## Development Rules

- Keep handlers thin.
- Do not put business logic in controllers or HTTP handlers.
- Repository layer must handle persistence only.
- Service layer must handle business logic and orchestration.
- External provider integrations must use interfaces.
- Do not bypass services from handlers to write directly to repositories.
- Do not leak GORM models directly through API responses.
- Do not enqueue jobs before required request validation and persistence have succeeded.
- Ensure worker jobs are idempotent or protected by deduplication where possible.
- Preserve clear package boundaries and avoid circular dependencies.

## Operational Expectations

- Log request IDs, correlation IDs, queue task IDs, message IDs, and provider response IDs when available.
- Include enough error context for debugging without exposing sensitive data.
- Make retry behavior explicit and observable.
- Track message lifecycle states consistently.
- Prefer graceful shutdown for HTTP servers, workers, database connections, and Redis clients.

## AI Assistant Instructions

When modifying this repository:

- Follow the existing project structure and naming conventions.
- Keep changes focused on the requested behavior.
- Add or update tests for behavior changes.
- Prefer simple, idiomatic Go over unnecessary abstraction.
- Update documentation when commands, configuration, endpoints, or architecture change.
- Do not introduce new dependencies unless they are justified by the task.
- Call out any assumptions or missing project context in the final response.
