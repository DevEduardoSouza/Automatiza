// Package connectrpc provides ConnectRPC server setup and configuration.
package connectrpc

import (
	"net/http"

	"automatiza-go/internal/auth"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// Server holds the ConnectRPC server configuration and handlers.
type Server struct {
	mux               *http.ServeMux
	authService       *auth.AuthService
	cookieConfig      *auth.CookieConfig
	suspensionChecker SuspensionChecker
	logger            *zap.Logger
}

// ServerConfig holds configuration for creating a new Server.
type ServerConfig struct {
	AuthService       *auth.AuthService
	CookieConfig      *auth.CookieConfig
	ProfileRepo       repository.ProfileRepository
	SuspensionChecker SuspensionChecker
	Logger            *zap.Logger
}

// NewServer creates a new ConnectRPC server.
func NewServer(cfg *ServerConfig) *Server {
	s := &Server{
		mux:               http.NewServeMux(),
		authService:       cfg.AuthService,
		cookieConfig:      cfg.CookieConfig,
		suspensionChecker: cfg.SuspensionChecker,
		logger:            cfg.Logger,
	}

	// Create default suspension checker if not provided
	if s.suspensionChecker == nil && cfg.ProfileRepo != nil {
		s.suspensionChecker = NewDBSuspensionChecker(cfg.ProfileRepo, cfg.Logger)
	}

	return s
}

// Handler returns the HTTP handler for the ConnectRPC server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// RegisterService registers a ConnectRPC service handler with interceptors.
func (s *Server) RegisterService(path string, handler http.Handler) {
	s.mux.Handle(path, handler)
}

// DefaultInterceptors returns the standard set of interceptors for ConnectRPC services.
// Chain order: Recovery -> Tracing -> Sentry -> Logging -> Validation
func (s *Server) DefaultInterceptors() []connect.Interceptor {
	return []connect.Interceptor{
		observability.NewSentryRecoveryInterceptor(),
		NewTracingInterceptor(s.logger),
		observability.NewSentryInterceptor(),
		NewLoggingInterceptor(s.logger),
		NewValidationInterceptor(),
	}
}

// AuthInterceptors returns interceptors including authentication and suspension check.
// Chain order: Recovery -> Tracing -> Sentry -> Logging -> Auth -> Suspension -> CSRF -> Validation
func (s *Server) AuthInterceptors() []connect.Interceptor {
	return []connect.Interceptor{
		observability.NewSentryRecoveryInterceptor(),
		NewTracingInterceptor(s.logger),
		observability.NewSentryInterceptor(),
		NewLoggingInterceptor(s.logger),
		NewAuthInterceptor(s.authService, s.cookieConfig),
		NewSuspensionInterceptor(s.suspensionChecker, s.logger),
		NewCSRFInterceptor(s.authService),
		NewValidationInterceptor(),
	}
}

// AuthInterceptorsWithoutSuspension returns interceptors without suspension check.
// Use this for endpoints that suspended users should still be able to access.
func (s *Server) AuthInterceptorsWithoutSuspension() []connect.Interceptor {
	return []connect.Interceptor{
		observability.NewSentryRecoveryInterceptor(),
		NewTracingInterceptor(s.logger),
		observability.NewSentryInterceptor(),
		NewLoggingInterceptor(s.logger),
		NewAuthInterceptor(s.authService, s.cookieConfig),
		NewCSRFInterceptor(s.authService),
		NewValidationInterceptor(),
	}
}

// WithInterceptors creates connect.HandlerOption with the given interceptors.
func WithInterceptors(interceptors ...connect.Interceptor) connect.HandlerOption {
	return connect.WithInterceptors(interceptors...)
}
