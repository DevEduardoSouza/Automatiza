package connectrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"automatiza-go/internal/repository"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// BillingStatus represents the billing status of a user account.
type BillingStatus string

const (
	// BillingStatusActive indicates the account is in good standing.
	BillingStatusActive BillingStatus = "active"
	// BillingStatusSuspended indicates the account is suspended due to payment issues.
	BillingStatusSuspended BillingStatus = "suspended"
	// BillingStatusCanceled indicates the subscription was canceled.
	BillingStatusCanceled BillingStatus = "canceled"
	// BillingStatusTrial indicates the account is on a trial period.
	BillingStatusTrial BillingStatus = "trial"
)

// ErrAccountSuspended indicates the user account is suspended.
var ErrAccountSuspended = errors.New("account suspended")

// SuspensionChecker provides methods to check user suspension status.
type SuspensionChecker interface {
	// IsUserSuspended checks if a user is suspended.
	// Returns true if suspended, false otherwise.
	IsUserSuspended(ctx context.Context, userID string) (bool, error)

	// GetBillingStatus returns the billing status for a user.
	GetBillingStatus(ctx context.Context, userID string) (BillingStatus, error)
}

// SuspensionInterceptor blocks requests from suspended users.
// This interceptor should be applied after AuthInterceptor to ensure
// the user context is available.
type SuspensionInterceptor struct {
	checker SuspensionChecker
	logger  *zap.Logger
}

// NewSuspensionInterceptor creates a new suspension interceptor.
func NewSuspensionInterceptor(checker SuspensionChecker, logger *zap.Logger) connect.Interceptor {
	return &SuspensionInterceptor{
		checker: checker,
		logger:  logger,
	}
}

// WrapUnary implements the connect.Interceptor interface for unary calls.
func (i *SuspensionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure

		// Skip check for public endpoints (no auth required)
		if publicEndpoints[procedure] {
			return next(ctx, req)
		}

		// Skip check for always-allowed endpoints (even when suspended)
		if i.isAlwaysAllowed(procedure) {
			return next(ctx, req)
		}

		// Get user from context (set by AuthInterceptor)
		user := GetUserFromContext(ctx)
		if user == nil {
			// No user in context - let auth interceptor handle this
			return next(ctx, req)
		}

		// Check if user is suspended
		suspended, err := i.checker.IsUserSuspended(ctx, user.ID)
		if err != nil {
			i.logger.Error("Failed to check suspension status",
				zap.String("userID", user.ID),
				zap.String("procedure", procedure),
				zap.Error(err))
			// Fail open to avoid blocking users due to internal errors
			// but log for monitoring
			return next(ctx, req)
		}

		if suspended {
			traceID := GetTraceIDFromContext(ctx)
			i.logger.Warn("Blocked request from suspended user",
				zap.String("traceID", traceID),
				zap.String("userID", user.ID),
				zap.String("procedure", procedure))

			return nil, connect.NewError(
				connect.CodePermissionDenied,
				fmt.Errorf("Sua conta está suspensa por cobrança (cancelamento/reembolso/atraso). Regularize o pagamento para voltar a usar a plataforma."),
			)
		}

		return next(ctx, req)
	}
}

// WrapStreamingClient implements the connect.Interceptor interface.
func (i *SuspensionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements the connect.Interceptor interface for streaming calls.
func (i *SuspensionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure

		if publicEndpoints[procedure] || i.isAlwaysAllowed(procedure) {
			return next(ctx, conn)
		}

		user := GetUserFromContext(ctx)
		if user == nil {
			return next(ctx, conn)
		}

		suspended, err := i.checker.IsUserSuspended(ctx, user.ID)
		if err != nil {
			i.logger.Error("Failed to check suspension status for streaming",
				zap.String("userID", user.ID),
				zap.String("procedure", procedure),
				zap.Error(err))
			return next(ctx, conn)
		}

		if suspended {
			traceID := GetTraceIDFromContext(ctx)
			i.logger.Warn("Blocked streaming request from suspended user",
				zap.String("traceID", traceID),
				zap.String("userID", user.ID),
				zap.String("procedure", procedure))

			return connect.NewError(
				connect.CodePermissionDenied,
				fmt.Errorf("Sua conta está suspensa por pendência de pagamento."),
			)
		}

		return next(ctx, conn)
	}
}

// isAlwaysAllowed checks if a procedure should be allowed even for suspended users.
// This includes endpoints that allow users to:
// - View their account/profile info
// - Manage their sessions (logout)
// - View billing information
// - Reactivate their account
func (i *SuspensionInterceptor) isAlwaysAllowed(procedure string) bool {
	// Extract service and method from procedure path
	// Format: /service.v1.ServiceName/MethodName
	parts := strings.Split(procedure, "/")
	if len(parts) < 3 {
		return false
	}

	servicePath := parts[1]
	method := parts[2]

	// Always allow auth-related endpoints for session management
	if strings.HasPrefix(servicePath, "auth.") {
		switch method {
		case "GetCurrentUser", "Logout", "ListSessions", "RevokeSession", "RefreshSession":
			return true
		}
	}

	// Always allow viewing own profile
	if strings.HasPrefix(servicePath, "profiles.") {
		switch method {
		case "GetProfile", "UpdateProfile", "ChangePassword":
			return true
		}
	}

	// Always allow viewing billing/plan information
	if strings.HasPrefix(servicePath, "plans.") {
		switch method {
		case "GetCurrentPlan", "ListPlans":
			return true
		}
	}

	// Always allow webhooks (external services calling in)
	if strings.HasPrefix(servicePath, "webhooks.") {
		return true
	}

	return false
}

// DBSuspensionChecker implements SuspensionChecker using the profile repository.
type DBSuspensionChecker struct {
	profileRepo repository.ProfileRepository
	logger      *zap.Logger
}

// NewDBSuspensionChecker creates a new database-backed suspension checker.
func NewDBSuspensionChecker(profileRepo repository.ProfileRepository, logger *zap.Logger) *DBSuspensionChecker {
	return &DBSuspensionChecker{
		profileRepo: profileRepo,
		logger:      logger,
	}
}

// IsUserSuspended checks if a user is suspended by querying the database.
func (c *DBSuspensionChecker) IsUserSuspended(ctx context.Context, userID string) (bool, error) {
	status, err := c.GetBillingStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	return status == BillingStatusSuspended, nil
}

// GetBillingStatus returns the billing status for a user.
// This queries the profiles table for the billing_status field.
func (c *DBSuspensionChecker) GetBillingStatus(ctx context.Context, userID string) (BillingStatus, error) {
	profile, err := c.profileRepo.GetProfile(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to get profile for suspension check",
			zap.String("userID", userID),
			zap.Error(err))
		// Fail open - don't block users due to internal errors
		return BillingStatusActive, nil
	}

	if profile == nil {
		c.logger.Warn("Profile not found for suspension check",
			zap.String("userID", userID))
		// User doesn't exist - let auth interceptor handle this
		return BillingStatusActive, nil
	}

	// Get billing status from profile
	if profile.BillingStatus == nil {
		return BillingStatusActive, nil
	}

	return BillingStatus(*profile.BillingStatus), nil
}
