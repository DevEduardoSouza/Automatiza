// Package errors provides domain-specific error types for the application.
package errors

import (
	"errors"
	"fmt"
)

// ML Link Lists domain errors
var (
	// ErrListNotFound indicates the requested ML link list was not found.
	ErrListNotFound = errors.New("ML_LINK_LIST_NOT_FOUND")

	// ErrItemNotFound indicates the requested ML link list item was not found.
	ErrItemNotFound = errors.New("ML_LINK_LIST_ITEM_NOT_FOUND")

	// ErrInvalidURL indicates the provided URL is not a valid Mercado Livre URL.
	ErrInvalidURL = errors.New("INVALID_MERCADOLIVRE_URL")

	// ErrInvalidURLFormat indicates the URL format is malformed.
	ErrInvalidURLFormat = errors.New("INVALID_URL_FORMAT")

	// ErrURLAlreadyExists indicates the URL already exists in the list.
	ErrURLAlreadyExists = errors.New("URL_ALREADY_EXISTS")

	// ErrItemAlreadyConsumed indicates the item was already consumed by another process.
	ErrItemAlreadyConsumed = errors.New("ITEM_ALREADY_CONSUMED")

	// ErrInvalidMetadataJSON indicates the metadata field contains invalid JSON.
	ErrInvalidMetadataJSON = errors.New("INVALID_METADATA_JSON")

	// ErrUnauthorized indicates the user is not authorized to access the resource.
	ErrUnauthorized = errors.New("UNAUTHORIZED")

	// ErrNoItemsAvailable indicates no unconsumed items are available in the list.
	ErrNoItemsAvailable = errors.New("NO_ITEMS_AVAILABLE")

	// ErrListNotConfigured indicates the automation has no ML link list configured.
	ErrListNotConfigured = errors.New("ML_LINK_LIST_NOT_CONFIGURED")

	// ErrNoGroupsConfigured indicates no send groups are configured for the automation.
	ErrNoGroupsConfigured = errors.New("NO_GROUPS_CONFIGURED")

	// ErrWhatsAppNotConnected indicates the WhatsApp instance is not connected.
	ErrWhatsAppNotConnected = errors.New("WHATSAPP_NOT_CONNECTED")
)

// MLLinksError provides detailed error information for ML Links operations.
type MLLinksError struct {
	Code    string // Error code for i18n mapping
	Message string // Human-readable message (for logging)
	Field   string // Optional field name for validation errors
	Err     error  // Wrapped error
}

func (e *MLLinksError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (field: %s)", e.Code, e.Message, e.Field)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *MLLinksError) Unwrap() error {
	return e.Err
}

// NewMLLinksError creates a new MLLinksError.
func NewMLLinksError(code, message string, err error) *MLLinksError {
	return &MLLinksError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewMLLinksFieldError creates a new MLLinksError with field information.
func NewMLLinksFieldError(code, message, field string, err error) *MLLinksError {
	return &MLLinksError{
		Code:    code,
		Message: message,
		Field:   field,
		Err:     err,
	}
}

// WrapInternal wraps an internal error with a generic message for the client.
// The original error is preserved for logging but not exposed to the client.
func WrapInternal(err error, operation string) *MLLinksError {
	return &MLLinksError{
		Code:    "INTERNAL_ERROR",
		Message: fmt.Sprintf("internal error during %s", operation),
		Err:     err,
	}
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrListNotFound) || errors.Is(err, ErrItemNotFound)
}

// IsValidationError checks if the error is a validation error.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidURL) ||
		errors.Is(err, ErrInvalidURLFormat) ||
		errors.Is(err, ErrInvalidMetadataJSON)
}

// IsAlreadyExists checks if the error is an already exists error.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrURLAlreadyExists) || errors.Is(err, ErrItemAlreadyConsumed)
}
