package apperror

import "errors"

var (
	ErrValidation        = errors.New("validation failed")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrRateLimited       = errors.New("rate limited")
	ErrProviderTemporary = errors.New("provider temporary failure")
	ErrProviderPermanent = errors.New("provider permanent failure")
)
