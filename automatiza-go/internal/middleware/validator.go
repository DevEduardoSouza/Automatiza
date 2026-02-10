package middleware

import (
	"automatiza-go/internal/models"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// ValidatorMiddleware validates request body against a schema
func ValidatorMiddleware[T any]() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req T

		if err := c.ShouldBindJSON(&req); err != nil {
			var validationErrs validator.ValidationErrors
			if errors.As(err, &validationErrs) {
				messages := formatValidationErrors(validationErrs)
				c.AbortWithStatusJSON(http.StatusBadRequest, models.ApiResponse[any]{
					Success: false,
					Error:   "Erro de validação",
					Message: strings.Join(messages, "; "),
				})
				return
			}

			c.AbortWithStatusJSON(http.StatusBadRequest, models.ApiResponse[any]{
				Success: false,
				Error:   "Dados inválidos",
				Message: err.Error(),
			})
			return
		}

		// Store validated request in context
		c.Set("validatedRequest", &req)
		c.Next()
	}
}

// GetValidatedRequest retrieves the validated request from context
func GetValidatedRequest[T any](c *gin.Context) (*T, bool) {
	val, exists := c.Get("validatedRequest")
	if !exists {
		return nil, false
	}

	req, ok := val.(*T)
	return req, ok
}

// BindAndValidate binds and validates the request body
func BindAndValidate[T any](c *gin.Context) (*T, error) {
	var req T

	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// BindQuery binds and validates query parameters
func BindQuery[T any](c *gin.Context) (*T, error) {
	var req T

	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// formatValidationErrors formats validation errors into readable messages
func formatValidationErrors(errs validator.ValidationErrors) []string {
	messages := make([]string, 0, len(errs))
	for _, e := range errs {
		messages = append(messages, formatSingleValidationError(e))
	}
	return messages
}

// formatSingleValidationError formats a single validation error
func formatSingleValidationError(e validator.FieldError) string {
	field := toSnakeCase(e.Field())
	tag := e.Tag()

	switch tag {
	case "required":
		return field + " é obrigatório"
	case "email":
		return field + " deve ser um email válido"
	case "min":
		return field + " deve ter no mínimo " + e.Param()
	case "max":
		return field + " deve ter no máximo " + e.Param()
	case "len":
		return field + " deve ter exatamente " + e.Param() + " caracteres"
	case "oneof":
		return field + " deve ser um dos valores: " + e.Param()
	case "url":
		return field + " deve ser uma URL válida"
	case "uuid":
		return field + " deve ser um UUID válido"
	case "gte":
		return field + " deve ser maior ou igual a " + e.Param()
	case "lte":
		return field + " deve ser menor ou igual a " + e.Param()
	default:
		return field + " é inválido (" + tag + ")"
	}
}

// toSnakeCase converts a string to snake_case
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// RegisterCustomValidators registers custom validation rules
func RegisterCustomValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		// Register custom struct tag name function
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})

		// Register custom validators here
		// Example:
		// v.RegisterValidation("phone_br", validateBrazilianPhone)
	}
}

// ValidateStruct validates a struct and returns formatted errors
func ValidateStruct(s interface{}) []string {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.Struct(s); err != nil {
			var validationErrs validator.ValidationErrors
			if errors.As(err, &validationErrs) {
				return formatValidationErrors(validationErrs)
			}
			return []string{err.Error()}
		}
	}
	return nil
}
