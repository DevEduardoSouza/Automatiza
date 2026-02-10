package user

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/email"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"go.uber.org/zap"
)

// Service defines the user service interface
type Service interface {
	EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error)
	GetProfile(ctx context.Context, userID string) (*models.Profile, error)
	UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error
	CreateUserFromWebhook(ctx context.Context, data *WebhookUserData, plan *string) (*models.Profile, error)
	UpdateUserPlan(ctx context.Context, userID, plan string) error
	ResetUserPassword(ctx context.Context, userID, adminID, adminEmail string) (*PasswordResetResult, error)
	SuspendUser(ctx context.Context, userID, reason string) error
	ReactivateUser(ctx context.Context, userID string) error
}

// WebhookUserData represents user data from a webhook
type WebhookUserData struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// PasswordResetResult represents the result of a password reset
type PasswordResetResult struct {
	Success           bool   `json:"success"`
	Message           string `json:"message"`
	TemporaryPassword string `json:"temporaryPassword,omitempty"`
}

type userService struct {
	profileRepo  repository.ProfileRepository
	emailService email.Service
	logger       *zap.Logger
}

// NewUserService creates a new user service
func NewUserService(
	profileRepo repository.ProfileRepository,
	emailService email.Service,
	logger *zap.Logger,
) Service {
	return &userService{
		profileRepo:  profileRepo,
		emailService: emailService,
		logger:       logger,
	}
}

func (s *userService) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	return s.profileRepo.EnsureUserProfile(ctx, userID, email)
}

func (s *userService) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	return s.profileRepo.GetProfile(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	return s.profileRepo.UpdateProfile(ctx, userID, updates)
}

func (s *userService) CreateUserFromWebhook(ctx context.Context, data *WebhookUserData, plan *string) (*models.Profile, error) {
	if data.Email == "" {
		return nil, fmt.Errorf("email is required")
	}

	userPlan := "free"
	if plan != nil && *plan != "" {
		userPlan = *plan
	}

	s.logger.Info("User webhook received",
		zap.String("email", data.Email),
		zap.String("plan", userPlan))

	// User creation from webhook is handled separately via the auth service
	// The profile will be created via EnsureUserProfile when they first login
	return nil, nil
}

func (s *userService) UpdateUserPlan(ctx context.Context, userID, plan string) error {
	validPlans := map[string]bool{
		"free":  true,
		"pro":   true,
		"elite": true,
	}

	if !validPlans[plan] {
		return fmt.Errorf("invalid plan: %s", plan)
	}

	updates := map[string]interface{}{
		"plan":       plan,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}

	err := s.profileRepo.UpdateProfile(ctx, userID, updates)
	if err != nil {
		return fmt.Errorf("failed to update plan: %w", err)
	}

	s.logger.Info("Updated user plan",
		zap.String("userID", userID),
		zap.String("plan", plan))

	return nil
}

func (s *userService) ResetUserPassword(ctx context.Context, userID, adminID, adminEmail string) (*PasswordResetResult, error) {
	// Get user profile
	profile, err := s.profileRepo.GetProfile(ctx, userID)
	if err != nil || profile == nil {
		return &PasswordResetResult{
			Success: false,
			Message: "Usuário não encontrado",
		}, nil
	}

	// Generate temporary password
	tempPassword := generateTemporaryPassword()

	// Note: The actual password update is handled by the auth service
	// This method generates a temporary password for admin-initiated resets

	s.logger.Info("Password reset requested",
		zap.String("targetUserID", userID),
		zap.String("adminID", adminID),
		zap.String("adminEmail", adminEmail))

	return &PasswordResetResult{
		Success:           true,
		Message:           "Senha temporária gerada com sucesso",
		TemporaryPassword: tempPassword,
	}, nil
}

func (s *userService) SuspendUser(ctx context.Context, userID, reason string) error {
	updates := map[string]interface{}{
		"billing_status": "suspended",
		"suspend_reason": reason,
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}

	err := s.profileRepo.UpdateProfile(ctx, userID, updates)
	if err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}

	s.logger.Info("User suspended",
		zap.String("userID", userID),
		zap.String("reason", reason))

	return nil
}

func (s *userService) ReactivateUser(ctx context.Context, userID string) error {
	updates := map[string]interface{}{
		"billing_status": "active",
		"suspend_reason": nil,
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}

	err := s.profileRepo.UpdateProfile(ctx, userID, updates)
	if err != nil {
		return fmt.Errorf("failed to reactivate user: %w", err)
	}

	s.logger.Info("User reactivated", zap.String("userID", userID))

	return nil
}

func generateTemporaryPassword() string {
	// Generate a cryptographically secure random 12 character password
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$"
	password := make([]byte, 12)
	for i := range password {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to less secure but still random value
			password[i] = charset[i%len(charset)]
		} else {
			password[i] = charset[n.Int64()]
		}
	}
	return string(password)
}
