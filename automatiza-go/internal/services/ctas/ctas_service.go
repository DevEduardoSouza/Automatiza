package ctas

import (
	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Service defines the CTAs service interface
type Service interface {
	GetUserCTAs(ctx context.Context, userID string, filters *CTAFilters) ([]models.CTA, error)
	GetCTAByID(ctx context.Context, ctaID, userID string) (*models.CTA, error)
	CreateCTA(ctx context.Context, userID string, data *CreateCTAData) (*models.CTA, error)
	UpdateCTA(ctx context.Context, ctaID, userID string, data *UpdateCTAData) (*models.CTA, error)
	DeleteCTA(ctx context.Context, ctaID, userID string) error
}

// CTAFilters represents filters for CTA queries
type CTAFilters struct {
	Platform string `json:"platform,omitempty"`
	CTAType  string `json:"ctaType,omitempty"`
	IsActive *bool  `json:"isActive,omitempty"`
}

// CreateCTAData represents data for creating a CTA
type CreateCTAData struct {
	Message  string `json:"message" validate:"required"`
	Platform string `json:"platform,omitempty"`
	CTAType  string `json:"ctaType,omitempty"`
}

// UpdateCTAData represents data for updating a CTA
type UpdateCTAData struct {
	Message  *string `json:"message,omitempty"`
	Platform *string `json:"platform,omitempty"`
	CTAType  *string `json:"ctaType,omitempty"`
	IsActive *bool   `json:"isActive,omitempty"`
}

type ctasService struct {
	ctaRepo repository.CTARepository
	cache   *cache.Cache
	logger  *zap.Logger
}

// NewCTAsService creates a new CTAs service
func NewCTAsService(
	ctaRepo repository.CTARepository,
	cache *cache.Cache,
	logger *zap.Logger,
) Service {
	return &ctasService{
		ctaRepo: ctaRepo,
		cache:   cache,
		logger:  logger,
	}
}

func (s *ctasService) GetUserCTAs(ctx context.Context, userID string, filters *CTAFilters) ([]models.CTA, error) {
	filterMap := make(map[string]interface{})

	if filters != nil {
		if filters.Platform != "" {
			filterMap["platform"] = filters.Platform
		}
		if filters.CTAType != "" {
			filterMap["cta_type"] = filters.CTAType
		}
		if filters.IsActive != nil {
			filterMap["is_active"] = *filters.IsActive
		}
	}

	ctaList, err := s.ctaRepo.GetUserCTAs(ctx, userID, filterMap)
	if err != nil {
		return nil, fmt.Errorf("failed to get CTAs: %w", err)
	}

	return ctaList, nil
}

func (s *ctasService) GetCTAByID(ctx context.Context, ctaID, userID string) (*models.CTA, error) {
	cta, err := s.ctaRepo.GetCTAByID(ctx, ctaID, userID)
	if err != nil {
		return nil, err
	}
	return cta, nil
}

func (s *ctasService) CreateCTA(ctx context.Context, userID string, data *CreateCTAData) (*models.CTA, error) {
	message := strings.TrimSpace(data.Message)
	if message == "" {
		return nil, fmt.Errorf("CTA message is required")
	}

	platform := data.Platform
	if platform == "" {
		platform = models.CTAPlatformAll
	}

	ctaType := data.CTAType
	if ctaType == "" {
		ctaType = models.CTATypeAll
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cta := &models.CTA{
		UserID:    userID,
		Message:   message,
		Platform:  platform,
		CTAType:   ctaType,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	created, err := s.ctaRepo.CreateCTA(ctx, userID, cta)
	if err != nil {
		return nil, fmt.Errorf("failed to create CTA: %w", err)
	}

	s.logger.Info("Created CTA",
		zap.String("userID", userID),
		zap.String("platform", platform),
		zap.String("ctaType", ctaType))

	return created, nil
}

func (s *ctasService) UpdateCTA(ctx context.Context, ctaID, userID string, data *UpdateCTAData) (*models.CTA, error) {
	existing, err := s.ctaRepo.GetCTAByID(ctx, ctaID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get CTA: %w", err)
	}
	if existing == nil {
		return nil, &CTANotFoundError{ID: ctaID}
	}

	updates := make(map[string]interface{})

	if data.Message != nil {
		message := strings.TrimSpace(*data.Message)
		if message == "" {
			return nil, fmt.Errorf("CTA message cannot be empty")
		}
		updates["message"] = message
	}
	if data.Platform != nil {
		updates["platform"] = *data.Platform
	}
	if data.CTAType != nil {
		updates["cta_type"] = *data.CTAType
	}
	if data.IsActive != nil {
		updates["is_active"] = *data.IsActive
	}

	err = s.ctaRepo.UpdateCTA(ctx, ctaID, userID, updates)
	if err != nil {
		return nil, fmt.Errorf("failed to update CTA: %w", err)
	}

	return s.ctaRepo.GetCTAByID(ctx, ctaID, userID)
}

func (s *ctasService) DeleteCTA(ctx context.Context, ctaID, userID string) error {
	existing, err := s.ctaRepo.GetCTAByID(ctx, ctaID, userID)
	if err != nil {
		return fmt.Errorf("failed to get CTA: %w", err)
	}
	if existing == nil {
		return &CTANotFoundError{ID: ctaID}
	}

	err = s.ctaRepo.DeleteCTA(ctx, ctaID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete CTA: %w", err)
	}

	s.logger.Info("Deleted CTA",
		zap.String("ctaID", ctaID),
		zap.String("userID", userID))

	return nil
}
