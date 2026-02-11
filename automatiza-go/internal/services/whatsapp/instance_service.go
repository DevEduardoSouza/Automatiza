// Package whatsapp provides domain services for WhatsApp operations.
// These services use the WhatsAppProvider interface (adapter pattern) to remain
// decoupled from specific WhatsApp API implementations like WAHA.
//
// IMPORTANT: Services in this package MUST NOT import internal/services/waha directly.
// All WhatsApp operations go through the WhatsAppProvider interface.
package whatsapp

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/observability"
	"automatiza-go/internal/repository"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

// InstanceService handles WhatsApp instance lifecycle operations.
// It orchestrates between the WhatsApp provider (adapter) and the database.
type InstanceService interface {
	// CreateInstance creates a new WhatsApp instance/session.
	CreateInstance(ctx context.Context, userID, instanceName string) (*models.Instance, error)

	// DeleteInstance permanently removes a WhatsApp instance.
	DeleteInstance(ctx context.Context, userID string, instanceID int) error

	// GetQRCode retrieves the QR code for instance authentication.
	GetQRCode(ctx context.Context, instanceName string) (*models.QRCode, error)

	// GetStatus retrieves the current status of a WhatsApp instance.
	GetStatus(ctx context.Context, instanceName string) (string, error)

	// ReconnectInstance restarts an existing WhatsApp instance.
	ReconnectInstance(ctx context.Context, instanceName string) (*models.QRCode, error)

	// LogoutInstance logs out the WhatsApp instance, requiring re-authentication.
	LogoutInstance(ctx context.Context, instanceName string) error

	// GetUserInstances retrieves all instances for a user.
	GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error)

	// GetConnectedInstance returns the first connected instance for a user.
	GetConnectedInstance(ctx context.Context, userID string) (*models.Instance, error)
}

// instanceService implements InstanceService.
type instanceService struct {
	provider whatsapp.WhatsAppProvider
	repo     repository.InstanceRepository
	logger   *zap.Logger
}

// NewInstanceService creates a new instance service.
func NewInstanceService(
	provider whatsapp.WhatsAppProvider,
	repo repository.InstanceRepository,
	logger *zap.Logger,
) InstanceService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &instanceService{
		provider: provider,
		repo:     repo,
		logger:   logger.Named("instance-service"),
	}
}

