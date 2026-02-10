package connectrpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"automatiza-go/internal/auth"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Context keys for auth data
type contextKey string

const (
	// SessionContextKey is the context key for session data.
	SessionContextKey contextKey = "session"
	// UserContextKey is the context key for user data.
	UserContextKey contextKey = "user"
	// TraceIDContextKey is the context key for trace ID.
	TraceIDContextKey contextKey = "traceID"
	// ClientIPContextKey is the context key for client IP address.
	ClientIPContextKey contextKey = "clientIP"
	// UserAgentContextKey is the context key for user agent.
	UserAgentContextKey contextKey = "userAgent"
)

// GetSessionFromContext retrieves the session from context.
func GetSessionFromContext(ctx context.Context) *auth.Session {
	session, _ := ctx.Value(SessionContextKey).(*auth.Session)
	return session
}

// GetUserFromContext retrieves the user from context.
func GetUserFromContext(ctx context.Context) *auth.User {
	user, _ := ctx.Value(UserContextKey).(*auth.User)
	return user
}

// GetTraceIDFromContext retrieves the trace ID from context.
func GetTraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(TraceIDContextKey).(string)
	return traceID
}

// GetClientIPFromContext retrieves the client IP from context.
func GetClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(ClientIPContextKey).(string)
	return ip
}

// GetUserAgentFromContext retrieves the user agent from context.
func GetUserAgentFromContext(ctx context.Context) string {
	ua, _ := ctx.Value(UserAgentContextKey).(string)
	return ua
}

// RecoveryInterceptor recovers from panics and returns an internal error.
type RecoveryInterceptor struct {
	logger *zap.Logger
}

// NewRecoveryInterceptor creates a new recovery interceptor.
func NewRecoveryInterceptor(logger *zap.Logger) connect.Interceptor {
	return &RecoveryInterceptor{logger: logger}
}

func (i *RecoveryInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
		defer func() {
			if r := recover(); r != nil {
				traceID := GetTraceIDFromContext(ctx)
				i.logger.Error("Panic recovered",
					zap.String("traceID", traceID),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				err = connect.NewError(connect.CodeInternal, fmt.Errorf("internal server error"))
			}
		}()
		return next(ctx, req)
	}
}

func (i *RecoveryInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *RecoveryInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) (err error) {
		defer func() {
			if r := recover(); r != nil {
				traceID := GetTraceIDFromContext(ctx)
				i.logger.Error("Panic recovered in streaming handler",
					zap.String("traceID", traceID),
					zap.Any("panic", r),
					zap.String("stack", string(debug.Stack())),
				)
				err = connect.NewError(connect.CodeInternal, fmt.Errorf("internal server error"))
			}
		}()
		return next(ctx, conn)
	}
}

// TracingInterceptor extracts and propagates trace context.
type TracingInterceptor struct {
	logger *zap.Logger
}

// NewTracingInterceptor creates a new tracing interceptor.
func NewTracingInterceptor(logger *zap.Logger) connect.Interceptor {
	return &TracingInterceptor{logger: logger}
}

func (i *TracingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Extract or generate trace ID
		traceID := extractTraceID(req.Header())
		if traceID == "" {
			traceID = generateTraceID()
		}

		// Add trace ID to context
		ctx = context.WithValue(ctx, TraceIDContextKey, traceID)

		// Extract client IP and User-Agent for audit logging
		clientIP := extractClientIP(req.Header())
		if clientIP == "" {
			clientIP = "unknown"
		}
		ctx = context.WithValue(ctx, ClientIPContextKey, clientIP)

		userAgent := req.Header().Get("User-Agent")
		ctx = context.WithValue(ctx, UserAgentContextKey, userAgent)

		return next(ctx, req)
	}
}

func (i *TracingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *TracingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		traceID := extractTraceID(conn.RequestHeader())
		if traceID == "" {
			traceID = generateTraceID()
		}
		ctx = context.WithValue(ctx, TraceIDContextKey, traceID)

		// Extract client IP and User-Agent for audit logging
		clientIP := extractClientIP(conn.RequestHeader())
		if clientIP == "" {
			clientIP = "unknown"
		}
		ctx = context.WithValue(ctx, ClientIPContextKey, clientIP)

		userAgent := conn.RequestHeader().Get("User-Agent")
		ctx = context.WithValue(ctx, UserAgentContextKey, userAgent)

		return next(ctx, conn)
	}
}

