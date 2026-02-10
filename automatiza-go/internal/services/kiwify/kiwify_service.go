// Package kiwify provides the Kiwify webhook processing service.
// This service handles webhooks from Kiwify payment platform to:
// - Create new users on approved purchases
// - Update user plans
// - Suspend users on refunds/chargebacks
// - Handle subscription cancellations
package kiwify

import (
	"automatiza-go/internal/auth"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/services/email"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service defines the Kiwify webhook service interface.
type Service interface {
	// ProcessWebhook processes an incoming Kiwify webhook and returns the result.
	ProcessWebhook(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error)
}

// Config holds configuration for the Kiwify service.
type Config struct {
	// WebhookToken is the optional token for validating webhooks.
	// If empty, token validation is skipped.
	WebhookToken string
}

type kiwifyService struct {
	profileRepo  repository.ProfileRepository
	planRepo     repository.PlanRepository
	settingsRepo repository.SettingsRepository
	userRoleRepo repository.UserRoleRepository
	emailService email.Service
	config       *Config
	logger       *zap.Logger
}

// NewKiwifyService creates a new Kiwify webhook service.
func NewKiwifyService(
	profileRepo repository.ProfileRepository,
	planRepo repository.PlanRepository,
	settingsRepo repository.SettingsRepository,
	userRoleRepo repository.UserRoleRepository,
	emailService email.Service,
	config *Config,
	logger *zap.Logger,
) Service {
	if config == nil {
		config = &Config{}
	}
	return &kiwifyService{
		profileRepo:  profileRepo,
		planRepo:     planRepo,
		settingsRepo: settingsRepo,
		userRoleRepo: userRoleRepo,
		emailService: emailService,
		config:       config,
		logger:       logger,
	}
}

// ProcessWebhook processes an incoming Kiwify webhook.
func (s *kiwifyService) ProcessWebhook(ctx context.Context, payload *models.KiwifyWebhookPayload, signature string) (*models.KiwifyWebhookResult, error) {
	// Normalize payload (handle wrapped vs direct format)
	data := s.normalizePayload(payload)

	// Validate required fields
	if data.Email == "" {
		return &models.KiwifyWebhookResult{
			Success: false,
			Message: "Customer.email is required",
		}, nil
	}
	if data.ProductName == "" {
		return &models.KiwifyWebhookResult{
			Success: false,
			Message: "Product.product_name is required",
		}, nil
	}
	if data.Event == "" {
		return &models.KiwifyWebhookResult{
			Success: false,
			Message: "webhook_event_type is required",
		}, nil
	}

	s.logger.Info("Processing Kiwify webhook",
		zap.String("email", data.Email),
		zap.String("event", data.Event),
		zap.String("product", data.ProductName),
		zap.String("productID", data.ProductID),
	)

	// Validate token if configured
	webhookToken := s.config.WebhookToken
	if webhookToken == "" {
		// Try to get from settings
		settings, err := s.settingsRepo.Get(ctx, "kiwify_webhook_token")
		if err == nil && settings != nil {
			if token, ok := settings["kiwify_webhook_token"].(string); ok {
				webhookToken = token
			}
		}
	}

	if webhookToken != "" && signature != "" && signature != webhookToken {
		s.logger.Warn("Signature received but not validated",
			zap.String("email", data.Email))
	} else if webhookToken == "" {
		s.logger.Warn("Webhook token not configured - consider setting KIWIFY_WEBHOOK_TOKEN")
	}

	// Determine plan based on product
	plan, err := s.mapProductToPlan(ctx, data.ProductName, data.ProductID)
	if err != nil {
		s.logger.Error("Failed to map product to plan",
			zap.String("product", data.ProductName),
			zap.Error(err))
		plan = "starter" // Default fallback
	}

	s.logger.Info("Plan determined",
		zap.String("plan", plan),
		zap.String("product", data.ProductName),
		zap.String("productID", data.ProductID),
	)

	// Classify event type
	// Supports both English keywords (from Kiwify API) and Portuguese keywords (from simulator)
	eventLower := strings.ToLower(data.Event)

	// Purchase events (includes Portuguese "aprovada", "renovada")
	isPurchaseEvent := eventLower == "order_approved" ||
		strings.Contains(eventLower, "approved") ||
		strings.Contains(eventLower, "active") ||
		strings.Contains(eventLower, "aprovada") ||
		strings.Contains(eventLower, "renovada")

	// Cancel events (includes Portuguese "cancelada")
	isCancelOnlyEvent := eventLower == "subscription_canceled" ||
		strings.Contains(eventLower, "subscription_canceled") ||
		strings.Contains(eventLower, "cancelada")

	// Refund events (includes Portuguese "reembolso")
	isRefundEvent := eventLower == "order_refunded" ||
		strings.Contains(eventLower, "refunded") ||
		strings.Contains(eventLower, "reembolso")

	// Late/overdue events (includes Portuguese "atrasada", "expirada")
	isLateEvent := eventLower == "subscription_late" ||
		strings.Contains(eventLower, "late") ||
		strings.Contains(eventLower, "overdue") ||
		strings.Contains(eventLower, "atrasada") ||
		strings.Contains(eventLower, "expirada")

	// Get existing profile with billing fields
	existingProfile, existingBilling, err := s.profileRepo.GetProfileByEmailWithBilling(ctx, data.Email)
	if err != nil {
		s.logger.Error("Failed to get profile",
			zap.String("email", data.Email),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	// Check for out-of-order events
	if existingProfile != nil && existingBilling != nil &&
		existingBilling.BillingLastEventAt != nil && data.EventAt != nil {
		prevTime := *existingBilling.BillingLastEventAt
		incomingTime := *data.EventAt
		if incomingTime.Before(prevTime) {
			s.logger.Warn("Ignoring out-of-order webhook event",
				zap.String("email", data.Email),
				zap.String("event", data.Event),
				zap.Time("prevEventAt", prevTime),
				zap.Time("incomingEventAt", incomingTime),
			)
			return &models.KiwifyWebhookResult{
				Success: true,
				Message: "Old event ignored (out of order)",
				Event:   data.Event,
			}, nil
		}
	}

	if existingProfile != nil {
		// User exists - process based on event type
		s.logger.Info("Existing user found",
			zap.String("email", data.Email),
			zap.String("userID", existingProfile.ID),
		)

		if isPurchaseEvent {
			return s.handlePurchaseApproved(ctx, existingProfile, existingBilling, data, plan)
		} else if isRefundEvent || data.IsRefundStatus || isLateEvent {
			return s.handleRefundOrLate(ctx, existingProfile, data)
		} else if isCancelOnlyEvent {
			return s.handleSubscriptionCanceled(ctx, existingProfile, data)
		}

		// Unknown event type - acknowledge but don't process
		return &models.KiwifyWebhookResult{
			Success: true,
			Message: "Event received but no action taken",
			UserID:  existingProfile.ID,
			Event:   data.Event,
		}, nil

	} else if isPurchaseEvent {
		// User doesn't exist and it's a purchase event - create new user
		return s.handleNewUserPurchase(ctx, data, plan)
	}

	// User doesn't exist and it's not a purchase event
	s.logger.Warn("Non-purchase event for non-existent user",
		zap.String("email", data.Email),
		zap.String("event", data.Event),
	)

	return &models.KiwifyWebhookResult{
		Success: false,
		Message: "User not found and event is not an approved purchase. No action taken.",
		Event:   data.Event,
	}, nil
}

// normalizePayload extracts and normalizes data from the Kiwify webhook payload.
func (s *kiwifyService) normalizePayload(payload *models.KiwifyWebhookPayload) *models.KiwifyProcessedData {
	var orderData *models.KiwifyOrderData

	// Handle wrapped vs direct format
	if payload.Order != nil {
		orderData = payload.Order
	} else {
		// Direct format - create orderData from payload fields
		orderData = &models.KiwifyOrderData{
			Customer:         payload.Customer,
			Product:          payload.Product,
			Subscription:     payload.Subscription,
			WebhookEventType: payload.WebhookEventType,
			CheckoutLink:     payload.CheckoutLink,
			OrderStatus:      payload.OrderStatus,
			NextPaymentAt:    payload.NextPaymentAt,
			NextPaymentDate:  payload.NextPaymentDate,
			NextPayment:      payload.NextPayment,
			NextBillingAt:    payload.NextBillingAt,
			NextBillingDate:  payload.NextBillingDate,
			NextChargeAt:     payload.NextChargeAt,
			NextChargeDate:   payload.NextChargeDate,
			AccessUntil:      payload.AccessUntil,
			AccessUntilAt:    payload.AccessUntilAt,
			RefundedAt:       payload.RefundedAt,
			ApprovedDate:     payload.ApprovedDate,
			UpdatedAt:        payload.UpdatedAt,
			CreatedAt:        payload.CreatedAt,
		}
	}

	data := &models.KiwifyProcessedData{}

	// Extract customer data
	if orderData.Customer != nil {
		data.Email = orderData.Customer.Email
		data.FullName = orderData.Customer.FullName
		if data.FullName == "" {
			data.FullName = orderData.Customer.FirstName
		}
		data.PhoneNumber = orderData.Customer.Mobile
		data.CPF = orderData.Customer.CPF
	}

	// Extract product data
	if orderData.Product != nil {
		data.ProductName = orderData.Product.ProductName
		data.ProductID = orderData.CheckoutLink
		if data.ProductID == "" {
			data.ProductID = orderData.Product.ProductID
		}
	}

	data.Event = orderData.WebhookEventType
	data.OrderStatus = strings.ToLower(orderData.OrderStatus)

	// Check if order status indicates refund
	data.IsRefundStatus = strings.Contains(data.OrderStatus, "refund") ||
		strings.Contains(data.OrderStatus, "chargeback") ||
		strings.Contains(data.OrderStatus, "dispute")

	// Parse dates
	data.NextPaymentAt = s.extractNextPaymentAt(orderData)
	data.AccessUntil = s.extractAccessUntil(orderData)
	data.EventAt = s.extractEventAt(orderData)

	return data
}

// extractNextPaymentAt extracts the next payment date from various possible fields.
func (s *kiwifyService) extractNextPaymentAt(data *models.KiwifyOrderData) *time.Time {
	candidates := []interface{}{
		data.NextPaymentAt,
		data.NextPaymentDate,
		data.NextPayment,
		data.NextBillingAt,
		data.NextBillingDate,
		data.NextChargeAt,
		data.NextChargeDate,
	}

	// Also check subscription fields
	if data.Subscription != nil {
		candidates = append(candidates,
			data.Subscription.NextPaymentAt,
			data.Subscription.NextPaymentDate,
			data.Subscription.NextPayment,
			data.Subscription.NextBillingAt,
			data.Subscription.NextBillingDate,
			data.Subscription.NextChargeAt,
			data.Subscription.NextChargeDate,
		)

		// Check future charges
		if data.Subscription.Charges != nil && len(data.Subscription.Charges.Future) > 0 {
			candidates = append(candidates, data.Subscription.Charges.Future[0].ChargeDate)
		}
	}

	for _, v := range candidates {
		if t := s.parseFlexibleDate(v); t != nil {
			return t
		}
	}
	return nil
}

// extractAccessUntil extracts the access until date from various possible fields.
func (s *kiwifyService) extractAccessUntil(data *models.KiwifyOrderData) *time.Time {
	candidates := []interface{}{
		data.AccessUntil,
		data.AccessUntilAt,
	}

	if data.Subscription != nil && data.Subscription.CustomerAccess != nil {
		candidates = append(candidates, data.Subscription.CustomerAccess.AccessUntil)
	}

	for _, v := range candidates {
		if t := s.parseFlexibleDate(v); t != nil {
			return t
		}
	}
	return nil
}

// extractEventAt extracts the event timestamp for ordering.
func (s *kiwifyService) extractEventAt(data *models.KiwifyOrderData) *time.Time {
	candidates := []interface{}{
		data.RefundedAt,
		data.ApprovedDate,
		data.UpdatedAt,
		data.CreatedAt,
	}

	if data.Subscription != nil {
		candidates = append(candidates, data.Subscription.StartDate)
	}

	for _, v := range candidates {
		if t := s.parseFlexibleDate(v); t != nil {
			return t
		}
	}
	return nil
}

// parseFlexibleDate parses dates from various formats (string, number, etc.).
func (s *kiwifyService) parseFlexibleDate(value interface{}) *time.Time {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case float64:
		// Unix timestamp (seconds or milliseconds)
		var ms int64
		if v < 1_000_000_000_000 {
			ms = int64(v * 1000)
		} else {
			ms = int64(v)
		}
		t := time.UnixMilli(ms)
		return &t

	case int64:
		var ms int64
		if v < 1_000_000_000_000 {
			ms = v * 1000
		} else {
			ms = v
		}
		t := time.UnixMilli(ms)
		return &t

	case string:
		str := strings.TrimSpace(v)
		if str == "" {
			return nil
		}

		// Check if it's a numeric string (unix timestamp)
		if matched, _ := regexp.MatchString(`^\d+$`, str); matched {
			n, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return nil
			}
			var ms int64
			if n < 1_000_000_000_000 {
				ms = n * 1000
			} else {
				ms = n
			}
			t := time.UnixMilli(ms)
			return &t
		}

		// Try parsing as ISO 8601 / RFC3339
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			if t, err := time.Parse(format, str); err == nil {
				return &t
			}
		}
	}

	return nil
}

