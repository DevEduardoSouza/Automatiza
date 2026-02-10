package middleware

import (
	"automatiza-go/internal/models"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// ApiError represents an API error with status code
type ApiError struct {
	StatusCode int
	Message    string
	Code       string
}

// Error implements the error interface
func (e *ApiError) Error() string {
	return e.Message
}

// NewApiError creates a new API error
func NewApiError(statusCode int, message string) *ApiError {
	return &ApiError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// NewApiErrorWithCode creates a new API error with a code
func NewApiErrorWithCode(statusCode int, message, code string) *ApiError {
	return &ApiError{
		StatusCode: statusCode,
		Message:    message,
		Code:       code,
	}
}

// Common API errors
var (
	ErrUnauthorized       = NewApiError(http.StatusUnauthorized, "Não autorizado")
	ErrForbidden          = NewApiError(http.StatusForbidden, "Acesso negado")
	ErrNotFound           = NewApiError(http.StatusNotFound, "Recurso não encontrado")
	ErrBadRequest         = NewApiError(http.StatusBadRequest, "Requisição inválida")
	ErrInternalServer     = NewApiError(http.StatusInternalServerError, "Erro interno do servidor")
	ErrServiceUnavailable = NewApiError(http.StatusServiceUnavailable, "Serviço indisponível")
)

// ErrorHandler is the global error handling middleware
func ErrorHandler(logger *zap.Logger, isDevelopment bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check for errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// Handle ApiError
			var apiErr *ApiError
			if errors.As(err, &apiErr) {
				response := models.ApiResponse[any]{
					Success: false,
					Error:   apiErr.Message,
				}
				c.JSON(apiErr.StatusCode, response)
				return
			}

			// Handle validation errors
			var validationErrs validator.ValidationErrors
			if errors.As(err, &validationErrs) {
				messages := make([]string, 0, len(validationErrs))
				for _, e := range validationErrs {
					messages = append(messages, formatValidationError(e))
				}

				response := models.ApiResponse[any]{
					Success: false,
					Error:   "Erro de validação",
					Message: joinStrings(messages, "; "),
				}
				c.JSON(http.StatusBadRequest, response)
				return
			}

			// Log internal errors
			logger.Error("Internal error",
				zap.Error(err),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
			)

			// Generic error response
			response := models.ApiResponse[any]{
				Success: false,
				Error:   "Erro interno do servidor",
			}

			// Include error details in development
			if isDevelopment {
				response.Message = err.Error()
			}

			c.JSON(http.StatusInternalServerError, response)
		}
	}
}

// RecoveryMiddleware handles panics
func RecoveryMiddleware(logger *zap.Logger, isDevelopment bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered",
					zap.Any("panic", r),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				response := models.ApiResponse[any]{
					Success: false,
					Error:   "Erro interno do servidor",
				}

				if isDevelopment {
					if err, ok := r.(error); ok {
						response.Message = err.Error()
					}
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			}
		}()

		c.Next()
	}
}

// NotFoundHandler handles 404 errors
func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, models.ApiResponse[any]{
			Success: false,
			Error:   "Rota não encontrada",
		})
	}
}

// formatValidationError formats a single validation error
func formatValidationError(e validator.FieldError) string {
	field := e.Field()
	tag := e.Tag()

	switch tag {
	case "required":
		return field + " é obrigatório"
	case "email":
		return field + " deve ser um email válido"
	case "min":
		return field + " deve ter no mínimo " + e.Param() + " caracteres"
	case "max":
		return field + " deve ter no máximo " + e.Param() + " caracteres"
	case "oneof":
		return field + " deve ser um dos valores: " + e.Param()
	default:
		return field + " é inválido"
	}
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// HandleError adds an error to the context
func HandleError(c *gin.Context, err error) {
	_ = c.Error(err)
	c.Abort()
}

// RespondError responds with an error
func RespondError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, models.ApiResponse[any]{
		Success: false,
		Error:   message,
	})
}

// RespondSuccess responds with success
func RespondSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, models.ApiResponse[T]{
		Success: true,
		Data:    data,
	})
}

// RespondSuccessWithMessage responds with success and a message
func RespondSuccessWithMessage[T any](c *gin.Context, data T, message string) {
	c.JSON(http.StatusOK, models.ApiResponse[T]{
		Success: true,
		Data:    data,
		Message: message,
	})
}

// RespondCreated responds with 201 Created
func RespondCreated[T any](c *gin.Context, data T) {
	c.JSON(http.StatusCreated, models.ApiResponse[T]{
		Success: true,
		Data:    data,
	})
}

// RespondNoContent responds with 204 No Content
func RespondNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