func extractTraceID(header http.Header) string {
	// Try W3C Trace Context format
	traceparent := header.Get("traceparent")
	if traceparent != "" {
		parts := strings.Split(traceparent, "-")
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Fallback to custom header
	return header.Get("x-trace-id")
}

func generateTraceID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// LoggingInterceptor logs requests and responses.
type LoggingInterceptor struct {
	logger *zap.Logger
}

// NewLoggingInterceptor creates a new logging interceptor.
func NewLoggingInterceptor(logger *zap.Logger) connect.Interceptor {
	return &LoggingInterceptor{logger: logger}
}

func (i *LoggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		traceID := GetTraceIDFromContext(ctx)

		resp, err := next(ctx, req)

		duration := time.Since(start)
		procedure := req.Spec().Procedure

		if err != nil {
			connectErr := new(connect.Error)
			code := connect.CodeUnknown
			if connect.CodeOf(err) != connect.CodeUnknown {
				code = connect.CodeOf(err)
			}

			i.logger.Warn("RPC failed",
				zap.String("traceID", traceID),
				zap.String("procedure", procedure),
				zap.String("code", code.String()),
				zap.Duration("duration", duration),
				zap.Error(connectErr),
			)
		} else {
			i.logger.Debug("RPC completed",
				zap.String("traceID", traceID),
				zap.String("procedure", procedure),
				zap.Duration("duration", duration),
			)
		}

		return resp, err
	}
}

func (i *LoggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *LoggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		traceID := GetTraceIDFromContext(ctx)
		procedure := conn.Spec().Procedure

		err := next(ctx, conn)

		duration := time.Since(start)

		if err != nil {
			i.logger.Warn("Streaming RPC failed",
				zap.String("traceID", traceID),
				zap.String("procedure", procedure),
				zap.Duration("duration", duration),
				zap.Error(err),
			)
		} else {
			i.logger.Debug("Streaming RPC completed",
				zap.String("traceID", traceID),
				zap.String("procedure", procedure),
				zap.Duration("duration", duration),
			)
		}

		return err
	}
}

// AuthInterceptor extracts session from cookies and validates authentication.
type AuthInterceptor struct {
	authService  *auth.AuthService
	cookieConfig *auth.CookieConfig
}

// NewAuthInterceptor creates a new auth interceptor.
func NewAuthInterceptor(authService *auth.AuthService, cookieConfig *auth.CookieConfig) connect.Interceptor {
	return &AuthInterceptor{
		authService:  authService,
		cookieConfig: cookieConfig,
	}
}

// Public endpoints that don't require authentication
var publicEndpoints = map[string]bool{
	"/auth.v1.AuthService/Login":          true,
	"/auth.v1.AuthService/Register":       true,
	"/auth.v1.AuthService/ForgotPassword": true,
	"/auth.v1.AuthService/ResetPassword":  true,
	"/auth.v1.AuthService/VerifyEmail":    true,
	"/auth.v1.AuthService/GetCsrfToken":   true,
	// Password reset via Profiles service (no auth required)
	"/profiles.v1.ProfilesService/RequestPasswordReset": true,
	"/profiles.v1.ProfilesService/ValidateResetToken":   true,
	"/profiles.v1.ProfilesService/ResetPassword":        true,
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure

		// Skip auth for public endpoints
		if publicEndpoints[procedure] {
			return next(ctx, req)
		}

		// Extract session cookie from header
		// ConnectRPC passes cookies in the Cookie header
		cookieHeader := req.Header().Get("Cookie")
		sessionID := extractSessionCookie(cookieHeader)

		if sessionID == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("session required"))
		}

		// Validate session
		user, session, err := i.authService.GetCurrentUser(ctx, sessionID)
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid session"))
		}

		// Add to context
		ctx = context.WithValue(ctx, SessionContextKey, session)
		ctx = context.WithValue(ctx, UserContextKey, user)

		return next(ctx, req)
	}
}

func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure

		if publicEndpoints[procedure] {
			return next(ctx, conn)
		}

		cookieHeader := conn.RequestHeader().Get("Cookie")
		sessionID := extractSessionCookie(cookieHeader)

		if sessionID == "" {
			return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("session required"))
		}

		user, session, err := i.authService.GetCurrentUser(ctx, sessionID)
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid session"))
		}

		ctx = context.WithValue(ctx, SessionContextKey, session)
		ctx = context.WithValue(ctx, UserContextKey, user)

		return next(ctx, conn)
	}
}