// mapProductToPlan maps a Kiwify product to a plan slug.
func (s *kiwifyService) mapProductToPlan(ctx context.Context, productName, productID string) (string, error) {
	// First, try to find by Kiwify product ID
	if productID != "" {
		// Try to extract ID from URL if needed
		searchID := productID
		if strings.Contains(productID, "pay.kiwify.com.br/") {
			re := regexp.MustCompile(`pay\.kiwify\.com\.br/([A-Za-z0-9]+)`)
			matches := re.FindStringSubmatch(productID)
			if len(matches) > 1 {
				searchID = matches[1]
			}
		}

		plan, err := s.planRepo.GetByKiwifyProductID(ctx, searchID)
		if err != nil {
			s.logger.Warn("Error looking up plan by Kiwify product ID",
				zap.String("productID", searchID),
				zap.Error(err))
		}
		if plan != nil {
			return plan.Slug, nil
		}
	}

	// Fallback to name-based mapping
	return s.mapProductNameToPlan(productName), nil
}

// mapProductNameToPlan maps a product name to a plan slug using keyword matching.
func (s *kiwifyService) mapProductNameToPlan(productName string) string {
	nameLower := strings.ToLower(productName)

	// Check for starter/basic
	if strings.Contains(nameLower, "starter") ||
		strings.Contains(nameLower, "basic") ||
		strings.Contains(nameLower, "start") {
		return "starter"
	}

	// Check for pro
	if strings.Contains(nameLower, "pro") {
		return "pro"
	}

	// Check for master/premium/elite
	if strings.Contains(nameLower, "master") ||
		strings.Contains(nameLower, "premium") ||
		strings.Contains(nameLower, "elite") {
		return "master"
	}

	// Default to starter
	s.logger.Warn("Product not recognized, using default plan 'starter'",
		zap.String("product", productName))
	return "starter"
}

