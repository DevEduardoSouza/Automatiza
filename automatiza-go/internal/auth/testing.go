package auth

import (
	"context"

	"go.uber.org/zap"
)

// getUserOverride is a test hook that allows overriding the getUser method.
// When non-nil, GetCurrentUser uses this function instead of querying the DB.
// This field is only set via NewAuthServiceWithSessionStore (for tests).
type getUserOverrideFunc func(ctx context.Context, userID string) (*User, error)

// AuthServiceTestOption configures an AuthService for testing.
type AuthServiceTestOption func(s *AuthService)

// WithGetUserOverride sets a function that overrides the DB-backed getUser method.
func WithGetUserOverride(fn getUserOverrideFunc) AuthServiceTestOption {
	return func(s *AuthService) {
		s.getUserOverride = fn
	}
}

// NewAuthServiceWithSessionStore creates an AuthService with a custom SessionStore.
// This is exported for use in tests in other packages (e.g., middleware tests)
// that need to inject a mock session store without real DB/Redis.
func NewAuthServiceWithSessionStore(store SessionStore, logger *zap.Logger, opts ...AuthServiceTestOption) *AuthService {
	svc := &AuthService{
		sessionStore: store,
		logger:       logger,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}
