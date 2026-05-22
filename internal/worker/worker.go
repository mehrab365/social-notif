package worker

import (
	"context"
	"fmt"

	"social-notif/internal/config"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const TaskDeliverWhatsAppMessage = "whatsapp:deliver_message"

type Dependencies struct {
	Config *config.Config
	Logger *zap.Logger
	DB     *gorm.DB
}

func RegisterHandlers(mux *asynq.ServeMux, deps Dependencies) {
	handler := NewDeliveryHandler(deps)
	mux.HandleFunc(TaskDeliverWhatsAppMessage, handler.Handle)
}

type DeliveryHandler struct {
	deps Dependencies
}

func NewDeliveryHandler(deps Dependencies) *DeliveryHandler {
	return &DeliveryHandler{deps: deps}
}

func (h *DeliveryHandler) Handle(ctx context.Context, task *asynq.Task) error {
	h.deps.Logger.Info("received delivery task",
		zap.String("task_type", task.Type()),
		zap.ByteString("payload", task.Payload()),
	)

	return fmt.Errorf("delivery task handler not implemented")
}

type ErrorHandler struct {
	logger *zap.Logger
}

func NewErrorHandler(logger *zap.Logger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

func (h *ErrorHandler) HandleError(ctx context.Context, task *asynq.Task, err error) {
	h.logger.Error("worker task failed",
		zap.String("task_type", task.Type()),
		zap.ByteString("payload", task.Payload()),
		zap.Error(err),
	)
}
