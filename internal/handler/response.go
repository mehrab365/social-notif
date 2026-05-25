package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Data      any    `json:"data,omitempty"`
	Error     any    `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Respond(c *gin.Context, status int, data any) {
	c.JSON(status, APIResponse{
		Data:      data,
		RequestID: requestID(c),
	})
}

func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIResponse{
		Error: APIError{
			Code:    code,
			Message: message,
		},
		RequestID: requestID(c),
	})
}

func RespondSuccess(c *gin.Context, code, message string) {
	c.JSON(http.StatusOK, APIResponse{
		Data: APIError{
			Code:    code,
			Message: message,
		},
		RequestID: requestID(c),
	})
}

func requestID(c *gin.Context) string {
	value, exists := c.Get("request_id")
	if !exists {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}
