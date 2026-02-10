// Package whatsapp provides domain services for WhatsApp operations.
package whatsapp

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/lib/text"
	"automatiza-go/internal/repository"
	"automatiza-go/internal/workers"

	"go.uber.org/zap"
)

// MessageService handles WhatsApp messaging operations.
// It normalizes text, validates inputs, and can either send directly
// or enqueue messages for background processing.
type MessageService interface {
	// SendText sends a text message to a chat (group or individual).
	SendText(ctx context.Context, input SendTextInput) (*whatsapp.MessageResult, error)

	// SendImage sends an image message with optional caption.
	SendImage(ctx context.Context, input SendImageInput) (*whatsapp.MessageResult, error)

	// SendVoice sends a voice message (PTT - Push to Talk).
	SendVoice(ctx context.Context, input SendVoiceInput) (*whatsapp.MessageResult, error)

	// SendPoll sends a poll message to a chat.
	SendPoll(ctx context.Context, input SendPollInput) (*whatsapp.MessageResult, error)

	// EnqueueMessage enqueues a message for background sending via Asynq.
	EnqueueMessage(ctx context.Context, input EnqueueMessageInput) error
}

// SendTextInput contains parameters for sending a text message.
type SendTextInput struct {
	SessionName string
	ChatID      string
	Text        string
	Mentions    []string
	UserID      string // For action history tracking
}

// SendImageInput contains parameters for sending an image message.
type SendImageInput struct {
	SessionName string
	ChatID      string
	Caption     string
	ImageBase64 string
	ImageURL    string
	Mimetype    string
	Filename    string
	UserID      string
}

// SendVoiceInput contains parameters for sending a voice message.
type SendVoiceInput struct {
	SessionName string
	ChatID      string
	AudioBase64 string
	Mimetype    string
	UserID      string
}

// SendPollInput contains parameters for sending a poll message.
type SendPollInput struct {
	SessionName     string
	ChatID          string
	Question        string
	Options         []string
	SelectableCount int
	UserID          string
}

// EnqueueMessageInput contains parameters for enqueueing a message.
type EnqueueMessageInput struct {
	SessionName   string
	ChatID        string
	Message       string
	Mentions      []string
	ImageURL      *string
	ImageBase64   *string
	AudioBase64   *string
	Mimetype      *string
	PTT           bool
	Delay         time.Duration
	UserID        string
	HistoryID     *string
	DispatchLogID *int
}

// messageService implements MessageService.
type messageService struct {
	provider    whatsapp.WhatsAppProvider
	historyRepo repository.ActionHistoryRepository
	queueClient *workers.QueueClient
	logger      *zap.Logger
}

// NewMessageService creates a new message service.
func NewMessageService(
	provider whatsapp.WhatsAppProvider,
	historyRepo repository.ActionHistoryRepository,
	queueClient *workers.QueueClient,
	logger *zap.Logger,
) MessageService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &messageService{
		provider:    provider,
		historyRepo: historyRepo,
		queueClient: queueClient,
		logger:      logger.Named("message-service"),
	}
}

