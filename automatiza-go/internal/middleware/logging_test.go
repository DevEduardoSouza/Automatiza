package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// LoggingMiddleware
// ---------------------------------------------------------------------------

func TestLoggingMiddleware_StatusLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		statusCode    int
		expectedLevel zapcore.Level
		expectedMsg   string
	}{
		{
			name:          "200 status logs Info",
			statusCode:    http.StatusOK,
			expectedLevel: zapcore.InfoLevel,
			expectedMsg:   "Request completed",
		},
		{
			name:          "201 status logs Info",
			statusCode:    http.StatusCreated,
			expectedLevel: zapcore.InfoLevel,
			expectedMsg:   "Request completed",
		},
		{
			name:          "400 status logs Warn",
			statusCode:    http.StatusBadRequest,
			expectedLevel: zapcore.WarnLevel,
			expectedMsg:   "Request error",
		},
		{
			name:          "404 status logs Warn",
			statusCode:    http.StatusNotFound,
			expectedLevel: zapcore.WarnLevel,
			expectedMsg:   "Request error",
		},
		{
			name:          "500 status logs Error",
			statusCode:    http.StatusInternalServerError,
			expectedLevel: zapcore.ErrorLevel,
			expectedMsg:   "Request failed",
		},
		{
			name:          "503 status logs Error",
			statusCode:    http.StatusServiceUnavailable,
			expectedLevel: zapcore.ErrorLevel,
			expectedMsg:   "Request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core, recorded := observer.New(zapcore.DebugLevel)
			logger := zap.New(core)

			w := httptest.NewRecorder()
			c, engine := gin.CreateTestContext(w)

			engine.Use(LoggingMiddleware(logger))
			engine.GET("/test", func(c *gin.Context) {
				c.Status(tt.statusCode)
			})

			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
			engine.ServeHTTP(w, c.Request)

			entries := recorded.All()
			if len(entries) == 0 {
				t.Fatal("expected at least one log entry, got none")
			}

			last := entries[len(entries)-1]
			if last.Level != tt.expectedLevel {
				t.Errorf("expected log level %v, got %v", tt.expectedLevel, last.Level)
			}
			if last.Message != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, last.Message)
			}
		})
	}
}

