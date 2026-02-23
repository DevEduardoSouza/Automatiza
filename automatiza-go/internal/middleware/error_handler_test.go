package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	ut "github.com/go-playground/universal-translator"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap/zaptest"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// triggerValidationErrors returns a validator.ValidationErrors by validating a
// struct that deliberately violates the supplied tags.
type validationTarget struct {
	Email    string `validate:"required,email"`
	Name     string `validate:"required,min=3,max=10"`
	Category string `validate:"required,oneof=a b c"`
}

func mustValidationErrors(t *testing.T, s interface{}) validator.ValidationErrors {
	t.Helper()
	v := validator.New()
	err := v.Struct(s)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected validator.ValidationErrors, got %T", err)
	}
	return ve
}

// ---------------------------------------------------------------------------
// ApiError type
// ---------------------------------------------------------------------------

func TestApiError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ApiError
		expected string
	}{
		{
			name:     "simple message",
			err:      &ApiError{StatusCode: 400, Message: "bad request"},
			expected: "bad request",
		},
		{
			name:     "empty message",
			err:      &ApiError{StatusCode: 500, Message: ""},
			expected: "",
		},
		{
			name:     "with code field",
			err:      &ApiError{StatusCode: 409, Message: "conflict", Code: "DUPLICATE"},
			expected: "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestApiError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()
	var _ error = (*ApiError)(nil)
}

func TestNewApiError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{name: "400", statusCode: http.StatusBadRequest, message: "bad"},
		{name: "500", statusCode: http.StatusInternalServerError, message: "internal"},
		{name: "404", statusCode: http.StatusNotFound, message: "not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewApiError(tt.statusCode, tt.message)
			if err.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", err.StatusCode, tt.statusCode)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Code != "" {
				t.Errorf("Code should be empty, got %q", err.Code)
			}
		})
	}
}

func TestNewApiErrorWithCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		message    string
		code       string
	}{
		{name: "with code", statusCode: 409, message: "conflict", code: "DUPLICATE_ENTRY"},
		{name: "empty code", statusCode: 400, message: "bad", code: ""},
		{name: "custom code", statusCode: 422, message: "unprocessable", code: "VALIDATION_FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := NewApiErrorWithCode(tt.statusCode, tt.message, tt.code)
			if err.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", err.StatusCode, tt.statusCode)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %q, want %q", err.Message, tt.message)
			}
			if err.Code != tt.code {
				t.Errorf("Code = %q, want %q", err.Code, tt.code)
			}
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            *ApiError
		expectedStatus int
		expectedMsg    string
	}{
		{name: "ErrUnauthorized", err: ErrUnauthorized, expectedStatus: http.StatusUnauthorized, expectedMsg: "Não autorizado"},
		{name: "ErrForbidden", err: ErrForbidden, expectedStatus: http.StatusForbidden, expectedMsg: "Acesso negado"},
		{name: "ErrNotFound", err: ErrNotFound, expectedStatus: http.StatusNotFound, expectedMsg: "Recurso não encontrado"},
		{name: "ErrBadRequest", err: ErrBadRequest, expectedStatus: http.StatusBadRequest, expectedMsg: "Requisição inválida"},
		{name: "ErrInternalServer", err: ErrInternalServer, expectedStatus: http.StatusInternalServerError, expectedMsg: "Erro interno do servidor"},
		{name: "ErrServiceUnavailable", err: ErrServiceUnavailable, expectedStatus: http.StatusServiceUnavailable, expectedMsg: "Serviço indisponível"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.StatusCode != tt.expectedStatus {
				t.Errorf("StatusCode = %d, want %d", tt.err.StatusCode, tt.expectedStatus)
			}
			if tt.err.Message != tt.expectedMsg {
				t.Errorf("Message = %q, want %q", tt.err.Message, tt.expectedMsg)
			}
			if tt.err.Error() != tt.expectedMsg {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.expectedMsg)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ErrorHandler middleware
// ---------------------------------------------------------------------------

func TestErrorHandler_NoErrors(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestErrorHandler_ApiError4xx(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		apiErr         *ApiError
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "400 bad request",
			apiErr:         NewApiError(http.StatusBadRequest, "campo invalido"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "campo invalido",
		},
		{
			name:           "401 unauthorized",
			apiErr:         ErrUnauthorized,
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Não autorizado",
		},
		{
			name:           "403 forbidden",
			apiErr:         ErrForbidden,
			expectedStatus: http.StatusForbidden,
			expectedMsg:    "Acesso negado",
		},
		{
			name:           "404 not found",
			apiErr:         ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedMsg:    "Recurso não encontrado",
		},
		{
			name:           "422 custom",
			apiErr:         NewApiErrorWithCode(http.StatusUnprocessableEntity, "entidade invalida", "INVALID"),
			expectedStatus: http.StatusUnprocessableEntity,
			expectedMsg:    "entidade invalida",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := zaptest.NewLogger(t)
			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			engine.Use(ErrorHandler(logger, false))
			engine.GET("/test", func(c *gin.Context) {
				_ = c.Error(tt.apiErr)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			resp := parseResponse(t, w)
			if resp.Success {
				t.Error("expected success=false")
			}
			if resp.Error != tt.expectedMsg {
				t.Errorf("expected error=%q, got %q", tt.expectedMsg, resp.Error)
			}
		})
	}
}

func TestErrorHandler_ApiError5xx(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(ErrInternalServer)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "Erro interno do servidor" {
		t.Errorf("expected error=%q, got %q", "Erro interno do servidor", resp.Error)
	}
}

func TestErrorHandler_ValidationErrors(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	// Create a real validation error via the validator package.
	ve := mustValidationErrors(t, &validationTarget{})

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(ve)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "Erro de validação" {
		t.Errorf("expected error=%q, got %q", "Erro de validação", resp.Error)
	}
	// The message field should contain joined validation messages.
	if resp.Message == "" {
		t.Error("expected non-empty message with validation details")
	}
}

func TestErrorHandler_GenericError_Production(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(errors.New("database connection lost"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "Erro interno do servidor" {
		t.Errorf("expected generic error message, got %q", resp.Error)
	}
	// In production mode, the internal error message must NOT leak.
	if resp.Message != "" {
		t.Errorf("expected empty message in production, got %q", resp.Message)
	}
}

func TestErrorHandler_GenericError_Development(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, true))
	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(errors.New("database connection lost"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Error != "Erro interno do servidor" {
		t.Errorf("expected generic error, got %q", resp.Error)
	}
	// In development mode, the detail should be present.
	if resp.Message != "database connection lost" {
		t.Errorf("expected message=%q in dev mode, got %q", "database connection lost", resp.Message)
	}
}

func TestErrorHandler_MultipleErrors_UsesLast(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(NewApiError(http.StatusBadRequest, "first error"))
		_ = c.Error(NewApiError(http.StatusConflict, "second error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	// ErrorHandler uses c.Errors.Last(), so the second error wins.
	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	if resp.Error != "second error" {
		t.Errorf("expected error=%q, got %q", "second error", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// RecoveryMiddleware
// ---------------------------------------------------------------------------

func TestRecoveryMiddleware_PanicString(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(RecoveryMiddleware(logger, false))
	engine.GET("/panic", func(_ *gin.Context) {
		panic("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "Erro interno do servidor" {
		t.Errorf("expected generic error, got %q", resp.Error)
	}
	// In production, the panic value must not appear in the message.
	if resp.Message != "" {
		t.Errorf("expected empty message in prod after string panic, got %q", resp.Message)
	}
}

func TestRecoveryMiddleware_PanicError_Development(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(RecoveryMiddleware(logger, true))
	engine.GET("/panic", func(_ *gin.Context) {
		panic(errors.New("nil pointer dereference"))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Error != "Erro interno do servidor" {
		t.Errorf("expected generic error, got %q", resp.Error)
	}
	// In dev mode with an error-typed panic, the message should contain the error text.
	if resp.Message != "nil pointer dereference" {
		t.Errorf("expected message=%q in dev, got %q", "nil pointer dereference", resp.Message)
	}
}

func TestRecoveryMiddleware_PanicError_Production(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(RecoveryMiddleware(logger, false))
	engine.GET("/panic", func(_ *gin.Context) {
		panic(errors.New("secret database error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	// In production, the error detail must NOT leak.
	if resp.Message != "" {
		t.Errorf("expected empty message in prod, got %q", resp.Message)
	}
}

func TestRecoveryMiddleware_PanicNonError_Development(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(RecoveryMiddleware(logger, true))
	engine.GET("/panic", func(_ *gin.Context) {
		panic(42) // panic with a non-error type
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	// Non-error panic in dev: the type assertion `r.(error)` fails, so message stays empty.
	if resp.Message != "" {
		t.Errorf("expected empty message for non-error panic in dev, got %q", resp.Message)
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(RecoveryMiddleware(logger, false))
	engine.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// NotFoundHandler
// ---------------------------------------------------------------------------

func TestNotFoundHandler(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.NoRoute(NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "Rota não encontrada" {
		t.Errorf("expected error=%q, got %q", "Rota não encontrada", resp.Error)
	}
}

func TestNotFoundHandler_DifferentMethods(t *testing.T) {
	t.Parallel()

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)
			engine.NoRoute(NotFoundHandler())

			req := httptest.NewRequest(method, "/does-not-exist", nil)
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected status 404 for %s, got %d", method, w.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatValidationError
// ---------------------------------------------------------------------------

// mockFieldError implements validator.FieldError for testing.
type mockFieldError struct {
	field string
	tag   string
	param string
}

func (m mockFieldError) Tag() string                      { return m.tag }
func (m mockFieldError) ActualTag() string                { return m.tag }
func (m mockFieldError) Namespace() string                { return "" }
func (m mockFieldError) StructNamespace() string          { return "" }
func (m mockFieldError) Field() string                    { return m.field }
func (m mockFieldError) StructField() string              { return m.field }
func (m mockFieldError) Value() interface{}               { return nil }
func (m mockFieldError) Param() string                    { return m.param }
func (m mockFieldError) Kind() reflect.Kind   { return reflect.String }
func (m mockFieldError) Type() reflect.Type   { return reflect.TypeOf("") }
func (m mockFieldError) Translate(_ ut.Translator) string { return "" }
func (m mockFieldError) Error() string                    { return fmt.Sprintf("Key: '%s' Error:Field validation failed on the '%s' tag", m.field, m.tag) }

// Ensure mockFieldError satisfies the interface at compile time.
var _ validator.FieldError = mockFieldError{}

func TestFormatValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fe       validator.FieldError
		expected string
	}{
		{
			name:     "required tag",
			fe:       mockFieldError{field: "Email", tag: "required"},
			expected: "Email é obrigatório",
		},
		{
			name:     "email tag",
			fe:       mockFieldError{field: "Email", tag: "email"},
			expected: "Email deve ser um email válido",
		},
		{
			name:     "min tag",
			fe:       mockFieldError{field: "Password", tag: "min", param: "8"},
			expected: "Password deve ter no mínimo 8 caracteres",
		},
		{
			name:     "max tag",
			fe:       mockFieldError{field: "Name", tag: "max", param: "100"},
			expected: "Name deve ter no máximo 100 caracteres",
		},
		{
			name:     "oneof tag",
			fe:       mockFieldError{field: "Role", tag: "oneof", param: "admin user guest"},
			expected: "Role deve ser um dos valores: admin user guest",
		},
		{
			name:     "unknown tag falls to default",
			fe:       mockFieldError{field: "Field", tag: "uuid"},
			expected: "Field é inválido",
		},
		{
			name:     "another unknown tag",
			fe:       mockFieldError{field: "Token", tag: "alphanum"},
			expected: "Token é inválido",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatValidationError(tt.fe)
			if got != tt.expected {
				t.Errorf("formatValidationError() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// joinStrings
// ---------------------------------------------------------------------------

func TestJoinStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		strs     []string
		sep      string
		expected string
	}{
		{
			name:     "empty slice",
			strs:     []string{},
			sep:      "; ",
			expected: "",
		},
		{
			name:     "nil slice",
			strs:     nil,
			sep:      "; ",
			expected: "",
		},
		{
			name:     "single element",
			strs:     []string{"hello"},
			sep:      "; ",
			expected: "hello",
		},
		{
			name:     "two elements",
			strs:     []string{"a", "b"},
			sep:      "; ",
			expected: "a; b",
		},
		{
			name:     "three elements",
			strs:     []string{"foo", "bar", "baz"},
			sep:      ", ",
			expected: "foo, bar, baz",
		},
		{
			name:     "empty separator",
			strs:     []string{"a", "b", "c"},
			sep:      "",
			expected: "abc",
		},
		{
			name:     "single char separator",
			strs:     []string{"x", "y"},
			sep:      "-",
			expected: "x-y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := joinStrings(tt.strs, tt.sep)
			if got != tt.expected {
				t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.strs, tt.sep, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// captureGinError (with nil hub - no panic)
// ---------------------------------------------------------------------------

func TestCaptureGinError_NilHub_NoPanic(t *testing.T) {
	t.Parallel()

	// Create a gin context without sentrygin middleware, so the hub is nil.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	// Should not panic even though there is no Sentry hub in the context.
	captureGinError(c, errors.New("test error"))
}

func TestCaptureGinError_NilHub_MultipleCallsNoPanic(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/data", nil)

	// Multiple calls should all be safe.
	for i := 0; i < 5; i++ {
		captureGinError(c, fmt.Errorf("error %d", i))
	}
}

// ---------------------------------------------------------------------------
// HandleError
// ---------------------------------------------------------------------------

func TestHandleError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "api error", err: NewApiError(http.StatusBadRequest, "bad")},
		{name: "generic error", err: errors.New("something broke")},
		{name: "wrapped error", err: fmt.Errorf("wrap: %w", errors.New("inner"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			handlerCalled := false
			engine.GET("/test", func(c *gin.Context) {
				HandleError(c, tt.err)
			}, func(_ *gin.Context) {
				handlerCalled = true
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			// HandleError calls c.Abort(), so subsequent handlers should NOT be called.
			if handlerCalled {
				t.Error("expected subsequent handler NOT to be called after HandleError")
			}
		})
	}
}

func TestHandleError_AddsErrorToContext(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	apiErr := NewApiError(http.StatusConflict, "duplicate entry")

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		HandleError(c, apiErr)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	// ErrorHandler should pick up the error set by HandleError.
	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Error != "duplicate entry" {
		t.Errorf("expected error=%q, got %q", "duplicate entry", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// RespondError
// ---------------------------------------------------------------------------

func TestRespondError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{name: "400", statusCode: http.StatusBadRequest, message: "campo invalido"},
		{name: "500", statusCode: http.StatusInternalServerError, message: "falha interna"},
		{name: "503", statusCode: http.StatusServiceUnavailable, message: "servico indisponivel"},
		{name: "404", statusCode: http.StatusNotFound, message: "nao encontrado"},
		{name: "422", statusCode: http.StatusUnprocessableEntity, message: "dados invalidos"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			engine.GET("/test", func(c *gin.Context) {
				RespondError(c, tt.statusCode, tt.message)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, w.Code)
			}

			resp := parseResponse(t, w)
			if resp.Success {
				t.Error("expected success=false")
			}
			if resp.Error != tt.message {
				t.Errorf("expected error=%q, got %q", tt.message, resp.Error)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RespondSuccess
// ---------------------------------------------------------------------------

func TestRespondSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data interface{}
	}{
		{name: "string data", data: "hello"},
		{name: "map data", data: map[string]string{"key": "value"}},
		{name: "nil data", data: nil},
		{name: "int data", data: 42},
		{name: "struct data", data: struct {
			Name string `json:"name"`
		}{Name: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			engine.GET("/test", func(c *gin.Context) {
				RespondSuccess(c, tt.data)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			resp := parseResponse(t, w)
			if !resp.Success {
				t.Error("expected success=true")
			}
			if resp.Error != "" {
				t.Errorf("expected no error, got %q", resp.Error)
			}
		})
	}
}

func TestRespondSuccess_DataPreserved(t *testing.T) {
	t.Parallel()

	type payload struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.GET("/test", func(c *gin.Context) {
		RespondSuccess(c, payload{ID: 1, Name: "test"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	data, ok := raw["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["name"] != "test" {
		t.Errorf("expected data.name=%q, got %v", "test", data["name"])
	}
	if int(data["id"].(float64)) != 1 {
		t.Errorf("expected data.id=1, got %v", data["id"])
	}
}

// ---------------------------------------------------------------------------
// RespondSuccessWithMessage
// ---------------------------------------------------------------------------

func TestRespondSuccessWithMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    interface{}
		message string
	}{
		{name: "with data and message", data: map[string]string{"ok": "true"}, message: "Operacao realizada"},
		{name: "empty message", data: "value", message: ""},
		{name: "nil data with message", data: nil, message: "Sucesso"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			engine.GET("/test", func(c *gin.Context) {
				RespondSuccessWithMessage(c, tt.data, tt.message)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			resp := parseResponse(t, w)
			if !resp.Success {
				t.Error("expected success=true")
			}
			if resp.Message != tt.message {
				t.Errorf("expected message=%q, got %q", tt.message, resp.Message)
			}
			if resp.Error != "" {
				t.Errorf("expected no error, got %q", resp.Error)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RespondCreated
// ---------------------------------------------------------------------------

func TestRespondCreated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data interface{}
	}{
		{name: "map data", data: map[string]string{"id": "abc-123"}},
		{name: "string data", data: "created-item"},
		{name: "struct data", data: struct {
			ID string `json:"id"`
		}{ID: "new-id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			engine.GET("/test", func(c *gin.Context) {
				RespondCreated(c, tt.data)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status 201, got %d", w.Code)
			}

			resp := parseResponse(t, w)
			if !resp.Success {
				t.Error("expected success=true")
			}
			if resp.Error != "" {
				t.Errorf("expected no error, got %q", resp.Error)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RespondNoContent
// ---------------------------------------------------------------------------

func TestRespondNoContent(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.DELETE("/test", func(c *gin.Context) {
		RespondNoContent(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ErrorHandler + RecoveryMiddleware integration
// ---------------------------------------------------------------------------

func TestErrorHandler_And_RecoveryMiddleware_Integration(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	// RecoveryMiddleware wraps ErrorHandler, which is the typical production setup.
	engine.Use(RecoveryMiddleware(logger, false))
	engine.Use(ErrorHandler(logger, false))

	engine.GET("/panic", func(_ *gin.Context) {
		panic("unexpected failure")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "Erro interno do servidor" {
		t.Errorf("expected generic error, got %q", resp.Error)
	}
}

func TestErrorHandler_WrappedApiError(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		// Wrap an ApiError with fmt.Errorf; errors.As should still unwrap it.
		wrapped := fmt.Errorf("service layer: %w", NewApiError(http.StatusConflict, "already exists"))
		_ = c.Error(wrapped)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Error != "already exists" {
		t.Errorf("expected error=%q, got %q", "already exists", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// ErrorHandler with ServiceUnavailable (5xx range)
// ---------------------------------------------------------------------------

func TestErrorHandler_ApiError503(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(ErrorHandler(logger, false))
	engine.GET("/test", func(c *gin.Context) {
		_ = c.Error(ErrServiceUnavailable)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	resp := parseResponse(t, w)
	if resp.Error != "Serviço indisponível" {
		t.Errorf("expected error=%q, got %q", "Serviço indisponível", resp.Error)
	}
}

// ---------------------------------------------------------------------------
// captureGinError with non-nil Sentry hub
// ---------------------------------------------------------------------------

func TestCaptureGinError_WithSentryHub(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		err    error
	}{
		{
			name:   "5xx ApiError triggers captureGinError with hub",
			method: http.MethodGet,
			path:   "/api/internal",
			err:    ErrInternalServer,
		},
		{
			name:   "generic error triggers captureGinError with hub",
			method: http.MethodPost,
			path:   "/api/data",
			err:    errors.New("database timeout"),
		},
		{
			name:   "503 ApiError triggers captureGinError with hub",
			method: http.MethodPut,
			path:   "/api/resource",
			err:    ErrServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Initialize Sentry with a noop transport so no actual events are sent.
			err := sentry.Init(sentry.ClientOptions{
				Dsn:       "",
				Transport: &noopTransport{},
			})
			if err != nil {
				t.Fatalf("failed to init sentry: %v", err)
			}

			// Create a hub with its own scope to isolate test state.
			client := sentry.CurrentHub().Client()
			hub := sentry.NewHub(client, sentry.NewScope())

			logger := zaptest.NewLogger(t)
			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			// Manually set the sentry hub on gin context (same key sentrygin uses).
			engine.Use(func(c *gin.Context) {
				c.Set("sentry", hub)
				c.Next()
			})
			engine.Use(ErrorHandler(logger, false))
			engine.Handle(tt.method, "/api/*path", func(c *gin.Context) {
				_ = c.Error(tt.err)
			})

			req := httptest.NewRequest(tt.method, tt.path, nil)
			engine.ServeHTTP(w, req)

			// Verify the response was still returned correctly.
			if w.Code < 400 {
				t.Errorf("expected error status code, got %d", w.Code)
			}

			resp := parseResponse(t, w)
			if resp.Success {
				t.Error("expected success=false")
			}
		})
	}
}

func TestCaptureGinError_DirectCallWithHub(t *testing.T) {
	t.Parallel()

	// Initialize Sentry with a noop transport.
	err := sentry.Init(sentry.ClientOptions{
		Dsn:       "",
		Transport: &noopTransport{},
	})
	if err != nil {
		t.Fatalf("failed to init sentry: %v", err)
	}

	client := sentry.CurrentHub().Client()
	hub := sentry.NewHub(client, sentry.NewScope())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test/path", nil)

	// Set the sentry hub on gin context using the key "sentry".
	c.Set("sentry", hub)

	// Should execute the hub.WithScope path (non-nil hub branch).
	// This should not panic and should capture the exception.
	captureGinError(c, errors.New("test error with hub"))
}

func TestCaptureGinError_HubCapturesExceptionTags(t *testing.T) {
	t.Parallel()

	// Initialize Sentry with a capturing transport to verify events are sent.
	transport := &capturingTransport{}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:       "https://examplePublicKey@o0.ingest.sentry.io/0",
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("failed to init sentry: %v", err)
	}

	client := sentry.CurrentHub().Client()
	hub := sentry.NewHub(client, sentry.NewScope())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/webhook/test", nil)

	// Set the sentry hub on the gin context.
	c.Set("sentry", hub)

	testErr := errors.New("webhook processing failed")
	captureGinError(c, testErr)

	// Flush to ensure event is captured.
	hub.Flush(2 * time.Second)

	events := transport.Events()
	if len(events) == 0 {
		t.Fatal("expected at least one captured event, got none")
	}

	lastEvent := events[len(events)-1]

	// Verify tags were set correctly by the hub.WithScope callback.
	if path, ok := lastEvent.Tags["path"]; !ok || path != "/webhook/test" {
		t.Errorf("expected tag path=%q, got %q", "/webhook/test", path)
	}
	if method, ok := lastEvent.Tags["method"]; !ok || method != "POST" {
		t.Errorf("expected tag method=%q, got %q", "POST", method)
	}
}

// noopTransport is a Sentry transport that discards all events.
type noopTransport struct{}

func (t *noopTransport) Configure(_ sentry.ClientOptions)         {}
func (t *noopTransport) SendEvent(_ *sentry.Event)                {}
func (t *noopTransport) Flush(_ time.Duration) bool               { return true }
func (t *noopTransport) FlushWithContext(_ context.Context) bool   { return true }
func (t *noopTransport) Close()                                    {}

// capturingTransport captures Sentry events for test assertions.
type capturingTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *capturingTransport) Configure(_ sentry.ClientOptions)       {}
func (t *capturingTransport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}
func (t *capturingTransport) Flush(_ time.Duration) bool             { return true }
func (t *capturingTransport) FlushWithContext(_ context.Context) bool { return true }
func (t *capturingTransport) Close()                                  {}
func (t *capturingTransport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	copied := make([]*sentry.Event, len(t.events))
	copy(copied, t.events)
	return copied
}
