# Production Readiness Notes

## Rate Limiting

The current rate limiter is process-local and suitable for local development or a single API instance. Multi-replica production deployments should use one of the following:

- API gateway or load balancer rate limiting.
- Redis-backed distributed rate limiting.
- Dedicated service-mesh or edge-policy enforcement.

Rate limits should be keyed by API key or source system rather than only by client IP.

## Observability

The codebase includes an `observability.Metrics` interface and a no-op implementation. Before production rollout, wire this interface to Prometheus, OpenTelemetry, or the organization's standard telemetry backend.

Minimum recommended metrics:

- HTTP request count and latency.
- HTTP error count by status code.
- Queue enqueue count and failure count.
- Worker task duration and failure count.
- Provider request latency and error count.
- Message delivery success, retry, and permanent failure counts.