func TestLoggingMiddleware_WithQueryParams(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(LoggingMiddleware(logger))
	engine.GET("/search", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=hello&page=1", nil)
	engine.ServeHTTP(w, req)

	entries := recorded.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	last := entries[len(entries)-1]
	found := false
	for _, field := range last.Context {
		if field.Key == "query" {
			found = true
			if field.String != "q=hello&page=1" {
				t.Errorf("expected query %q, got %q", "q=hello&page=1", field.String)
			}
		}
	}
	if !found {
		t.Error("expected 'query' field in log entry")
	}
}

func TestLoggingMiddleware_WithAuthenticatedUser(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(func(c *gin.Context) {
		c.Set(string(UserContextKey), &AuthenticatedUser{ID: "user-123", Email: "test@example.com"})
		c.Next()
	})
	engine.Use(LoggingMiddleware(logger))
	engine.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	engine.ServeHTTP(w, req)

	entries := recorded.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	last := entries[len(entries)-1]
	found := false
	for _, field := range last.Context {
		if field.Key == "user_id" {
			found = true
			if field.String != "user-123" {
				t.Errorf("expected user_id %q, got %q", "user-123", field.String)
			}
		}
	}
	if !found {
		t.Error("expected 'user_id' field in log entry")
	}
}

func TestLoggingMiddleware_WithErrorsInContext(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(LoggingMiddleware(logger))
	engine.GET("/err", func(c *gin.Context) {
		_ = c.Error(&ApiError{StatusCode: 500, Message: "something broke"})
		c.Status(http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	engine.ServeHTTP(w, req)

	entries := recorded.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	last := entries[len(entries)-1]
	found := false
	for _, field := range last.Context {
		if field.Key == "error" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'error' field in log entry when errors exist in context")
	}
}

func TestLoggingMiddleware_NoQueryParam(t *testing.T) {
	t.Parallel()

	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(LoggingMiddleware(logger))
	engine.GET("/no-query", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/no-query", nil)
	engine.ServeHTTP(w, req)

	entries := recorded.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	last := entries[len(entries)-1]
	for _, field := range last.Context {
		if field.Key == "query" {
			t.Error("did not expect 'query' field in log entry when no query params")
		}
	}
}

// ---------------------------------------------------------------------------
// RequestIDMiddleware
// ---------------------------------------------------------------------------

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		incomingHeaderID string
		expectGenerated  bool
	}{
		{
			name:             "generates ID when no header",
			incomingHeaderID: "",
			expectGenerated:  true,
		},
		{
			name:             "preserves X-Request-ID header",
			incomingHeaderID: "custom-req-id-abc",
			expectGenerated:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			_, engine := gin.CreateTestContext(w)

			var capturedID string
			engine.Use(RequestIDMiddleware())
			engine.GET("/test", func(c *gin.Context) {
				capturedID = GetRequestID(c)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.incomingHeaderID != "" {
				req.Header.Set("X-Request-ID", tt.incomingHeaderID)
			}
			engine.ServeHTTP(w, req)

			// Should always have a request ID in response header
			respID := w.Header().Get("X-Request-ID")
			if respID == "" {
				t.Error("expected X-Request-ID response header to be set")
			}

			if tt.expectGenerated {
				if capturedID == "" {
					t.Error("expected a generated request ID in context")
				}
			} else {
				if capturedID != tt.incomingHeaderID {
					t.Errorf("expected preserved request ID %q, got %q", tt.incomingHeaderID, capturedID)
				}
				if respID != tt.incomingHeaderID {
					t.Errorf("expected response header %q, got %q", tt.incomingHeaderID, respID)
				}
			}
		})
	}
}

func TestRequestIDMiddleware_SetsHeaderInResponse(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)

	engine.Use(RequestIDMiddleware())
	engine.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	respID := w.Header().Get("X-Request-ID")
	if respID == "" {
		t.Error("expected X-Request-ID to be set in response")
	}
}

// ---------------------------------------------------------------------------
// generateRequestID
// ---------------------------------------------------------------------------

func TestGenerateRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		check func(t *testing.T, id string)
	}{
		{
			name: "returns non-empty string",
			check: func(t *testing.T, id string) {
				t.Helper()
				if id == "" {
					t.Error("expected non-empty request ID")
				}
			},
		},
		{
			name: "format is timestamp-based",
			check: func(t *testing.T, id string) {
				t.Helper()
				// Format: "20060102150405.000000" => 21 chars
				if len(id) < 15 {
					t.Errorf("expected timestamp-based ID with at least 15 chars, got %d chars: %q", len(id), id)
				}
				// Should contain a dot separating date from microseconds
				hasDot := false
				for _, ch := range id {
					if ch == '.' {
						hasDot = true
						break
					}
				}
				if !hasDot {
					t.Errorf("expected timestamp-based ID to contain a dot, got %q", id)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := generateRequestID()
			tt.check(t, id)
		})
	}
}

// ---------------------------------------------------------------------------
// GetRequestID
// ---------------------------------------------------------------------------

func TestGetRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(c *gin.Context)
		expected string
	}{
		{
			name: "returns ID when set",
			setup: func(c *gin.Context) {
				c.Set("request_id", "my-request-id")
			},
			expected: "my-request-id",
		},
		{
			name:     "returns empty when not set",
			setup:    func(c *gin.Context) {},
			expected: "",
		},
		{
			name: "returns empty when wrong type",
			setup: func(c *gin.Context) {
				c.Set("request_id", 12345)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			tt.setup(c)

			got := GetRequestID(c)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
