// Package workers provides background job processing.
// This file implements a MessageProcessor for reconciliation that creates dispatch_logs
// for reconciled messages, ensuring no gaps in the dispatch history.
package workers

import (
	"context"
	"fmt"
	"strings"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/config"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// =============================================================================
// Constants
// =============================================================================

const (
	// DefaultReconciliationDedupWindowHours is the time window (in hours) for deduplication
	// during reconciliation. Prevents creating duplicate logs for the same message.
	DefaultReconciliationDedupWindowHours = 168 // 7 days
)

// =============================================================================
// DispatchLogProcessor
// =============================================================================

// DispatchLogProcessor implements reconciliation.MessageProcessor and creates
// dispatch_logs for reconciled messages. It ensures idempotency using canonical_key
// deduplication to avoid creating duplicate logs for the same message.
//
// This processor is used by the ReconciliationService when backfilling missed messages.
// It creates logs with status "reconciled" to distinguish them from normal dispatches.
type DispatchLogProcessor struct {
	dispatchLogRepo repository.DispatchLogRepository
	instanceRepo    repository.InstanceRepository
	logger          *zap.Logger

	// Configuration
	dedupWindowHours int
}

// DispatchLogProcessorConfig holds configuration for creating a DispatchLogProcessor.
type DispatchLogProcessorConfig struct {
	DispatchLogRepo  repository.DispatchLogRepository
	InstanceRepo     repository.InstanceRepository
	Logger           *zap.Logger
	DedupWindowHours int // Optional, defaults to DefaultReconciliationDedupWindowHours
}

// NewDispatchLogProcessor creates a new DispatchLogProcessor.
//
// Parameters:
//   - dispatchLogRepo: Repository for creating dispatch logs
//   - instanceRepo: Repository for resolving session to user mapping
//   - logger: Structured logger
//   - dedupWindowHours: Time window for deduplication (0 uses default)
func NewDispatchLogProcessor(config DispatchLogProcessorConfig) *DispatchLogProcessor {
	dedupHours := config.DedupWindowHours
	if dedupHours <= 0 {
		dedupHours = DefaultReconciliationDedupWindowHours
	}

	return &DispatchLogProcessor{
		dispatchLogRepo:  config.DispatchLogRepo,
		instanceRepo:     config.InstanceRepo,
		logger:           config.Logger.Named("dispatch-log-processor"),
		dedupWindowHours: dedupHours,
	}
}

// ProcessMessage implements reconciliation.MessageProcessor.
// It creates a dispatch_log entry for the reconciled message if one doesn't already exist.
//
// This method is idempotent - calling it multiple times with the same message
// will not create duplicate logs due to canonical_key deduplication.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - sessionName: The WAHA session identifier (e.g., "user_123_instance_456")
//   - msg: The WhatsApp message from reconciliation
//
// Returns:
//   - nil if successfully processed or already exists (idempotent)
//   - error if processing failed
func (p *DispatchLogProcessor) ProcessMessage(ctx context.Context, sessionName string, msg *whatsapp.Message) error {
	// Check feature flag: skip if Go should not perform reconciliation
	featureFlags := config.GetFeatureFlags()
	if !featureFlags.ShouldReconcile() {
		p.logger.Debug("Skipping reconciliation dispatch log creation (feature flag disabled)",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID))
		return nil
	}

	// Skip messages from me (outbound) - we only care about incoming messages for monitoring
	if msg.FromMe {
		p.logger.Debug("Skipping outbound message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID))
		return nil
	}

	// Skip messages without body content
	if msg.Body == "" && !msg.HasMedia {
		p.logger.Debug("Skipping empty message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID))
		return nil
	}

	// Resolve session to user ID via instance repository
	userID, err := p.resolveUserID(ctx, sessionName)
	if err != nil {
		p.logger.Warn("Failed to resolve user ID from session",
			zap.String("session", sessionName),
			zap.Error(err))
		// Don't fail - this might be a session we don't have mapped
		return nil
	}

	// Extract chat/group ID from the message
	groupID := msg.ChatID
	if groupID == "" {
		p.logger.Debug("Skipping message without chat ID",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID))
		return nil
	}

	// Generate canonical key for deduplication
	// For reconciliation, we use the message ID as the canonical key
	// since we're tracking which messages have been processed
	canonicalKey := p.generateCanonicalKey(sessionName, msg)

	// Create the dispatch log entry
	logEntry := &models.DispatchLog{
		UserID:       userID,
		GroupID:      &groupID,
		CanonicalKey: &canonicalKey,
		Status:       models.DispatchLogStatusReconciled,
		Type:         ptr("reconciliation"),
		ProductURL:   p.extractProductURL(msg.Body),
		CreatedAt:    msg.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Try to create the log (idempotent - won't create duplicates)
	created, wasCreated, err := p.dispatchLogRepo.CreateIfNotExists(ctx, logEntry, p.dedupWindowHours)
	if err != nil {
		p.logger.Error("Failed to create dispatch log for reconciled message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID),
			zap.String("canonicalKey", canonicalKey),
			zap.Error(err))
		return fmt.Errorf("failed to create dispatch log: %w", err)
	}

	if wasCreated {
		p.logger.Info("Created dispatch log for reconciled message",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID),
			zap.Int("logID", created.ID),
			zap.String("groupID", groupID),
			zap.String("canonicalKey", canonicalKey))
	} else {
		p.logger.Debug("Dispatch log already exists (dedup)",
			zap.String("session", sessionName),
			zap.String("messageID", msg.ID),
			zap.String("canonicalKey", canonicalKey))
	}

	return nil
}

// resolveUserID maps a session name to a user ID via the instance repository.
// Session names typically follow the format: "user_{userID}_instance_{instanceID}"
func (p *DispatchLogProcessor) resolveUserID(ctx context.Context, sessionName string) (string, error) {
	// Try to get instance by session name
	instance, err := p.instanceRepo.GetInstanceByName(ctx, sessionName)
	if err != nil {
		return "", fmt.Errorf("failed to get instance by name: %w", err)
	}
	if instance == nil {
		return "", fmt.Errorf("instance not found for session: %s", sessionName)
	}

	return instance.UserID, nil
}

// generateCanonicalKey creates a unique key for deduplication.
// For reconciliation, we combine session + message ID to ensure uniqueness
// across different sessions and prevent processing the same message twice.
func (p *DispatchLogProcessor) generateCanonicalKey(sessionName string, msg *whatsapp.Message) string {
	// Use message ID as primary key since it's unique per message
	// Prefix with "reconcile:" to distinguish from normal dispatch keys
	return fmt.Sprintf("reconcile:%s:%s", sessionName, msg.ID)
}

// extractProductURL attempts to extract a product URL from the message body.
// Returns nil if no recognizable URL pattern is found.
func (p *DispatchLogProcessor) extractProductURL(body string) *string {
	if body == "" {
		return nil
	}

	// Look for common e-commerce URL patterns
	// Mercado Livre, Amazon, Shopee, etc.
	patterns := []string{
		"mercadolivre.com",
		"mercadolibre.com",
		"produto.mercadolivre.com.br",
		"amazon.com",
		"amzn.to",
		"shopee.com",
		"magazineluiza.com.br",
		"magalu.com",
		"americanas.com.br",
		"casasbahia.com.br",
		"kabum.com.br",
	}

	bodyLower := strings.ToLower(body)
	for _, pattern := range patterns {
		if strings.Contains(bodyLower, pattern) {
			// Try to extract the full URL
			url := p.extractURL(body, pattern)
			if url != "" {
				return &url
			}
		}
	}

	return nil
}

// extractURL extracts a URL containing the given pattern from text.
func (p *DispatchLogProcessor) extractURL(text, pattern string) string {
	// Simple URL extraction - find http(s):// followed by the pattern
	lowerText := strings.ToLower(text)
	idx := strings.Index(lowerText, pattern)
	if idx == -1 {
		return ""
	}

	// Find the start of the URL (look backwards for http:// or https://)
	start := idx
	for i := idx - 1; i >= 0; i-- {
		if text[i] == ' ' || text[i] == '\n' || text[i] == '\t' {
			start = i + 1
			break
		}
		if i == 0 {
			start = 0
		}
	}

	// Find the end of the URL (whitespace or end of string)
	end := len(text)
	for i := idx; i < len(text); i++ {
		if text[i] == ' ' || text[i] == '\n' || text[i] == '\t' {
			end = i
			break
		}
	}

	url := text[start:end]

	// Ensure it starts with http
	if !strings.HasPrefix(strings.ToLower(url), "http") {
		url = "https://" + url
	}

	return url
}

// =============================================================================
// Statistics
// =============================================================================

// ReconciliationStats holds statistics about reconciliation processing.
type ReconciliationStats struct {
	TotalProcessed int
	LogsCreated    int
	LogsSkipped    int
	Errors         int
}

// ptr is a helper function to create a pointer to a string.
func ptr(s string) *string {
	return &s
}
