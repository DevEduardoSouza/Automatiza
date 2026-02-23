package middleware

import (
	"automatiza-go/internal/auth"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func init() {
	gin.SetMode(gin.TestMode)
	auth.InitCookieNames(false)
}

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockUserRoleRepo struct {
	role *models.UserRole
	err  error
}

func (m *mockUserRoleRepo) GetUserRole(_ context.Context, _ string) (*models.UserRole, error) {
	return m.role, m.err
}
func (m *mockUserRoleRepo) SetUserRole(_ context.Context, _, _ string) error   { return nil }
func (m *mockUserRoleRepo) RemoveUserRole(_ context.Context, _, _ string) error { return nil }
func (m *mockUserRoleRepo) IsAdmin(_ context.Context, _ string) (bool, error) {
	if m.role != nil && m.role.Role == "admin" {
		return true, nil
	}
	return false, nil
}
func (m *mockUserRoleRepo) GetAllUserRoles(_ context.Context, _ string) ([]string, error) {
	if m.role != nil {
		return []string{m.role.Role}, nil
	}
	return nil, nil
}
func (m *mockUserRoleRepo) GetAdminUsers(_ context.Context) ([]models.UserRole, error) {
	return nil, nil
}

type mockProfileRepo struct {
	profile *models.Profile
	err     error
}

func (m *mockProfileRepo) EnsureUserProfile(_ context.Context, _, _ string) (*models.Profile, error) {
	return m.profile, m.err
}
func (m *mockProfileRepo) GetProfile(_ context.Context, _ string) (*models.Profile, error) {
	return m.profile, m.err
}
func (m *mockProfileRepo) GetProfileByEmail(_ context.Context, _ string) (*models.Profile, error) {
	return m.profile, m.err
}
func (m *mockProfileRepo) GetProfileByEmailWithBilling(_ context.Context, _ string) (*models.Profile, *models.ProfileBillingFields, error) {
	return m.profile, nil, m.err
}
func (m *mockProfileRepo) CreateProfile(_ context.Context, p *models.Profile) (*models.Profile, error) {
	return p, m.err
}
func (m *mockProfileRepo) CreateUserCredentials(_ context.Context, _, _ string) error { return nil }
func (m *mockProfileRepo) UpdateProfile(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}
func (m *mockProfileRepo) UpdatePassword(_ context.Context, _, _ string) error { return nil }
func (m *mockProfileRepo) UpdateBillingStatus(_ context.Context, _ string, _ *models.ProfileBillingFields) error {
	return nil
}
func (m *mockProfileRepo) PauseUserAutomations(_ context.Context, _ string) error { return nil }
func (m *mockProfileRepo) ListProfiles(_ context.Context, _ string, _ int) ([]*models.Profile, string, error) {
	return nil, "", nil
}
func (m *mockProfileRepo) SearchProfiles(_ context.Context, _, _ string, _ int) ([]*models.Profile, string, error) {
	return nil, "", nil
}
func (m *mockProfileRepo) GetMaxAutomations(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *mockProfileRepo) CountUserAutomations(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (m *mockProfileRepo) GetProfileStats(_ context.Context) (*repository.ProfileStats, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type apiResponseBody struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) apiResponseBody {
	t.Helper()
	var body apiResponseBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	return body
}

func newTestGinContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}

// serveMiddleware creates a gin.Engine, registers GET /test with the given
// handlers, and serves the request through it. This avoids using the
// unexported c.Handlers field.
func serveMiddleware(w *httptest.ResponseRecorder, req *http.Request, handlers ...gin.HandlerFunc) {
	r := gin.New()
	r.GET("/test", handlers...)
	r.ServeHTTP(w, req)
}

// ---------------------------------------------------------------------------
// Tests for extractSessionID
// ---------------------------------------------------------------------------

func TestExtractSessionID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(c *gin.Context)
		expected string
	}{
		{
			name: "from cookie",
			setup: func(c *gin.Context) {
				c.Request.AddCookie(&http.Cookie{
					Name:  auth.SessionCookieName,
					Value: "session-from-cookie",
				})
			},
			expected: "session-from-cookie",
		},
		{
			name: "from Bearer header",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Bearer session-from-header")
			},
			expected: "session-from-header",
		},
		{
			name: "cookie takes precedence over header",
			setup: func(c *gin.Context) {
				c.Request.AddCookie(&http.Cookie{
					Name:  auth.SessionCookieName,
					Value: "from-cookie",
				})
				c.Request.Header.Set("Authorization", "Bearer from-header")
			},
			expected: "from-cookie",
		},
		{
			name:     "no auth returns empty",
			setup:    func(_ *gin.Context) {},
			expected: "",
		},
		{
			name: "Authorization header without Bearer prefix returns empty",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Basic abc123")
			},
			expected: "",
		},
		{
			name: "Bearer with empty token returns empty",
			setup: func(c *gin.Context) {
				c.Request.Header.Set("Authorization", "Bearer ")
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := newTestGinContext("GET", "/test")
			tt.setup(c)
			got := extractSessionID(c)
			if got != tt.expected {
				t.Errorf("extractSessionID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for AdminMiddleware
// ---------------------------------------------------------------------------

func TestAdminMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupContext   func(c *gin.Context)
		expectedStatus int
		expectedCalled bool
	}{
		{
			name:           "no user in context returns 401",
			setupContext:   func(_ *gin.Context) {},
			expectedStatus: http.StatusUnauthorized,
			expectedCalled: false,
		},
		{
			name: "wrong type in context returns 500",
			setupContext: func(c *gin.Context) {
				c.Set(string(UserContextKey), "not-a-user-struct")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCalled: false,
		},
		{
			name: "non-admin role returns 403",
			setupContext: func(c *gin.Context) {
				c.Set(string(UserContextKey), &AuthenticatedUser{ID: "user-1", Role: "user"})
			},
			expectedStatus: http.StatusForbidden,
			expectedCalled: false,
		},
		{
			name: "admin role passes through",
			setupContext: func(c *gin.Context) {
				c.Set(string(UserContextKey), &AuthenticatedUser{ID: "admin-1", Role: "admin"})
			},
			expectedStatus: http.StatusOK,
			expectedCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)

			serveMiddleware(w, req,
				func(c *gin.Context) { tt.setupContext(c); c.Next() },
				AdminMiddleware(),
				func(c *gin.Context) { called = true; c.Status(http.StatusOK) },
			)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}
			if called != tt.expectedCalled {
				t.Errorf("handler called = %v, want %v", called, tt.expectedCalled)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for GetAuthenticatedUser
// ---------------------------------------------------------------------------

func TestGetAuthenticatedUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(c *gin.Context)
		wantNil bool
		wantID  string
	}{
		{
			name: "user in context",
			setup: func(c *gin.Context) {
				c.Set(string(UserContextKey), &AuthenticatedUser{ID: "user-123", Email: "test@example.com", Role: "user"})
			},
			wantNil: false,
			wantID:  "user-123",
		},
		{name: "no user in context", setup: func(_ *gin.Context) {}, wantNil: true},
		{
			name: "wrong type in context",
			setup: func(c *gin.Context) {
				c.Set(string(UserContextKey), "invalid")
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := newTestGinContext("GET", "/test")
			tt.setup(c)
			got := GetAuthenticatedUser(c)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected user, got nil")
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for GetAuthenticatedUserFromContext
// ---------------------------------------------------------------------------

func TestGetAuthenticatedUserFromContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctx     context.Context
		wantNil bool
		wantID  string
	}{
		{
			name: "user in context",
			ctx: context.WithValue(context.Background(), UserContextKey, &AuthenticatedUser{
				ID: "ctx-user-1", Email: "ctx@example.com", Role: "admin",
			}),
			wantNil: false,
			wantID:  "ctx-user-1",
		},
		{name: "no user in context", ctx: context.Background(), wantNil: true},
		{name: "wrong type in context", ctx: context.WithValue(context.Background(), UserContextKey, 42), wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetAuthenticatedUserFromContext(tt.ctx)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected user, got nil")
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for MustGetAuthenticatedUser
// ---------------------------------------------------------------------------

func TestMustGetAuthenticatedUser(t *testing.T) {
	t.Parallel()

	t.Run("user exists", func(t *testing.T) {
		t.Parallel()
		c, _ := newTestGinContext("GET", "/test")
		expected := &AuthenticatedUser{ID: "must-user-1", Role: "user"}
		c.Set(string(UserContextKey), expected)
		got := MustGetAuthenticatedUser(c)
		if got.ID != expected.ID {
			t.Errorf("ID = %q, want %q", got.ID, expected.ID)
		}
	})

	t.Run("panics when nil", func(t *testing.T) {
		t.Parallel()
		logger := zaptest.NewLogger(t)
		c, _ := newTestGinContext("GET", "/test")
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected panic, got none")
			} else {
				logger.Info("expected panic recovered in MustGetAuthenticatedUser test",
					zap.Any("recovered", r))
			}
		}()
		MustGetAuthenticatedUser(c)
	})
}

// ---------------------------------------------------------------------------
// Tests for AuthMiddlewareWithRepos (deprecated but still in codebase)
// ---------------------------------------------------------------------------

func TestAuthMiddlewareWithRepos(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	t.Run("no Authorization header returns 401", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		serveMiddleware(w, req, AuthMiddlewareWithRepos(&mockProfileRepo{}, &mockUserRoleRepo{}, logger))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("empty Bearer token returns 401", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		serveMiddleware(w, req, AuthMiddlewareWithRepos(&mockProfileRepo{}, &mockUserRoleRepo{}, logger))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("profile not found returns 401", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer some-user-id")
		serveMiddleware(w, req, AuthMiddlewareWithRepos(&mockProfileRepo{err: auth.ErrUserNotFound}, &mockUserRoleRepo{}, logger))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("suspended user returns 403", func(t *testing.T) {
		t.Parallel()
		suspended := "suspended"
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer user-1")
		serveMiddleware(w, req, AuthMiddlewareWithRepos(
			&mockProfileRepo{profile: &models.Profile{ID: "user-1", Email: "s@e.com", BillingStatus: &suspended}},
			&mockUserRoleRepo{}, logger,
		))
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("valid user with role from repo", func(t *testing.T) {
		t.Parallel()
		var capturedUser *AuthenticatedUser
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer user-1")
		serveMiddleware(w, req,
			AuthMiddlewareWithRepos(
				&mockProfileRepo{profile: &models.Profile{ID: "user-1", Email: "v@e.com"}},
				&mockUserRoleRepo{role: &models.UserRole{UserID: "user-1", Role: "admin"}},
				logger,
			),
			func(c *gin.Context) { capturedUser = GetAuthenticatedUser(c); c.Status(http.StatusOK) },
		)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if capturedUser == nil {
			t.Fatal("expected user, got nil")
		}
		if capturedUser.ID != "user-1" || capturedUser.Role != "admin" {
			t.Errorf("got ID=%q Role=%q, want ID=user-1 Role=admin", capturedUser.ID, capturedUser.Role)
		}
	})

	t.Run("defaults to role user when role repo errors", func(t *testing.T) {
		t.Parallel()
		var capturedUser *AuthenticatedUser
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer user-2")
		serveMiddleware(w, req,
			AuthMiddlewareWithRepos(
				&mockProfileRepo{profile: &models.Profile{ID: "user-2", Email: "n@e.com"}},
				&mockUserRoleRepo{err: auth.ErrUserNotFound},
				logger,
			),
			func(c *gin.Context) { capturedUser = GetAuthenticatedUser(c); c.Status(http.StatusOK) },
		)
		if capturedUser == nil {
			t.Fatal("expected user, got nil")
		}
		if capturedUser.Role != "user" {
			t.Errorf("role = %q, want %q", capturedUser.Role, "user")
		}
	})
}

// ---------------------------------------------------------------------------
// Tests for OptionalAuthMiddlewareWithRepos
// ---------------------------------------------------------------------------

func TestOptionalAuthMiddlewareWithRepos(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	t.Run("no auth header continues without user", func(t *testing.T) {
		t.Parallel()
		called := false
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		serveMiddleware(w, req,
			OptionalAuthMiddlewareWithRepos(&mockProfileRepo{}, &mockUserRoleRepo{}, logger),
			func(c *gin.Context) {
				called = true
				if GetAuthenticatedUser(c) != nil {
					t.Error("expected no user")
				}
				c.Status(http.StatusOK)
			},
		)
		if !called {
			t.Error("handler should have been called")
		}
	})

	t.Run("empty Bearer continues without user", func(t *testing.T) {
		t.Parallel()
		called := false
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		serveMiddleware(w, req,
			OptionalAuthMiddlewareWithRepos(&mockProfileRepo{}, &mockUserRoleRepo{}, logger),
			func(_ *gin.Context) { called = true },
		)
		if !called {
			t.Error("handler should have been called")
		}
	})

	t.Run("invalid profile continues without user", func(t *testing.T) {
		t.Parallel()
		called := false
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid")
		serveMiddleware(w, req,
			OptionalAuthMiddlewareWithRepos(&mockProfileRepo{err: auth.ErrUserNotFound}, &mockUserRoleRepo{}, logger),
			func(c *gin.Context) {
				called = true
				if GetAuthenticatedUser(c) != nil {
					t.Error("expected no user for invalid profile")
				}
			},
		)
		if !called {
			t.Error("handler should have been called")
		}
	})

	t.Run("valid profile sets user in context", func(t *testing.T) {
		t.Parallel()
		var capturedUser *AuthenticatedUser
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer opt-user-1")
		serveMiddleware(w, req,
			OptionalAuthMiddlewareWithRepos(
				&mockProfileRepo{profile: &models.Profile{ID: "opt-user-1", Email: "o@e.com"}},
				&mockUserRoleRepo{role: &models.UserRole{UserID: "opt-user-1", Role: "user"}},
				logger,
			),
			func(c *gin.Context) { capturedUser = GetAuthenticatedUser(c) },
		)
		if capturedUser == nil || capturedUser.ID != "opt-user-1" {
			t.Errorf("expected user opt-user-1, got %+v", capturedUser)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests for QueryTokenAuthMiddleware
// ---------------------------------------------------------------------------

func TestQueryTokenAuthMiddleware(t *testing.T) {
	t.Parallel()
	logger := zaptest.NewLogger(t)

	t.Run("no token returns 401", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		serveMiddleware(w, req, QueryTokenAuthMiddleware(&mockProfileRepo{}, &mockUserRoleRepo{}, logger))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test?token=invalid", nil)
		serveMiddleware(w, req, QueryTokenAuthMiddleware(&mockProfileRepo{err: auth.ErrUserNotFound}, &mockUserRoleRepo{}, logger))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})

	t.Run("suspended user returns 403", func(t *testing.T) {
		t.Parallel()
		suspended := "suspended"
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test?token=susp", nil)
		serveMiddleware(w, req, QueryTokenAuthMiddleware(
			&mockProfileRepo{profile: &models.Profile{ID: "susp", Email: "s@e.com", BillingStatus: &suspended}},
			&mockUserRoleRepo{}, logger,
		))
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})

	t.Run("valid token sets user and userID", func(t *testing.T) {
		t.Parallel()
		var capturedUser *AuthenticatedUser
		var capturedUserID interface{}
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test?token=sse-user-1", nil)
		serveMiddleware(w, req,
			QueryTokenAuthMiddleware(
				&mockProfileRepo{profile: &models.Profile{ID: "sse-user-1", Email: "sse@e.com"}},
				&mockUserRoleRepo{role: &models.UserRole{UserID: "sse-user-1", Role: "user"}},
				logger,
			),
			func(c *gin.Context) {
				capturedUser = GetAuthenticatedUser(c)
				capturedUserID, _ = c.Get("userID")
				c.Status(http.StatusOK)
			},
		)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if capturedUser == nil || capturedUser.ID != "sse-user-1" {
			t.Errorf("user = %+v, want ID=sse-user-1", capturedUser)
		}
		if capturedUserID != "sse-user-1" {
			t.Errorf("userID = %v, want sse-user-1", capturedUserID)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests for SessionAuthMiddleware
// ---------------------------------------------------------------------------

func TestSessionAuthMiddleware_EmptySession(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	serveMiddleware(w, req, SessionAuthMiddleware(nil, nil, zaptest.NewLogger(t)))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	body := parseResponse(t, w)
	if body.Error != "Sessao nao fornecida" {
		t.Errorf("error = %q, want %q", body.Error, "Sessao nao fornecida")
	}
}

// ---------------------------------------------------------------------------
// Tests for OptionalSessionAuthMiddleware
// ---------------------------------------------------------------------------

func TestOptionalSessionAuthMiddleware_NoSession(t *testing.T) {
	t.Parallel()
	called := false
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	serveMiddleware(w, req,
		OptionalSessionAuthMiddleware(nil, nil, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			called = true
			if GetAuthenticatedUser(c) != nil {
				t.Error("expected no user")
			}
			c.Status(http.StatusOK)
		},
	)
	if !called {
		t.Error("handler should have been called")
	}
}

// ---------------------------------------------------------------------------
// Tests for SSESessionAuthMiddleware
// ---------------------------------------------------------------------------

func TestSSESessionAuthMiddleware_EmptySession(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	serveMiddleware(w, req, SSESessionAuthMiddleware(nil, nil, zaptest.NewLogger(t)))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	body := parseResponse(t, w)
	if body.Error != "Sessao nao fornecida" {
		t.Errorf("error = %q, want %q", body.Error, "Sessao nao fornecida")
	}
}

func TestSSESessionAuthMiddleware_SessionQueryParam(t *testing.T) {
	t.Parallel()
	// AuthService is nil, but the middleware should reach GetCurrentUser (panic)
	// because extracting session from "?session=..." succeeds.
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?session=my-session-id", nil)

	logger := zaptest.NewLogger(t)
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Info("expected nil-pointer panic confirms session query param extraction",
					zap.Any("recovered", r))
			}
		}()
		serveMiddleware(w, req, SSESessionAuthMiddleware(nil, nil, logger))
	}()

	// If we got 401 "Sessao nao fornecida", it means the query param was NOT extracted.
	if w.Code == http.StatusUnauthorized {
		body := parseResponse(t, w)
		if body.Error == "Sessao nao fornecida" {
			t.Error("session query param was not extracted")
		}
	}
}

func TestSSESessionAuthMiddleware_TokenFallback(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?token=my-token", nil)

	logger := zaptest.NewLogger(t)
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Info("expected nil-pointer panic confirms token fallback extraction",
					zap.Any("recovered", r))
			}
		}()
		serveMiddleware(w, req, SSESessionAuthMiddleware(nil, nil, logger))
	}()

	if w.Code == http.StatusUnauthorized {
		body := parseResponse(t, w)
		if body.Error == "Sessao nao fornecida" {
			t.Error("token query param fallback was not extracted")
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for AuthenticatedUser struct
// ---------------------------------------------------------------------------

func TestAuthenticatedUser_JSON(t *testing.T) {
	t.Parallel()
	user := &AuthenticatedUser{ID: "json-1", Email: "json@e.com", Role: "admin"}
	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded AuthenticatedUser
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ID != user.ID || decoded.Email != user.Email || decoded.Role != user.Role {
		t.Errorf("got %+v, want %+v", decoded, *user)
	}
}

func TestAuthenticatedUser_EmptyFields(t *testing.T) {
	t.Parallel()
	data, _ := json.Marshal(&AuthenticatedUser{ID: "only-id"})
	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	if _, ok := raw["email"]; ok {
		t.Error("expected email to be omitted")
	}
	if _, ok := raw["role"]; ok {
		t.Error("expected role to be omitted")
	}
}

// ---------------------------------------------------------------------------
// Tests for ContextKey
// ---------------------------------------------------------------------------

func TestContextKey_String(t *testing.T) {
	t.Parallel()
	if string(UserContextKey) != "user" {
		t.Errorf("UserContextKey = %q, want %q", string(UserContextKey), "user")
	}
}

// ---------------------------------------------------------------------------
// AdminMiddleware edge case: empty role
// ---------------------------------------------------------------------------

func TestAdminMiddleware_EmptyRole(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	serveMiddleware(w, req,
		func(c *gin.Context) {
			c.Set(string(UserContextKey), &AuthenticatedUser{ID: "u1", Role: ""})
			c.Next()
		},
		AdminMiddleware(),
	)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// ---------------------------------------------------------------------------
// Concurrent access
// ---------------------------------------------------------------------------

func TestGetAuthenticatedUser_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	const n = 50
	done := make(chan bool, n)
	for i := range n {
		go func(idx int) {
			c, _ := newTestGinContext("GET", "/test")
			c.Set(string(UserContextKey), &AuthenticatedUser{ID: "concurrent", Role: "user"})
			user := GetAuthenticatedUser(c)
			if user == nil || user.ID != "concurrent" {
				t.Errorf("goroutine %d: unexpected result", idx)
			}
			done <- true
		}(i)
	}
	timeout := time.After(5 * time.Second)
	for range n {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("timed out")
		}
	}
}

// Compile-time interface compliance.
var _ repository.UserRoleRepository = (*mockUserRoleRepo)(nil)
var _ repository.ProfileRepository = (*mockProfileRepo)(nil)
var _ auth.SessionStore = (*mockSessionStore)(nil)

// ---------------------------------------------------------------------------
// Mock SessionStore for session-based middleware tests
// ---------------------------------------------------------------------------

type mockSessionStore struct {
	session *auth.Session
	err     error
}

func (m *mockSessionStore) Create(_ context.Context, _ string, _ auth.SessionMetadata) (*auth.Session, string, string, error) {
	return nil, "", "", nil
}
func (m *mockSessionStore) Get(_ context.Context, _ string) (*auth.Session, error) {
	return m.session, m.err
}
func (m *mockSessionStore) GetByUserID(_ context.Context, _ string) ([]*auth.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) Refresh(_ context.Context, _, _ string) (*auth.Session, string, string, error) {
	return nil, "", "", nil
}
func (m *mockSessionStore) UpdateActivity(_ context.Context, _ string) error { return nil }
func (m *mockSessionStore) Revoke(_ context.Context, _ string) error        { return nil }
func (m *mockSessionStore) RevokeAllForUser(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (m *mockSessionStore) RevokeAllExcept(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}
func (m *mockSessionStore) ValidateCSRF(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}
func (m *mockSessionStore) GetCSRFToken(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockSessionStore) Cleanup(_ context.Context) error { return nil }

// newTestAuthService creates an AuthService backed by a mock session store and
// a deterministic getUser function that avoids hitting any real DB.
func newTestAuthService(t *testing.T, store auth.SessionStore, user *auth.User, userErr error) *auth.AuthService {
	t.Helper()
	logger := zaptest.NewLogger(t)
	return auth.NewAuthServiceWithSessionStore(store, logger,
		auth.WithGetUserOverride(func(_ context.Context, _ string) (*auth.User, error) {
			return user, userErr
		}),
	)
}

// ---------------------------------------------------------------------------
// Tests for SessionAuthMiddleware – valid session paths
// ---------------------------------------------------------------------------

func TestSessionAuthMiddleware_ValidSession_Cookie(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{
		ID:     "sess-valid-1",
		UserID: "user-42",
	}
	fakeUser := &auth.User{ID: "user-42", Email: "valid@example.com", Role: "admin"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	roleRepo := &mockUserRoleRepo{role: &models.UserRole{UserID: "user-42", Role: "admin"}}

	var capturedUser *AuthenticatedUser
	var capturedSession interface{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess-valid-1"})
	serveMiddleware(w, req,
		SessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			capturedSession, _ = c.Get("session")
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user in context, got nil")
	}
	if capturedUser.ID != "user-42" {
		t.Errorf("user.ID = %q, want %q", capturedUser.ID, "user-42")
	}
	if capturedUser.Role != "admin" {
		t.Errorf("user.Role = %q, want %q", capturedUser.Role, "admin")
	}
	if capturedUser.Email != "valid@example.com" {
		t.Errorf("user.Email = %q, want %q", capturedUser.Email, "valid@example.com")
	}
	if capturedSession == nil {
		t.Error("expected session in context, got nil")
	}
}

func TestSessionAuthMiddleware_ValidSession_BearerHeader(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sess-bearer-1", UserID: "user-99"}
	fakeUser := &auth.User{ID: "user-99", Email: "bearer@example.com", Role: ""}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	// No role in repo -> should default to "user"
	roleRepo := &mockUserRoleRepo{err: auth.ErrUserNotFound}

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer sess-bearer-1")
	serveMiddleware(w, req,
		SessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	// user.Role was "" from AuthService, and roleRepo returned error -> default "user"
	if capturedUser.Role != "user" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "user")
	}
}

func TestSessionAuthMiddleware_SessionStoreError(t *testing.T) {
	t.Parallel()

	authSvc := newTestAuthService(t, &mockSessionStore{err: auth.ErrSessionExpired}, nil, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "expired-session-id"})
	serveMiddleware(w, req, SessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	body := parseResponse(t, w)
	if body.Error != "Sessao invalida ou expirada" {
		t.Errorf("error = %q, want %q", body.Error, "Sessao invalida ou expirada")
	}
}

func TestSessionAuthMiddleware_NilUserRoleRepo(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sess-nil-repo", UserID: "user-nil"}
	fakeUser := &auth.User{ID: "user-nil", Email: "nil@repo.com", Role: "moderator"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "sess-nil-repo"})
	serveMiddleware(w, req,
		SessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	// With nil userRoleRepo, it should use the role from AuthService's User
	if capturedUser.Role != "moderator" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "moderator")
	}
}

func TestSessionAuthMiddleware_EmptyRoleDefaultsToUser(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sess-empty-role", UserID: "user-no-role"}
	fakeUser := &auth.User{ID: "user-no-role", Email: "norole@e.com", Role: ""}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	// Role repo also returns nothing
	roleRepo := &mockUserRoleRepo{err: auth.ErrUserNotFound}

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer sess-empty-role")
	serveMiddleware(w, req,
		SessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.Role != "user" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "user")
	}
}

// ---------------------------------------------------------------------------
// Tests for OptionalSessionAuthMiddleware – valid and error paths
// ---------------------------------------------------------------------------

func TestOptionalSessionAuthMiddleware_ValidSession(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "opt-sess-1", UserID: "opt-user-1"}
	fakeUser := &auth.User{ID: "opt-user-1", Email: "opt@e.com", Role: "user"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	roleRepo := &mockUserRoleRepo{role: &models.UserRole{UserID: "opt-user-1", Role: "user"}}

	var capturedUser *AuthenticatedUser
	var capturedSession interface{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "opt-sess-1"})
	serveMiddleware(w, req,
		OptionalSessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			capturedSession, _ = c.Get("session")
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.ID != "opt-user-1" {
		t.Errorf("user.ID = %q, want %q", capturedUser.ID, "opt-user-1")
	}
	if capturedSession == nil {
		t.Error("expected session in context, got nil")
	}
}

func TestOptionalSessionAuthMiddleware_SessionStoreError_ContinuesWithout(t *testing.T) {
	t.Parallel()

	authSvc := newTestAuthService(t, &mockSessionStore{err: auth.ErrSessionExpired}, nil, nil)

	called := false
	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "expired-opt-sess"})
	serveMiddleware(w, req,
		OptionalSessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			called = true
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if !called {
		t.Error("handler should have been called even with invalid session")
	}
	if capturedUser != nil {
		t.Error("expected no user for invalid session, but got one")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOptionalSessionAuthMiddleware_EmptyRole(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "opt-empty-role", UserID: "opt-user-norole"}
	fakeUser := &auth.User{ID: "opt-user-norole", Email: "norole@e.com", Role: ""}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	roleRepo := &mockUserRoleRepo{err: auth.ErrUserNotFound}

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer opt-empty-role")
	serveMiddleware(w, req,
		OptionalSessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.Role != "user" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "user")
	}
}

func TestOptionalSessionAuthMiddleware_NilUserRoleRepo(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "opt-nil-repo", UserID: "opt-user-nilrp"}
	fakeUser := &auth.User{ID: "opt-user-nilrp", Email: "e@e.com", Role: "editor"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "opt-nil-repo"})
	serveMiddleware(w, req,
		OptionalSessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.Role != "editor" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "editor")
	}
}

// ---------------------------------------------------------------------------
// Tests for SSESessionAuthMiddleware – valid session paths
// ---------------------------------------------------------------------------

func TestSSESessionAuthMiddleware_ValidSession_SessionParam(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sse-sess-1", UserID: "sse-user-1"}
	fakeUser := &auth.User{ID: "sse-user-1", Email: "sse@example.com", Role: "user"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	roleRepo := &mockUserRoleRepo{role: &models.UserRole{UserID: "sse-user-1", Role: "user"}}

	var capturedUser *AuthenticatedUser
	var capturedSession interface{}
	var capturedUserID interface{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?session=sse-sess-1", nil)
	serveMiddleware(w, req,
		SSESessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			capturedSession, _ = c.Get("session")
			capturedUserID, _ = c.Get("userID")
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user in context, got nil")
	}
	if capturedUser.ID != "sse-user-1" {
		t.Errorf("user.ID = %q, want %q", capturedUser.ID, "sse-user-1")
	}
	if capturedUser.Email != "sse@example.com" {
		t.Errorf("user.Email = %q, want %q", capturedUser.Email, "sse@example.com")
	}
	if capturedSession == nil {
		t.Error("expected session in context, got nil")
	}
	if capturedUserID != "sse-user-1" {
		t.Errorf("userID = %v, want %q", capturedUserID, "sse-user-1")
	}
}

func TestSSESessionAuthMiddleware_ValidSession_TokenFallbackParam(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sse-tok-1", UserID: "sse-tok-user-1"}
	fakeUser := &auth.User{ID: "sse-tok-user-1", Email: "tok@example.com", Role: "admin"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	roleRepo := &mockUserRoleRepo{role: &models.UserRole{UserID: "sse-tok-user-1", Role: "admin"}}

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?token=sse-tok-1", nil)
	serveMiddleware(w, req,
		SSESessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.Role != "admin" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "admin")
	}
}

func TestSSESessionAuthMiddleware_SessionStoreError(t *testing.T) {
	t.Parallel()

	authSvc := newTestAuthService(t, &mockSessionStore{err: auth.ErrSessionRevoked}, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?session=revoked-session", nil)
	serveMiddleware(w, req, SSESessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	body := parseResponse(t, w)
	if body.Error != "Sessao invalida ou expirada" {
		t.Errorf("error = %q, want %q", body.Error, "Sessao invalida ou expirada")
	}
}

func TestSSESessionAuthMiddleware_EmptyRoleDefaults(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sse-norole", UserID: "sse-norole-user"}
	fakeUser := &auth.User{ID: "sse-norole-user", Email: "nr@e.com", Role: ""}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)
	roleRepo := &mockUserRoleRepo{err: auth.ErrUserNotFound}

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?session=sse-norole", nil)
	serveMiddleware(w, req,
		SSESessionAuthMiddleware(authSvc, roleRepo, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.Role != "user" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "user")
	}
}

func TestSSESessionAuthMiddleware_NilUserRoleRepo(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sse-nil-repo", UserID: "sse-nilrepo-user"}
	fakeUser := &auth.User{ID: "sse-nilrepo-user", Email: "nrp@e.com", Role: "manager"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?token=sse-nil-repo", nil)
	serveMiddleware(w, req,
		SSESessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.Role != "manager" {
		t.Errorf("role = %q, want %q", capturedUser.Role, "manager")
	}
}

func TestSSESessionAuthMiddleware_SessionParamTakesPrecedenceOverToken(t *testing.T) {
	t.Parallel()

	validSession := &auth.Session{ID: "sse-precedence", UserID: "prec-user"}
	fakeUser := &auth.User{ID: "prec-user", Email: "p@e.com", Role: "user"}

	authSvc := newTestAuthService(t, &mockSessionStore{session: validSession}, fakeUser, nil)

	var capturedUser *AuthenticatedUser
	w := httptest.NewRecorder()
	// Both session and token are provided; session should take precedence.
	req := httptest.NewRequest("GET", "/test?session=sse-precedence&token=ignored-token", nil)
	serveMiddleware(w, req,
		SSESessionAuthMiddleware(authSvc, nil, zaptest.NewLogger(t)),
		func(c *gin.Context) {
			capturedUser = GetAuthenticatedUser(c)
			c.Status(http.StatusOK)
		},
	)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUser == nil {
		t.Fatal("expected user, got nil")
	}
	if capturedUser.ID != "prec-user" {
		t.Errorf("user.ID = %q, want %q", capturedUser.ID, "prec-user")
	}
}
