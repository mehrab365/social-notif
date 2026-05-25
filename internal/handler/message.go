package handler

import (
	"errors"
	"net/http"

	"social-notif/internal/apperror"
	"social-notif/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type MessageHandler struct {
	svc    service.MessageService
	logger *zap.Logger
}

func NewMessageHandler(svc service.MessageService, logger *zap.Logger) *MessageHandler {
	return &MessageHandler{svc: svc, logger: logger}
}

func (h *MessageHandler) CreateWhatsApp(c *gin.Context) {
	var req CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Debug("invalid request body", zap.Error(err))
		RespondError(c, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	result, err := h.svc.EnqueueWhatsAppMessage(c.Request.Context(), service.EnqueueWhatsAppMessageInput{
		PhoneNumber:        req.PhoneNumber,
		Body:               req.Body,
		TemplateName:       req.TemplateName,
		TemplateLanguage:   req.TemplateLanguage,
		TemplateBodyParams: req.TemplateBodyParams,
	})
	if err != nil {
		h.logger.Error("failed to enqueue message", zap.Error(err))

		switch {
		case errors.Is(err, apperror.ErrValidation):
			RespondError(c, http.StatusBadRequest, "validation_error", err.Error())
		default:
			RespondError(c, http.StatusInternalServerError, "internal_error", "failed to process message")
		}
		return
	}

	resp := MessageStatusResponse{
		ID:         result.Message.ID.String(),
		Status:     result.Message.Status,
		RetryCount: result.Message.RetryCount,
		UpdatedAt:  result.Message.UpdatedAt,
	}

	Respond(c, http.StatusAccepted, resp)
}
