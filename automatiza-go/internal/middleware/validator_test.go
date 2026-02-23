package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"automatiza-go/internal/models"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// Test structs
// ---------------------------------------------------------------------------

type testValidatorRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
	Age   int    `json:"age" binding:"gte=0,lte=150"`
}

type testQueryRequest struct {
	Page  int    `form:"page" binding:"required,gte=1"`
	Limit int    `form:"limit" binding:"required,gte=1,lte=100"`
	Sort  string `form:"sort" binding:"omitempty,oneof=asc desc"`
}

type testMinMaxRequest struct {
	Password string `json:"password" binding:"required,min=8,max=64"`
}

type testLenRequest struct {
	Code string `json:"code" binding:"required,len=6"`
}

type testURLRequest struct {
	Website string `json:"website" binding:"required,url"`
}

type testUUIDRequest struct {
	ID string `json:"id" binding:"required,uuid"`
}

// unmarshalAPIResponse is a test helper that unmarshals the response body.
// Uses t.Helper + t.Fatal so the calling test gets the correct line number.
func unmarshalAPIResponse(t *testing.T, w *httptest.ResponseRecorder) models.ApiResponse[any] {
	t.Helper()
	var resp models.ApiResponse[any]
	// slog: test-only unmarshal helper — no observability needed in test harness
	data := w.Body.Bytes()
	if jsonErr := json.Unmarshal(data, &resp); jsonErr != nil {
		t.Fatalf("failed to unmarshal response body: %v", jsonErr)
	}
	return resp
}

// ---------------------------------------------------------------------------
// ValidatorMiddleware
// ---------------------------------------------------------------------------

func TestValidatorMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		contentType    string
		expectStatus   int
		expectSuccess  bool
		expectError    string
		expectNextCall bool
	}{
		{
			name:           "valid JSON passes validation and calls next",
			body:           `{"name":"Bruno","email":"bruno@test.com","age":30}`,
			contentType:    "application/json",
			expectStatus:   http.StatusOK,
			expectSuccess:  true,
			expectNextCall: true,
		},
		{
			name:           "invalid JSON returns 400 with parse error",
			body:           `{"name": invalid}`,
			contentType:    "application/json",
			expectStatus:   http.StatusBadRequest,
			expectSuccess:  false,
			expectError:    "Dados inválidos",
			expectNextCall: false,
		},
		{
			name:           "empty body returns 400",
			body:           ``,
			contentType:    "application/json",
			expectStatus:   http.StatusBadRequest,
			expectSuccess:  false,
			expectError:    "Dados inválidos",
			expectNextCall: false,
		},
		{
			name:           "missing required fields returns validation error",
			body:           `{"age":25}`,
			contentType:    "application/json",
			expectStatus:   http.StatusBadRequest,
			expectSuccess:  false,
			expectError:    "Erro de validação",
			expectNextCall: false,
		},
		{
			name:           "invalid email returns validation error",
			body:           `{"name":"Bruno","email":"not-an-email","age":30}`,
			contentType:    "application/json",
			expectStatus:   http.StatusBadRequest,
			expectSuccess:  false,
			expectError:    "Erro de validação",
			expectNextCall: false,
		},
		{
			name:           "age below gte returns validation error",
			body:           `{"name":"Bruno","email":"bruno@test.com","age":-1}`,
			contentType:    "application/json",
			expectStatus:   http.StatusBadRequest,
			expectSuccess:  false,
			expectError:    "Erro de validação",
			expectNextCall: false,
		},
		{
			name:           "age above lte returns validation error",
			body:           `{"name":"Bruno","email":"bruno@test.com","age":200}`,
			contentType:    "application/json",
			expectStatus:   http.StatusBadRequest,
			expectSuccess:  false,
			expectError:    "Erro de validação",
			expectNextCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			nextCalled := false

			r.POST("/test", ValidatorMiddleware[testValidatorRequest](), func(c *gin.Context) {
				nextCalled = true
				c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)

			r.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			resp := unmarshalAPIResponse(t, w)

			if resp.Success != tt.expectSuccess {
				t.Errorf("expected success=%v, got %v", tt.expectSuccess, resp.Success)
			}

			if tt.expectError != "" && resp.Error != tt.expectError {
				t.Errorf("expected error=%q, got %q", tt.expectError, resp.Error)
			}

			if nextCalled != tt.expectNextCall {
				t.Errorf("expected nextCalled=%v, got %v", tt.expectNextCall, nextCalled)
			}
		})
	}
}