func extractSessionCookie(cookieHeader string) string {
	if cookieHeader == "" {
		return ""
	}

	// Parse cookies
	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		parts := strings.SplitN(cookie, "=", 2)
		if len(parts) == 2 && parts[0] == auth.SessionCookieName {
			return parts[1]
		}
	}

	return ""
}

// CSRFInterceptor validates CSRF tokens for mutating operations.
type CSRFInterceptor struct {
	authService *auth.AuthService
}

// NewCSRFInterceptor creates a new CSRF interceptor.
func NewCSRFInterceptor(authService *auth.AuthService) connect.Interceptor {
	return &CSRFInterceptor{authService: authService}
}

func (i *CSRFInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure

		// Skip CSRF for exempt paths
		if auth.IsExemptPath(procedure) {
			return next(ctx, req)
		}

		// Only validate for mutating methods (POST)
		// All ConnectRPC requests are POST, so check if it's a read-only operation
		if isReadOnlyProcedure(procedure) {
			return next(ctx, req)
		}

		// Get session from context
		session := GetSessionFromContext(ctx)
		if session == nil {
			// Auth interceptor should have caught this
			return next(ctx, req)
		}

		// Get CSRF token from header
		csrfToken := req.Header().Get(auth.CSRFHeaderName)
		if csrfToken == "" {
			csrfToken = req.Header().Get(auth.CSRFHeaderNameAlt)
		}

		if csrfToken == "" {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("CSRF token required"))
		}

		// Validate CSRF token
		valid, err := i.authService.ValidateCSRF(ctx, session.ID, csrfToken)
		if err != nil || !valid {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("invalid CSRF token"))
		}

		return next(ctx, req)
	}
}

func (i *CSRFInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *CSRFInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // Streaming typically doesn't need CSRF
}

// isReadOnlyProcedure checks if a procedure is read-only based on naming convention.
func isReadOnlyProcedure(procedure string) bool {
	// Extract method name from procedure path
	parts := strings.Split(procedure, "/")
	if len(parts) < 2 {
		return false
	}
	method := parts[len(parts)-1]

	readOnlyPrefixes := []string{"Get", "List", "Check", "Fetch", "Search", "Find", "Count"}
	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}
	return false
}

// ValidationInterceptor validates request messages using protovalidate.
type ValidationInterceptor struct {
	validator protovalidate.Validator
}

// NewValidationInterceptor creates a new validation interceptor.
func NewValidationInterceptor() connect.Interceptor {
	validator, err := protovalidate.New()
	if err != nil {
		// Return a no-op interceptor if validation setup fails
		return &ValidationInterceptor{}
	}
	return &ValidationInterceptor{validator: validator}
}

func (i *ValidationInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if i.validator == nil {
			return next(ctx, req)
		}

		// Validate request message
		msg, ok := req.Any().(proto.Message)
		if ok {
			if err := i.validator.Validate(msg); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		return next(ctx, req)
	}
}

func (i *ValidationInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *ValidationInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // Streaming messages would need per-message validation
}

// RateLimitInterceptor provides rate limiting for endpoints.
type RateLimitInterceptor struct {
	rateLimiter *auth.RateLimiter
	logger      *zap.Logger
}

// NewRateLimitInterceptor creates a new rate limit interceptor.
func NewRateLimitInterceptor(rateLimiter *auth.RateLimiter, logger *zap.Logger) connect.Interceptor {
	return &RateLimitInterceptor{
		rateLimiter: rateLimiter,
		logger:      logger,
	}
}

func (i *RateLimitInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Extract client IP for rate limiting
		ip := extractClientIP(req.Header())
		if ip == "" {
			ip = "unknown"
		}

		key := auth.IPRateLimitKey(ip)
		result, err := i.rateLimiter.Check(ctx, key)
		if err != nil {
			i.logger.Error("Rate limit check failed", zap.Error(err))
			// Continue on error - fail open for rate limiting
			return next(ctx, req)
		}

		if !result.Allowed {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				fmt.Errorf("rate limit exceeded, retry after %v", result.RetryAfter))
		}

		// Increment counter after processing
		resp, err := next(ctx, req)
		_, _ = i.rateLimiter.Increment(ctx, key)
		return resp, err
	}
}

func (i *RateLimitInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *RateLimitInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func extractClientIP(header http.Header) string {
	// Try common forwarded headers
	if ip := header.Get("X-Forwarded-For"); ip != "" {
		// Take first IP in chain
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	if ip := header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return ""
}
