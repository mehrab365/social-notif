package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"social-notif/internal/worker"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type MessageQueue interface {
	EnqueueMessageDelivery(ctx context.Context, messageID uuid.UUID) error
}

type AsynqMessageQueue struct {
	client *asynq.Client
}

type MessageDeliveryPayload struct {
	MessageID string `json:"message_id"`
}

func NewAsynqMessageQueue(client *asynq.Client) MessageQueue {
	return &AsynqMessageQueue{client: client}
}

func (q *AsynqMessageQueue) EnqueueMessageDelivery(ctx context.Context, messageID uuid.UUID) error {
	payload, err := json.Marshal(MessageDeliveryPayload{MessageID: messageID.String()})
	if err != nil {
		return fmt.Errorf("marshal message delivery payload: %w", err)
	}

	task := asynq.NewTask(worker.TaskDeliverWhatsAppMessage, payload)
	if _, err := q.client.EnqueueContext(ctx, task); err != nil {
		return fmt.Errorf("enqueue message delivery task: %w", err)
	}

	return nil
}
