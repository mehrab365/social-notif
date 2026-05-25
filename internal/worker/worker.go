package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"social-notif/internal/apperror"
	"social-notif/internal/config"
	"social-notif/internal/domain"
	"social-notif/internal/provider"
	"social-notif/internal/repository"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ProviderFactory interface {
	NewClient(accessToken, phoneNumberID string) provider.WhatsAppProvider
}

type DefaultProviderFactory struct {
	BaseURL    string
	APIVersion string
}

func (f *DefaultProviderFactory) NewClient(accessToken, phoneNumberID string) provider.WhatsAppProvider {
	return provider.NewMetaWhatsAppClientFromConfig(f.BaseURL, f.APIVersion, phoneNumberID, accessToken)
}

const TaskDeliverWhatsAppMessage = "whatsapp:deliver_message"

type MessageDeliveryPayload struct {
	MessageID string `json:"message_id"`
}

type Dependencies struct {
	Config          *config.Config
	Logger          *zap.Logger
	DB              *gorm.DB
	MessageRepo     repository.MessageRepository
	ShopRepo        repository.ShopRepository
	Provider        provider.WhatsAppProvider
	ProviderFactory ProviderFactory
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
	var payload MessageDeliveryPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	messageID, err := uuid.Parse(payload.MessageID)
	if err != nil {
		return fmt.Errorf("parse message id: %w", err)
	}

	logger := h.deps.Logger.With(
		zap.String("message_id", payload.MessageID),
		zap.String("task_type", task.Type()),
	)

	msg, err := h.deps.MessageRepo.GetByID(ctx, messageID)
	if err != nil {
		if errors.Is(err, repository.ErrMessageNotFound) {
			logger.Warn("message not found, skipping delivery")
			return asynq.SkipRetry
		}
		return fmt.Errorf("get message: %w", err)
	}

	logger.Info("processing message delivery",
		zap.String("status", string(msg.Status)),
		zap.Int("retry_count", msg.RetryCount),
	)

	if err := h.deps.MessageRepo.UpdateStatus(ctx, msg.ID, domain.MessageStatusProcessing); err != nil {
		logger.Error("failed to update status to processing", zap.Error(err))
	}

	if err := h.deps.MessageRepo.IncrementRetryCount(ctx, msg.ID); err != nil {
		logger.Error("failed to increment retry count", zap.Error(err))
	}

	p := h.resolveProvider(ctx, msg, logger)
	resp, err := p.SendMessage(ctx, provider.SendMessageRequest{
		PhoneNumber:      msg.PhoneNumber,
		Body:             msg.Body,
		TemplateName:     msg.TemplateName,
		TemplateLanguage: msg.TemplateLanguage,
		TemplateParams:   msg.TemplateParams,
	})
	if err != nil {
		status := classifyProviderError(err)
		providerErrJSON, _ := json.Marshal(map[string]string{"error": err.Error()})

		if updateErr := h.deps.MessageRepo.RecordDeliveryAttempt(ctx, msg.ID, status, providerErrJSON); updateErr != nil {
			logger.Error("failed to record delivery failure", zap.Error(updateErr))
		}

		logger.Error("message delivery failed",
			zap.String("delivery_status", string(status)),
			zap.Error(err),
		)

		if status == domain.MessageStatusFailedPermanent {
			return asynq.SkipRetry
		}
		return fmt.Errorf("send message: %w", err)
	}

	respJSON, _ := json.Marshal(resp)
	if err := h.deps.MessageRepo.RecordDeliveryAttempt(ctx, msg.ID, domain.MessageStatusSent, respJSON); err != nil {
		logger.Error("failed to record delivery success", zap.Error(err))
	}

	logger.Info("message delivered successfully",
		zap.String("provider_message_id", resp.MessageID),
	)
	return nil
}

func (h *DeliveryHandler) resolveProvider(ctx context.Context, msg *domain.Message, logger *zap.Logger) provider.WhatsAppProvider {
	if msg.ShopID == "" {
		return h.deps.Provider
	}

	shop, err := h.deps.ShopRepo.GetByID(ctx, msg.ShopID)
	if err != nil {
		logger.Warn("failed to look up shop for message, using default provider",
			zap.String("shop_id", msg.ShopID),
			zap.Error(err),
		)
		return h.deps.Provider
	}

	if shop.WhatsAppAccessToken == "" || shop.WhatsAppPhoneNumberID == "" {
		logger.Warn("shop has no whatsapp credentials, using default provider",
			zap.String("shop_id", msg.ShopID),
			zap.String("shop_domain", shop.ShopDomain),
		)
		return h.deps.Provider
	}

	logger.Info("using per-shop whatsapp provider",
		zap.String("shop_id", msg.ShopID),
		zap.String("shop_domain", shop.ShopDomain),
	)
	return h.deps.ProviderFactory.NewClient(shop.WhatsAppAccessToken, shop.WhatsAppPhoneNumberID)
}

func classifyProviderError(err error) domain.MessageStatus {
	if errors.Is(err, apperror.ErrProviderPermanent) {
		return domain.MessageStatusFailedPermanent
	}
	return domain.MessageStatusFailedRetryable
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