// SendText sends a text message to a chat.
func (s *messageService) SendText(ctx context.Context, input SendTextInput) (*whatsapp.MessageResult, error) {
	// Validate input
	if input.SessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if input.ChatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}

	// Normalize text
	normalizedText := text.NormalizeOutgoingWhatsAppText(input.Text)
	if text.IsEffectivelyEmptyWhatsAppText(normalizedText) {
		return nil, fmt.Errorf("message text cannot be empty")
	}

	// Track action history if user ID provided
	var historyID string
	if input.UserID != "" && s.historyRepo != nil {
		history, err := s.historyRepo.Create(ctx, input.UserID, "send_message", "Sending text message", map[string]interface{}{
			"chatId":      input.ChatID,
			"textLength":  len(normalizedText),
			"hasmentions": len(input.Mentions) > 0,
		})
		if err != nil {
			s.logger.Warn("Failed to create action history",
				zap.String("userID", input.UserID),
				zap.Error(err))
		} else if history != nil {
			historyID = history.ID
		}
	}

	// Send via provider
	providerInput := whatsapp.SendTextInput{
		SessionName: input.SessionName,
		ChatID:      input.ChatID,
		Text:        normalizedText,
		Mentions:    input.Mentions,
	}

	result, err := s.provider.SendText(ctx, providerInput)
	if err != nil {
		// Update history with failure
		if historyID != "" && s.historyRepo != nil {
			_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
				"status":       "failed",
				"error":        err.Error(),
				"completed_at": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return nil, fmt.Errorf("failed to send text message: %w", err)
	}

	// Update history with success
	if historyID != "" && s.historyRepo != nil {
		_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
			"status":       "completed",
			"message_id":   result.ID,
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.logger.Info("Text message sent successfully",
		zap.String("sessionName", input.SessionName),
		zap.String("chatID", input.ChatID),
		zap.String("messageID", result.ID))

	return result, nil
}

// SendImage sends an image message with optional caption.
func (s *messageService) SendImage(ctx context.Context, input SendImageInput) (*whatsapp.MessageResult, error) {
	// Validate input
	if input.SessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if input.ChatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}
	if input.ImageBase64 == "" && input.ImageURL == "" {
		return nil, fmt.Errorf("either image base64 or URL is required")
	}
	if input.ImageBase64 != "" && input.ImageURL != "" {
		return nil, fmt.Errorf("only one of image base64 or URL can be provided")
	}

	// Validate image URL format if provided
	if input.ImageURL != "" {
		if err := validateImageURL(input.ImageURL); err != nil {
			return nil, fmt.Errorf("invalid image URL: %w", err)
		}
	}

	// Normalize caption if provided
	normalizedCaption := ""
	if input.Caption != "" {
		normalizedCaption = text.NormalizeOutgoingWhatsAppText(input.Caption)
	}

	// Track action history
	var historyID string
	if input.UserID != "" && s.historyRepo != nil {
		history, err := s.historyRepo.Create(ctx, input.UserID, "send_image", "Sending image message", map[string]interface{}{
			"chatId":     input.ChatID,
			"hasCaption": normalizedCaption != "",
			"hasURL":     input.ImageURL != "",
		})
		if err != nil {
			s.logger.Warn("Failed to create action history", zap.Error(err))
		} else if history != nil {
			historyID = history.ID
		}
	}

	// Send via provider
	providerInput := whatsapp.SendImageInput{
		SessionName: input.SessionName,
		ChatID:      input.ChatID,
		Caption:     normalizedCaption,
		ImageBase64: input.ImageBase64,
		ImageURL:    input.ImageURL,
		Mimetype:    input.Mimetype,
		Filename:    input.Filename,
	}

	result, err := s.provider.SendImage(ctx, providerInput)
	if err != nil {
		if historyID != "" && s.historyRepo != nil {
			_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
				"status":       "failed",
				"error":        err.Error(),
				"completed_at": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return nil, fmt.Errorf("failed to send image message: %w", err)
	}

	if historyID != "" && s.historyRepo != nil {
		_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
			"status":       "completed",
			"message_id":   result.ID,
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.logger.Info("Image message sent successfully",
		zap.String("sessionName", input.SessionName),
		zap.String("chatID", input.ChatID),
		zap.String("messageID", result.ID))

	return result, nil
}

// SendVoice sends a voice message (PTT - Push to Talk).
func (s *messageService) SendVoice(ctx context.Context, input SendVoiceInput) (*whatsapp.MessageResult, error) {
	// Validate input
	if input.SessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if input.ChatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}
	if input.AudioBase64 == "" {
		return nil, fmt.Errorf("audio base64 data is required")
	}

	// Default mimetype for voice messages
	mimetype := input.Mimetype
	if mimetype == "" {
		mimetype = "audio/ogg; codecs=opus"
	}

	// Track action history
	var historyID string
	if input.UserID != "" && s.historyRepo != nil {
		history, err := s.historyRepo.Create(ctx, input.UserID, "send_voice", "Sending voice message", map[string]interface{}{
			"chatId":   input.ChatID,
			"mimetype": mimetype,
		})
		if err != nil {
			s.logger.Warn("Failed to create action history", zap.Error(err))
		} else if history != nil {
			historyID = history.ID
		}
	}

	// Send via provider
	providerInput := whatsapp.SendVoiceInput{
		SessionName: input.SessionName,
		ChatID:      input.ChatID,
		AudioBase64: input.AudioBase64,
		Mimetype:    mimetype,
	}

	result, err := s.provider.SendVoice(ctx, providerInput)
	if err != nil {
		if historyID != "" && s.historyRepo != nil {
			_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
				"status":       "failed",
				"error":        err.Error(),
				"completed_at": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return nil, fmt.Errorf("failed to send voice message: %w", err)
	}

	if historyID != "" && s.historyRepo != nil {
		_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
			"status":       "completed",
			"message_id":   result.ID,
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.logger.Info("Voice message sent successfully",
		zap.String("sessionName", input.SessionName),
		zap.String("chatID", input.ChatID),
		zap.String("messageID", result.ID))

	return result, nil
}

// SendPoll sends a poll message to a chat.
func (s *messageService) SendPoll(ctx context.Context, input SendPollInput) (*whatsapp.MessageResult, error) {
	// Validate input
	if input.SessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if input.ChatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}
	if input.Question == "" {
		return nil, fmt.Errorf("poll question is required")
	}
	if len(input.Options) < 2 {
		return nil, fmt.Errorf("poll must have at least 2 options")
	}
	if len(input.Options) > 12 {
		return nil, fmt.Errorf("poll cannot have more than 12 options")
	}

	// Validate selectable count
	selectableCount := input.SelectableCount
	if selectableCount < 1 {
		selectableCount = 1
	}
	if selectableCount > len(input.Options) {
		selectableCount = len(input.Options)
	}

	// Track action history
	var historyID string
	if input.UserID != "" && s.historyRepo != nil {
		history, err := s.historyRepo.Create(ctx, input.UserID, "send_poll", "Sending poll message", map[string]interface{}{
			"chatId":      input.ChatID,
			"question":    input.Question,
			"optionCount": len(input.Options),
		})
		if err != nil {
			s.logger.Warn("Failed to create action history", zap.Error(err))
		} else if history != nil {
			historyID = history.ID
		}
	}

	// Send via provider
	providerInput := whatsapp.SendPollInput{
		SessionName:     input.SessionName,
		ChatID:          input.ChatID,
		Question:        input.Question,
		Options:         input.Options,
		SelectableCount: selectableCount,
	}

	result, err := s.provider.SendPoll(ctx, providerInput)
	if err != nil {
		if historyID != "" && s.historyRepo != nil {
			_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
				"status":       "failed",
				"error":        err.Error(),
				"completed_at": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return nil, fmt.Errorf("failed to send poll message: %w", err)
	}

	if historyID != "" && s.historyRepo != nil {
		_ = s.historyRepo.Update(ctx, historyID, map[string]interface{}{
			"status":       "completed",
			"message_id":   result.ID,
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		})
	}

	s.logger.Info("Poll message sent successfully",
		zap.String("sessionName", input.SessionName),
		zap.String("chatID", input.ChatID),
		zap.String("messageID", result.ID))

	return result, nil
}

// EnqueueMessage enqueues a message for background sending via Asynq.
func (s *messageService) EnqueueMessage(ctx context.Context, input EnqueueMessageInput) error {
	if s.queueClient == nil {
		return fmt.Errorf("queue client is not configured")
	}

	// Validate input
	if input.SessionName == "" {
		return fmt.Errorf("session name is required")
	}
	if input.ChatID == "" {
		return fmt.Errorf("chat ID is required")
	}

	// Normalize message text
	normalizedMessage := text.NormalizeOutgoingWhatsAppText(input.Message)
	if text.IsEffectivelyEmptyWhatsAppText(normalizedMessage) && input.ImageURL == nil && input.ImageBase64 == nil && input.AudioBase64 == nil {
		return fmt.Errorf("message content cannot be empty")
	}

	// Build job data
	jobData := &workers.MessageJobData{
		InstanceName:  input.SessionName,
		GroupID:       input.ChatID,
		Message:       normalizedMessage,
		Mentions:      input.Mentions,
		ImageURL:      input.ImageURL,
		ImageBase64:   input.ImageBase64,
		AudioBase64:   input.AudioBase64,
		Mimetype:      input.Mimetype,
		PTT:           input.PTT,
		UserID:        input.UserID,
		HistoryID:     input.HistoryID,
		DispatchLogID: input.DispatchLogID,
	}

	// Enqueue the job
	_, err := s.queueClient.EnqueueMessage(ctx, jobData, input.Delay)
	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	s.logger.Info("Message enqueued successfully",
		zap.String("sessionName", input.SessionName),
		zap.String("chatID", input.ChatID),
		zap.Duration("delay", input.Delay))

	return nil
}

// validateImageURL validates that a URL is well-formed and uses HTTP/HTTPS.
func validateImageURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Parse the URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}

	// Check scheme
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("URL must use HTTP or HTTPS scheme, got: %s", parsed.Scheme)
	}

	// Check host is present
	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
}
