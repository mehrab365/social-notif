package observability

import (
	"context"
	"time"
)

type Metrics interface {
	RecordHTTPRequest(ctx context.Context, method, path string, status int, duration time.Duration)
	RecordQueueEnqueue(ctx context.Context, taskType string)
	RecordWorkerTask(ctx context.Context, taskType string, status string, duration time.Duration)
	RecordProviderRequest(ctx context.Context, provider string, status string, duration time.Duration)
}

type NoopMetrics struct{}

func (NoopMetrics) RecordHTTPRequest(ctx context.Context, method, path string, status int, duration time.Duration) {
}

func (NoopMetrics) RecordQueueEnqueue(ctx context.Context, taskType string) {
}

func (NoopMetrics) RecordWorkerTask(ctx context.Context, taskType string, status string, duration time.Duration) {
}

func (NoopMetrics) RecordProviderRequest(ctx context.Context, provider string, status string, duration time.Duration) {
}