// handlePurchaseApproved handles an approved purchase for an existing user.
func (s *kiwifyService) handlePurchaseApproved(
	ctx context.Context,
	profile *models.Profile,
	billing *models.ProfileBillingFields,
	data *models.KiwifyProcessedData,
	plan string,
) (*models.KiwifyWebhookResult, error) {
	// Update plan
	err := s.profileRepo.UpdateProfile(ctx, profile.ID, map[string]interface{}{
		"plan": plan,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	s.logger.Info("Plan updated",
		zap.String("userID", profile.ID),
		zap.String("plan", plan),
	)

	// Check if we should auto-unsuspend
	// Only auto-unsuspend if the suspension was due to payment, not manual
	currentBillingStatus := ""
	if billing != nil && billing.BillingStatus != nil {
		currentBillingStatus = strings.ToLower(*billing.BillingStatus)
	}
	suspensionKind := ""
	if billing != nil && billing.BillingSuspensionKind != nil {
		suspensionKind = strings.ToLower(*billing.BillingSuspensionKind)
	}

	shouldAutoUnsuspend := currentBillingStatus != "suspended" || suspensionKind != "manual"

	if !shouldAutoUnsuspend {
		s.logger.Warn("Account remains suspended (manual block)",
			zap.String("userID", profile.ID),
			zap.String("suspensionKind", suspensionKind),
		)
		return &models.KiwifyWebhookResult{
			Success: true,
			Message: "Payment approved but account remains blocked (manual block)",
			UserID:  profile.ID,
			Plan:    plan,
			Event:   data.Event,
		}, nil
	}

	// Clear suspension and update billing fields
	newBilling := &models.ProfileBillingFields{
		BillingStatus:           nil, // Clear suspension
		BillingSuspendedAt:      nil,
		BillingSuspensionReason: nil,
		BillingSuspensionKind:   nil,
		BillingNextPaymentAt:    data.NextPaymentAt,
		PlanAccessUntil:         data.AccessUntil,
		BillingLastEventAt:      data.EventAt,
		BillingLastEventType:    &data.Event,
	}

	err = s.profileRepo.UpdateBillingStatus(ctx, profile.ID, newBilling)
	if err != nil {
		s.logger.Warn("Failed to update billing status",
			zap.String("userID", profile.ID),
			zap.Error(err))
	}

	return &models.KiwifyWebhookResult{
		Success: true,
		Message: "User plan updated",
		UserID:  profile.ID,
		Plan:    plan,
		Event:   data.Event,
	}, nil
}

// handleRefundOrLate handles refund, chargeback, or late payment events.
func (s *kiwifyService) handleRefundOrLate(
	ctx context.Context,
	profile *models.Profile,
	data *models.KiwifyProcessedData,
) (*models.KiwifyWebhookResult, error) {
	s.logger.Warn("Suspending account due to billing issue",
		zap.String("userID", profile.ID),
		zap.String("email", data.Email),
		zap.String("event", data.Event),
	)

	now := time.Now().UTC()
	suspensionReason := data.Event
	if data.IsRefundStatus {
		suspensionReason = fmt.Sprintf("refund_requested:%s", data.OrderStatus)
	}
	suspensionKind := models.BillingSuspensionKindPayment

	// Update billing status to suspended
	billing := &models.ProfileBillingFields{
		BillingStatus:           strPtr("suspended"),
		BillingSuspendedAt:      &now,
		BillingSuspensionReason: &suspensionReason,
		BillingSuspensionKind:   &suspensionKind,
		BillingNextPaymentAt:    nil, // Clear next payment
		PlanAccessUntil:         data.AccessUntil,
		BillingLastEventAt:      data.EventAt,
		BillingLastEventType:    &suspensionReason,
	}

	err := s.profileRepo.UpdateBillingStatus(ctx, profile.ID, billing)
	if err != nil {
		return nil, fmt.Errorf("failed to suspend account: %w", err)
	}

	// Pause all active automations
	err = s.profileRepo.PauseUserAutomations(ctx, profile.ID)
	if err != nil {
		s.logger.Warn("Failed to pause automations",
			zap.String("userID", profile.ID),
			zap.Error(err))
	}

	return &models.KiwifyWebhookResult{
		Success: true,
		Message: "Account suspended and automations paused",
		UserID:  profile.ID,
		Event:   data.Event,
	}, nil
}

// handleSubscriptionCanceled handles subscription cancellation events.
// This does NOT suspend the account - it only clears the next payment date.
func (s *kiwifyService) handleSubscriptionCanceled(
	ctx context.Context,
	profile *models.Profile,
	data *models.KiwifyProcessedData,
) (*models.KiwifyWebhookResult, error) {
	s.logger.Info("Subscription renewal canceled (no suspension)",
		zap.String("userID", profile.ID),
		zap.String("email", data.Email),
		zap.String("event", data.Event),
	)

	// Only clear the next payment date
	err := s.profileRepo.UpdateProfile(ctx, profile.ID, map[string]interface{}{
		"billing_next_payment_at": nil,
	})
	if err != nil {
		s.logger.Warn("Failed to clear next payment date",
			zap.String("userID", profile.ID),
			zap.Error(err))
	}

	return &models.KiwifyWebhookResult{
		Success: true,
		Message: "Renewal canceled (no suspension)",
		UserID:  profile.ID,
		Event:   data.Event,
	}, nil
}

// handleNewUserPurchase creates a new user from a purchase webhook.
func (s *kiwifyService) handleNewUserPurchase(
	ctx context.Context,
	data *models.KiwifyProcessedData,
	plan string,
) (*models.KiwifyWebhookResult, error) {
	s.logger.Info("Creating new user from webhook",
		zap.String("email", data.Email),
		zap.String("plan", plan),
	)

	// Generate a new UUID for the user
	userID := uuid.New().String()

	// Generate a temporary password
	tempPassword, err := crypto.GenerateTemporaryPassword()
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	// Hash the password
	passwordHash, err := auth.HashPassword(tempPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create the profile
	profile := &models.Profile{
		ID:           userID,
		Email:        data.Email,
		Plan:         plan,
		PhoneNumber:  strPtrIfNotEmpty(data.PhoneNumber),
		PasswordHash: &passwordHash,
	}

	createdProfile, err := s.profileRepo.CreateProfile(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile: %w", err)
	}

	// Create user credentials for authentication
	// This is required for the user to be able to login
	err = s.profileRepo.CreateUserCredentials(ctx, userID, passwordHash)
	if err != nil {
		s.logger.Error("Failed to create user credentials",
			zap.String("userID", userID),
			zap.Error(err))
		// Don't fail - profile was created, user can be fixed manually
	}

	// Assign 'user' role to the new user
	if s.userRoleRepo != nil {
		err = s.userRoleRepo.SetUserRole(ctx, userID, "user")
		if err != nil {
			s.logger.Error("Failed to set user role",
				zap.String("userID", userID),
				zap.Error(err))
			// Don't fail - profile was created, role can be fixed manually
		}
	}

	// Send welcome email with credentials
	if s.emailService != nil {
		name := data.FullName
		if name == "" {
			name = strings.Split(data.Email, "@")[0]
		}

		err = s.emailService.SendWelcomeEmail(ctx, data.Email, name, tempPassword)
		if err != nil {
			s.logger.Error("Failed to send welcome email",
				zap.String("email", data.Email),
				zap.Error(err))
			// Don't fail the webhook - user was created successfully
		}
	}

	return &models.KiwifyWebhookResult{
		Success: true,
		Message: "User created successfully. Credentials sent via email.",
		UserID:  createdProfile.ID,
		Plan:    plan,
		Event:   data.Event,
	}, nil
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func strPtrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