func TestValidatorMiddleware_StoresValidatedRequest(t *testing.T) {
	t.Parallel()

	r := gin.New()

	r.POST("/test", ValidatorMiddleware[testValidatorRequest](), func(c *gin.Context) {
		req, ok := GetValidatedRequest[testValidatorRequest](c)
		if !ok {
			t.Error("expected validated request to be in context")
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, models.ApiResponse[any]{
			Success: true,
			Message: req.Name,
		})
	})

	w := httptest.NewRecorder()
	body := `{"name":"Bruno","email":"bruno@test.com","age":30}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	resp := unmarshalAPIResponse(t, w)

	if resp.Message != "Bruno" {
		t.Errorf("expected message=%q, got %q", "Bruno", resp.Message)
	}
}

// ---------------------------------------------------------------------------
// GetValidatedRequest
// ---------------------------------------------------------------------------

func TestGetValidatedRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func(c *gin.Context)
		expectOK   bool
		expectName string
	}{
		{
			name: "returns validated request when present",
			setup: func(c *gin.Context) {
				c.Set("validatedRequest", &testValidatorRequest{
					Name:  "Bruno",
					Email: "bruno@test.com",
					Age:   30,
				})
			},
			expectOK:   true,
			expectName: "Bruno",
		},
		{
			name:     "returns false when key not present",
			setup:    func(c *gin.Context) {},
			expectOK: false,
		},
		{
			name: "returns false when wrong type stored",
			setup: func(c *gin.Context) {
				c.Set("validatedRequest", "a plain string")
			},
			expectOK: false,
		},
		{
			name: "returns false when different struct type stored",
			setup: func(c *gin.Context) {
				c.Set("validatedRequest", &testQueryRequest{Page: 1, Limit: 10})
			},
			expectOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			var gotOK bool
			var gotName string

			r.GET("/test", func(c *gin.Context) {
				tt.setup(c)
				req, ok := GetValidatedRequest[testValidatorRequest](c)
				gotOK = ok
				if ok && req != nil {
					gotName = req.Name
				}
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.ServeHTTP(w, req)

			if gotOK != tt.expectOK {
				t.Errorf("expected ok=%v, got %v", tt.expectOK, gotOK)
			}

			if tt.expectOK && gotName != tt.expectName {
				t.Errorf("expected name=%q, got %q", tt.expectName, gotName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BindAndValidate
// ---------------------------------------------------------------------------

func TestBindAndValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		expectErr   bool
		expectName  string
		expectEmail string
	}{
		{
			name:        "valid body binds and validates successfully",
			body:        `{"name":"Alice","email":"alice@example.com","age":25}`,
			expectErr:   false,
			expectName:  "Alice",
			expectEmail: "alice@example.com",
		},
		{
			name:      "missing required field returns error",
			body:      `{"email":"alice@example.com"}`,
			expectErr: true,
		},
		{
			name:      "invalid email returns error",
			body:      `{"name":"Alice","email":"bad-email","age":25}`,
			expectErr: true,
		},
		{
			name:      "malformed JSON returns error",
			body:      `{broken json`,
			expectErr: true,
		},
		{
			name:      "empty body returns error",
			body:      ``,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			var gotErr error
			var gotName, gotEmail string

			r.POST("/test", func(c *gin.Context) {
				result, bindErr := BindAndValidate[testValidatorRequest](c)
				gotErr = bindErr
				if bindErr == nil && result != nil {
					gotName = result.Name
					gotEmail = result.Email
				}
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if tt.expectErr && gotErr == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && gotErr != nil {
				t.Errorf("expected no error, got %v", gotErr)
			}

			if !tt.expectErr {
				if gotName != tt.expectName {
					t.Errorf("expected name=%q, got %q", tt.expectName, gotName)
				}
				if gotEmail != tt.expectEmail {
					t.Errorf("expected email=%q, got %q", tt.expectEmail, gotEmail)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BindQuery
// ---------------------------------------------------------------------------

func TestBindQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		query       string
		expectErr   bool
		expectPage  int
		expectLimit int
		expectSort  string
	}{
		{
			name:        "valid query params bind successfully",
			query:       "page=1&limit=20&sort=asc",
			expectErr:   false,
			expectPage:  1,
			expectLimit: 20,
			expectSort:  "asc",
		},
		{
			name:        "valid without optional sort",
			query:       "page=5&limit=50",
			expectErr:   false,
			expectPage:  5,
			expectLimit: 50,
			expectSort:  "",
		},
		{
			name:      "missing required page",
			query:     "limit=20",
			expectErr: true,
		},
		{
			name:      "missing required limit",
			query:     "page=1",
			expectErr: true,
		},
		{
			name:      "page below gte=1 returns error",
			query:     "page=0&limit=10",
			expectErr: true,
		},
		{
			name:      "limit above lte=100 returns error",
			query:     "page=1&limit=200",
			expectErr: true,
		},
		{
			name:      "invalid sort value returns error",
			query:     "page=1&limit=10&sort=random",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			var gotErr error
			var gotPage, gotLimit int
			var gotSort string

			r.GET("/test", func(c *gin.Context) {
				result, bindErr := BindQuery[testQueryRequest](c)
				gotErr = bindErr
				if bindErr == nil && result != nil {
					gotPage = result.Page
					gotLimit = result.Limit
					gotSort = result.Sort
				}
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test?"+tt.query, nil)
			r.ServeHTTP(w, req)

			if tt.expectErr && gotErr == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && gotErr != nil {
				t.Errorf("expected no error, got %v", gotErr)
			}

			if !tt.expectErr {
				if gotPage != tt.expectPage {
					t.Errorf("expected page=%d, got %d", tt.expectPage, gotPage)
				}
				if gotLimit != tt.expectLimit {
					t.Errorf("expected limit=%d, got %d", tt.expectLimit, gotLimit)
				}
				if gotSort != tt.expectSort {
					t.Errorf("expected sort=%q, got %q", tt.expectSort, gotSort)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatSingleValidationError (all switch cases)
// ---------------------------------------------------------------------------

// mockFieldError is defined in error_handler_test.go (shared across test files).

func TestFormatSingleValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		field    string
		tag      string
		param    string
		expected string
	}{
		{
			name:     "required tag",
			field:    "Name",
			tag:      "required",
			param:    "",
			expected: "name é obrigatório",
		},
		{
			name:     "email tag",
			field:    "Email",
			tag:      "email",
			param:    "",
			expected: "email deve ser um email válido",
		},
		{
			name:     "min tag",
			field:    "Password",
			tag:      "min",
			param:    "8",
			expected: "password deve ter no mínimo 8",
		},
		{
			name:     "max tag",
			field:    "Password",
			tag:      "max",
			param:    "64",
			expected: "password deve ter no máximo 64",
		},
		{
			name:     "len tag",
			field:    "Code",
			tag:      "len",
			param:    "6",
			expected: "code deve ter exatamente 6 caracteres",
		},
		{
			name:     "oneof tag",
			field:    "Status",
			tag:      "oneof",
			param:    "active inactive",
			expected: "status deve ser um dos valores: active inactive",
		},
		{
			name:     "url tag",
			field:    "Website",
			tag:      "url",
			param:    "",
			expected: "website deve ser uma URL válida",
		},
		{
			name:     "uuid tag",
			field:    "ID",
			tag:      "uuid",
			param:    "",
			expected: "i_d deve ser um UUID válido",
		},
		{
			name:     "gte tag",
			field:    "Age",
			tag:      "gte",
			param:    "0",
			expected: "age deve ser maior ou igual a 0",
		},
		{
			name:     "lte tag",
			field:    "Age",
			tag:      "lte",
			param:    "150",
			expected: "age deve ser menor ou igual a 150",
		},
		{
			name:     "default unknown tag",
			field:    "Field",
			tag:      "alphanum",
			param:    "",
			expected: "field é inválido (alphanum)",
		},
		{
			name:     "default with camelCase field",
			field:    "UserName",
			tag:      "numeric",
			param:    "",
			expected: "user_name é inválido (numeric)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fe := mockFieldError{field: tt.field, tag: tt.tag, param: tt.param}
			got := formatSingleValidationError(fe)

			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatValidationErrors (integration via ValidatorMiddleware)
// ---------------------------------------------------------------------------

func TestFormatValidationErrors(t *testing.T) {
	t.Parallel()

	// Use ValidatorMiddleware which internally calls formatValidationErrors
	// when multiple fields fail validation. This avoids manual error handling
	// in the test handler (the middleware handles the c.AbortWithStatusJSON).
	r := gin.New()

	r.POST("/test", ValidatorMiddleware[testValidatorRequest](), func(c *gin.Context) {
		c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
	})

	w := httptest.NewRecorder()
	// Missing name and email, age=-1 (below gte=0) -> 3 validation errors
	body := `{"age":-1}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	resp := unmarshalAPIResponse(t, w)

	if resp.Error != "Erro de validação" {
		t.Errorf("expected error=%q, got %q", "Erro de validação", resp.Error)
	}

	// The message should contain multiple formatted errors joined by "; "
	if resp.Message == "" {
		t.Fatal("expected non-empty message with formatted validation errors")
	}

	// Verify that multiple error messages are present (joined with "; ")
	parts := strings.Split(resp.Message, "; ")
	if len(parts) < 2 {
		t.Errorf("expected multiple validation errors joined by '; ', got %q", resp.Message)
	}

	// Verify Portuguese error messages are formatted correctly
	if !strings.Contains(resp.Message, "obrigatório") {
		t.Errorf("expected message to contain 'obrigatório', got %q", resp.Message)
	}
}

// ---------------------------------------------------------------------------
// toSnakeCase
// ---------------------------------------------------------------------------

func TestToSnakeCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "name",
			expected: "name",
		},
		{
			name:     "camelCase",
			input:    "firstName",
			expected: "first_name",
		},
		{
			name:     "PascalCase",
			input:    "FirstName",
			expected: "first_name",
		},
		{
			name:     "multiple uppercase words",
			input:    "MyHTTPServer",
			expected: "my_h_t_t_p_server",
		},
		{
			name:     "all uppercase",
			input:    "ABC",
			expected: "a_b_c",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single lowercase char",
			input:    "a",
			expected: "a",
		},
		{
			name:     "single uppercase char",
			input:    "A",
			expected: "a",
		},
		{
			name:     "already snake_case",
			input:    "first_name",
			expected: "first_name",
		},
		{
			name:     "consecutive capitals in middle",
			input:    "getHTTPSUrl",
			expected: "get_h_t_t_p_s_url",
		},
		{
			name:     "trailing uppercase",
			input:    "nameID",
			expected: "name_i_d",
		},
		{
			name:     "numbers mixed in",
			input:    "field2Value",
			expected: "field2_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toSnakeCase(tt.input)
			if got != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RegisterCustomValidators
// ---------------------------------------------------------------------------

func TestRegisterCustomValidators(t *testing.T) {
	t.Parallel()

	// RegisterCustomValidators should not panic. If it does, the test
	// framework will catch the panic and report it as a failure automatically.
	// slog: test assertion — observability not applicable in unit test context
	RegisterCustomValidators()
}

func TestRegisterCustomValidators_TagNameFunc(t *testing.T) {
	t.Parallel()

	// After registering custom validators, the JSON tag name should be used
	// in validation error messages instead of the struct field name.
	RegisterCustomValidators()

	r := gin.New()

	r.POST("/test", ValidatorMiddleware[testValidatorRequest](), func(c *gin.Context) {
		c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
	})

	w := httptest.NewRecorder()
	body := `{"age":25}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	resp := unmarshalAPIResponse(t, w)

	// With custom tag name func, error should reference JSON field names
	if resp.Error != "Erro de validação" {
		t.Errorf("expected error=%q, got %q", "Erro de validação", resp.Error)
	}

	// Message should contain field names (name, email) derived from JSON tags
	if !strings.Contains(resp.Message, "obrigatório") {
		t.Errorf("expected message to contain 'obrigatório', got %q", resp.Message)
	}
}

// ---------------------------------------------------------------------------
// ValidateStruct
// ---------------------------------------------------------------------------

func TestValidateStruct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       interface{}
		expectNil   bool
		expectCount int
	}{
		{
			name: "valid struct returns nil",
			input: &testValidatorRequest{
				Name:  "Bruno",
				Email: "bruno@test.com",
				Age:   30,
			},
			expectNil: true,
		},
		{
			name: "invalid struct returns errors",
			input: &testValidatorRequest{
				Name:  "",
				Email: "not-an-email",
				Age:   -5,
			},
			expectNil:   false,
			expectCount: 3, // name required, email invalid, age gte
		},
		{
			name: "single validation error",
			input: &testValidatorRequest{
				Name:  "Bruno",
				Email: "bruno@test.com",
				Age:   200, // above lte=150
			},
			expectNil:   false,
			expectCount: 1,
		},
		{
			name: "missing all required fields",
			input: &testValidatorRequest{
				Age: 25,
			},
			expectNil:   false,
			expectCount: 2, // name required, email required
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errs := ValidateStruct(tt.input)

			if tt.expectNil {
				if errs != nil {
					t.Errorf("expected nil errors, got %v", errs)
				}
			} else {
				if errs == nil {
					t.Fatal("expected errors, got nil")
				}
				if len(errs) != tt.expectCount {
					t.Errorf("expected %d errors, got %d: %v", tt.expectCount, len(errs), errs)
				}
			}
		})
	}
}

func TestValidateStruct_NilInput(t *testing.T) {
	t.Parallel()

	// Passing nil to ValidateStruct should not panic. The validator library
	// returns an InvalidValidationError (not ValidationErrors) for nil input.
	// The function returns []string{err.Error()} for non-ValidationErrors,
	// or nil if the validator engine cast fails. Either way, no panic.
	// slog: test assertion — observability not applicable in unit test context
	errs := ValidateStruct(nil)
	_ = errs
}

func TestValidateStruct_ErrorMessageContent(t *testing.T) {
	t.Parallel()

	errs := ValidateStruct(&testValidatorRequest{
		Name:  "",
		Email: "bad",
		Age:   -1,
	})

	if errs == nil {
		t.Fatal("expected errors, got nil")
	}

	// Verify error messages contain expected Portuguese text
	joined := strings.Join(errs, "; ")

	expectedFragments := []string{"obrigatório", "email válido", "maior ou igual"}
	for _, frag := range expectedFragments {
		if !strings.Contains(joined, frag) {
			t.Errorf("expected errors to contain %q, got: %s", frag, joined)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidatorMiddleware - integration with min/max/len/url/uuid tags
// ---------------------------------------------------------------------------

func TestValidatorMiddleware_MinMaxValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		expectStatus int
		expectMsg    string
	}{
		{
			name:         "password too short fails min",
			body:         `{"password":"short"}`,
			expectStatus: http.StatusBadRequest,
			expectMsg:    "mínimo",
		},
		{
			name:         "password at min length passes",
			body:         `{"password":"12345678"}`,
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			r.POST("/test", ValidatorMiddleware[testMinMaxRequest](), func(c *gin.Context) {
				c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			if tt.expectMsg != "" {
				resp := unmarshalAPIResponse(t, w)
				if !strings.Contains(resp.Message, tt.expectMsg) {
					t.Errorf("expected message to contain %q, got %q", tt.expectMsg, resp.Message)
				}
			}
		})
	}
}

func TestValidatorMiddleware_LenValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		expectStatus int
		expectMsg    string
	}{
		{
			name:         "code with wrong length fails",
			body:         `{"code":"123"}`,
			expectStatus: http.StatusBadRequest,
			expectMsg:    "exatamente",
		},
		{
			name:         "code with exact length passes",
			body:         `{"code":"123456"}`,
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			r.POST("/test", ValidatorMiddleware[testLenRequest](), func(c *gin.Context) {
				c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			if tt.expectMsg != "" {
				resp := unmarshalAPIResponse(t, w)
				if !strings.Contains(resp.Message, tt.expectMsg) {
					t.Errorf("expected message to contain %q, got %q", tt.expectMsg, resp.Message)
				}
			}
		})
	}
}

func TestValidatorMiddleware_URLValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		expectStatus int
		expectMsg    string
	}{
		{
			name:         "invalid URL fails",
			body:         `{"website":"not-a-url"}`,
			expectStatus: http.StatusBadRequest,
			expectMsg:    "URL válida",
		},
		{
			name:         "valid URL passes",
			body:         `{"website":"https://example.com"}`,
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			r.POST("/test", ValidatorMiddleware[testURLRequest](), func(c *gin.Context) {
				c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			if tt.expectMsg != "" {
				resp := unmarshalAPIResponse(t, w)
				if !strings.Contains(resp.Message, tt.expectMsg) {
					t.Errorf("expected message to contain %q, got %q", tt.expectMsg, resp.Message)
				}
			}
		})
	}
}

func TestValidatorMiddleware_UUIDValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		expectStatus int
		expectMsg    string
	}{
		{
			name:         "invalid UUID fails",
			body:         `{"id":"not-a-uuid"}`,
			expectStatus: http.StatusBadRequest,
			expectMsg:    "UUID válido",
		},
		{
			name:         "valid UUID passes",
			body:         `{"id":"550e8400-e29b-41d4-a716-446655440000"}`,
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := gin.New()
			r.POST("/test", ValidatorMiddleware[testUUIDRequest](), func(c *gin.Context) {
				c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			if tt.expectMsg != "" {
				resp := unmarshalAPIResponse(t, w)
				if !strings.Contains(resp.Message, tt.expectMsg) {
					t.Errorf("expected message to contain %q, got %q", tt.expectMsg, resp.Message)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BindQuery - oneOf validation
// ---------------------------------------------------------------------------

func TestBindQuery_OneOfValidation(t *testing.T) {
	t.Parallel()

	r := gin.New()
	var gotErr error

	r.GET("/test", func(c *gin.Context) {
		_, bindErr := BindQuery[testQueryRequest](c)
		gotErr = bindErr
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test?page=1&limit=10&sort=invalid", nil)
	r.ServeHTTP(w, req)

	if gotErr == nil {
		t.Error("expected error for invalid oneof value, got nil")
	}
}

// ---------------------------------------------------------------------------
// RegisterCustomValidators — TagNameFunc with json:"-" tag
// ---------------------------------------------------------------------------

// testDashJSONTag is a struct where one field has json:"-" (excluded from JSON).
// When TagNameFunc encounters json:"-", it returns "" (empty string).
type testDashJSONTag struct {
	Visible string `json:"visible" binding:"required"`
	Hidden  string `json:"-" binding:"required"`
}

func TestRegisterCustomValidators_TagNameFunc_DashJsonTag(t *testing.T) {
	t.Parallel()

	// Ensure custom validators are registered (includes TagNameFunc).
	RegisterCustomValidators()

	r := gin.New()

	r.POST("/test", ValidatorMiddleware[testDashJSONTag](), func(c *gin.Context) {
		c.JSON(http.StatusOK, models.ApiResponse[any]{Success: true})
	})

	w := httptest.NewRecorder()
	// Provide "visible" but not "Hidden" (which has json:"-").
	// Since Hidden is required but its JSON name is "-", TagNameFunc returns "".
	// The validator should report a required error, but the field name
	// used in the error message will be empty (because TagNameFunc returned "").
	body := `{"visible":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// The request should fail validation because "Hidden" is required.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	resp := unmarshalAPIResponse(t, w)

	if resp.Error != "Erro de validação" {
		t.Errorf("expected error=%q, got %q", "Erro de validação", resp.Error)
	}

	// The message should contain a validation error. Because TagNameFunc returns ""
	// for json:"-" fields, the field name in the error should be empty or
	// the raw struct name (depending on validator behavior). The key point is
	// that the "-" branch was exercised and didn't panic.
	if resp.Message == "" {
		t.Error("expected non-empty validation message for dash-tagged field")
	}
}

func TestRegisterCustomValidators_TagNameFunc_DashJsonTag_DirectValidation(t *testing.T) {
	t.Parallel()

	// Ensure custom validators are registered.
	RegisterCustomValidators()

	// Validate a struct with a json:"-" field directly via ValidateStruct.
	errs := ValidateStruct(&testDashJSONTag{
		Visible: "present",
		Hidden:  "", // required field is empty → validation error
	})

	if errs == nil {
		t.Fatal("expected validation errors for empty Hidden field, got nil")
	}

	// The error should reference the field. When json tag is "-",
	// TagNameFunc returns "", so the field name in the error will be
	// either empty string or handled via toSnakeCase("").
	if len(errs) == 0 {
		t.Error("expected at least one validation error")
	}
}