// instanceNamePattern defines valid instance name format.
// Must be alphanumeric with optional underscores/hyphens, 3-50 chars.
var instanceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{2,49}$`)

// validateInstanceName checks if the instance name is valid.
func validateInstanceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}
	if !instanceNamePattern.MatchString(name) {
		return fmt.Errorf("instance name must be 3-50 alphanumeric characters (underscores and hyphens allowed)")
	}
	return nil
}

// CreateInstance creates a new WhatsApp instance/session.
func (s *instanceService) CreateInstance(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	// Validate instance name
	if err := validateInstanceName(instanceName); err != nil {
		return nil, fmt.Errorf("invalid instance name: %w", err)
	}

	// Check if instance already exists for this user
	existing, err := s.repo.GetInstanceByUserAndName(ctx, userID, instanceName)
	if err != nil {
		s.logger.Error("Failed to check existing instance",
			zap.String("userID", userID),
			zap.String("instanceName", instanceName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to check existing instance: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("instance '%s' already exists for this user", instanceName)
	}

	// Create session via WhatsApp provider
	input := whatsapp.CreateSessionInput{
		SessionName: instanceName,
		Engine: whatsapp.EngineConfig{
			StorageMessages: true,
			StorageGroups:   true,
			StorageChats:    true,
			StorageLabels:   false,
		},
	}

	sessionInfo, err := s.provider.CreateSession(ctx, input)
	if err != nil {
		s.logger.Error("Failed to create session via provider",
			zap.String("instanceName", instanceName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create WhatsApp session: %w", err)
	}

	// Map provider status to DB status
	dbStatus := mapSessionStatusToDBStatus(sessionInfo.Status)

	// Upsert instance in database
	instance := &models.Instance{
		InstanceID: instanceName,
		UserID:     userID,
		Status:     dbStatus,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	savedInstance, err := s.repo.UpsertInstance(ctx, userID, instance)
	if err != nil {
		s.logger.Error("Failed to save instance to database",
			zap.String("instanceName", instanceName),
			zap.Error(err))
		// Try to clean up the session we created
		if delErr := s.provider.DeleteSession(ctx, instanceName); delErr != nil {
			s.logger.Warn("Failed to cleanup session after DB error",
				zap.String("instanceName", instanceName),
				zap.Error(delErr))
		}
		return nil, fmt.Errorf("failed to save instance: %w", err)
	}

	s.logger.Info("Instance created successfully",
		zap.String("userID", userID),
		zap.String("instanceName", instanceName),
		zap.Int("instanceID", savedInstance.ID))

	return savedInstance, nil
}

// DeleteInstance permanently removes a WhatsApp instance.
func (s *instanceService) DeleteInstance(ctx context.Context, userID string, instanceID int) error {
	// Get instance to verify ownership and get the instance name
	instance, err := s.repo.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance not found")
	}
	if instance.UserID != userID {
		return fmt.Errorf("instance does not belong to this user")
	}

	// Delete session via WhatsApp provider
	if err := s.provider.DeleteSession(ctx, instance.InstanceID); err != nil {
		// Log but continue - we still want to remove from DB
		s.logger.Warn("Failed to delete session via provider (continuing with DB deletion)",
			zap.String("instanceName", instance.InstanceID),
			zap.Error(err))
	}

	// Delete from database
	if err := s.repo.DeleteInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to delete instance from database: %w", err)
	}

	s.logger.Info("Instance deleted successfully",
		zap.String("userID", userID),
		zap.Int("instanceID", instanceID),
		zap.String("instanceName", instance.InstanceID))

	return nil
}

// GetQRCode retrieves the QR code for instance authentication.
func (s *instanceService) GetQRCode(ctx context.Context, instanceName string) (*models.QRCode, error) {
	qrData, err := s.provider.GetQRCode(ctx, instanceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get QR code: %w", err)
	}
	if qrData == nil {
		// Session is already authenticated
		return nil, nil
	}

	return &models.QRCode{
		Base64: qrData.Base64Image,
		Code:   qrData.Value,
	}, nil
}

// GetStatus retrieves the current status of a WhatsApp instance.
func (s *instanceService) GetStatus(ctx context.Context, instanceName string) (string, error) {
	sessionInfo, err := s.provider.GetSessionStatus(ctx, instanceName)
	if err != nil {
		return "", fmt.Errorf("failed to get session status: %w", err)
	}

	// Map to DB status format
	return mapSessionStatusToDBStatus(sessionInfo.Status), nil
}

// ReconnectInstance restarts an existing WhatsApp instance.
func (s *instanceService) ReconnectInstance(ctx context.Context, instanceName string) (*models.QRCode, error) {
	// Restart the session
	if err := s.provider.RestartSession(ctx, instanceName); err != nil {
		return nil, fmt.Errorf("failed to restart session: %w", err)
	}

	// Wait a bit for the session to initialize
	time.Sleep(500 * time.Millisecond)

	// Get the QR code
	return s.GetQRCode(ctx, instanceName)
}

// LogoutInstance logs out the WhatsApp instance, requiring re-authentication.
func (s *instanceService) LogoutInstance(ctx context.Context, instanceName string) error {
	if err := s.provider.LogoutSession(ctx, instanceName); err != nil {
		return fmt.Errorf("failed to logout session: %w", err)
	}

	// Update status in database
	instance, err := s.repo.GetInstanceByName(ctx, instanceName)
	if err != nil {
		s.logger.Warn("Failed to get instance for status update after logout",
			zap.String("instanceName", instanceName),
			zap.Error(err))
		return nil // Logout succeeded, just couldn't update DB
	}
	if instance != nil {
		if err := s.repo.UpdateInstanceStatus(ctx, instance.ID, models.InstanceStatusDisconnected); err != nil {
			observability.LogAndCapture(s.logger, sentry.LevelWarning, "instance status update failed after logout (fire-and-forget)", err, map[string]string{
				"module": "whatsapp", "operation": "logout_instance_status_update", "instanceName": instanceName,
			})
		}
	}

	return nil
}

// GetUserInstances retrieves all instances for a user.
func (s *instanceService) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	instances, err := s.repo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user instances: %w", err)
	}
	return instances, nil
}

// GetConnectedInstance returns the first connected instance for a user.
func (s *instanceService) GetConnectedInstance(ctx context.Context, userID string) (*models.Instance, error) {
	instances, err := s.repo.GetUserInstances(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user instances: %w", err)
	}

	for i := range instances {
		if instances[i].IsConnected() {
			return &instances[i], nil
		}
	}

	return nil, nil
}

// mapSessionStatusToDBStatus maps the provider's SessionStatus to database status string.
func mapSessionStatusToDBStatus(status whatsapp.SessionStatus) string {
	switch status {
	case whatsapp.SessionStatusConnected:
		return models.InstanceStatusConnected
	case whatsapp.SessionStatusDisconnected:
		return models.InstanceStatusDisconnected
	case whatsapp.SessionStatusScanQR:
		return models.InstanceStatusPending
	case whatsapp.SessionStatusStarting:
		return models.InstanceStatusPending
	case whatsapp.SessionStatusStopped:
		return models.InstanceStatusDisconnected
	case whatsapp.SessionStatusFailed:
		return models.InstanceStatusDisconnected
	default:
		return models.InstanceStatusDisconnected
	}
}
