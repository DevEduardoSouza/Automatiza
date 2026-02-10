// Package waha provides a direct WAHA HTTP client.
//
// DEPRECATED: This package is deprecated and should not be used directly by new code.
//
// Instead, use the adapter pattern:
//   - For WhatsApp operations, use internal/adapters/whatsapp.WhatsAppProvider interface
//   - For services, use internal/services/whatsapp package (InstanceService, GroupsService, MessageService)
//   - The WahaAdapter (internal/adapters/whatsapp/waha_adapter.go) implements WhatsAppProvider
//
// Migration guide:
//   - Replace waha.Service with whatsapp.WhatsAppProvider in your dependencies
//   - Use whatsapp.NewWahaAdapter() to create a WhatsAppProvider implementation
//   - Services in internal/services/whatsapp/ provide higher-level operations
//
// This file remains for reference and for handlers that haven't been migrated yet.
// All new code MUST use the adapter pattern to maintain decoupling.
package waha

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"automatiza-go/internal/config"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// Service defines the interface for Waha Plus operations.
//
// Deprecated: Use whatsapp.WhatsAppProvider from internal/adapters/whatsapp instead.
// This interface exists for backward compatibility with handlers that haven't been migrated.
type Service interface {
	// Instance Management
	CreateInstance(ctx context.Context, instanceName string) (*models.WahaInstance, error)
	DeleteInstance(ctx context.Context, instanceName string) error
	GetQRCode(ctx context.Context, instanceName string) (*models.QRCode, error)
	CheckStatus(ctx context.Context, instanceName string) (string, error)
	ReconnectInstance(ctx context.Context, instanceName string) (*models.QRCode, error)
	LogoutInstance(ctx context.Context, instanceName string) error

	// Messaging
	SendMessage(ctx context.Context, instanceName, groupID, text string, mentions []string) error
	SendMessageWithImage(ctx context.Context, instanceName, groupID, text, imageURL string) error
	SendAudio(ctx context.Context, instanceName, groupID, audioBase64, mimetype string, ptt bool) error
	SendPoll(ctx context.Context, instanceName, groupID, question string, options []string, selectableCount int) error

	// Groups
	GetGroups(ctx context.Context, instanceName string) ([]models.WahaGroup, error)
	GetGroupParticipants(ctx context.Context, instanceName, groupID string) ([]models.Contact, error)
	GetGroupInviteCode(ctx context.Context, instanceName, groupID string) (string, error)
	UpdateGroupSubject(ctx context.Context, instanceName, groupID, subject string) error
	UpdateGroupDescription(ctx context.Context, instanceName, groupID, description string) error
	UpdateGroupPicture(ctx context.Context, instanceName, groupID, imageBase64 string) error
	UpdateGroupSettings(ctx context.Context, instanceName, groupID, action string) error

	// Webhook
	SetWebhook(ctx context.Context, instanceName, webhookURL string, events []string) error

	// Media
	DownloadMedia(ctx context.Context, instanceName string, messagePayload interface{}) ([]byte, error)
}

type wahaService struct {
	configProvider *config.WahaConfigProvider
	httpClient     *http.Client
	logger         *zap.Logger
	webhookBaseURL string
	webhookSecret  string

	// Status cache
	statusCache   map[string]*statusCacheEntry
	statusCacheMu sync.RWMutex

	// Webhook ensure cache
	webhookEnsured   map[string]time.Time
	webhookEnsuredMu sync.RWMutex
}

type statusCacheEntry struct {
	status    string
	expiresAt time.Time
}

// NewWahaService creates a new Waha service
func NewWahaService(configProvider *config.WahaConfigProvider, webhookBaseURL, webhookSecret string, logger *zap.Logger) Service {
	return &wahaService{
		configProvider: configProvider,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:         logger,
		webhookBaseURL: webhookBaseURL,
		webhookSecret:  webhookSecret,
		statusCache:    make(map[string]*statusCacheEntry),
		webhookEnsured: make(map[string]time.Time),
	}
}

func (s *wahaService) getConfig(ctx context.Context) (*config.WahaConfig, error) {
	return s.configProvider.GetConfig(ctx)
}

func (s *wahaService) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get waha config: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	url := cfg.APIURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", cfg.APIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		s.logger.Warn("Waha API error",
			zap.Int("status", resp.StatusCode),
			zap.String("path", path),
			zap.String("response", string(respBody)))
		return nil, fmt.Errorf("waha error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Instance Management

func (s *wahaService) CreateInstance(ctx context.Context, instanceName string) (*models.WahaInstance, error) {
	webhookURL := s.webhookBaseURL + "/api/v1/webhooks/whatsapp"

	payload := map[string]interface{}{
		"name":  instanceName,
		"start": true,
		"config": map[string]interface{}{
			"webhooks": []map[string]interface{}{
				{
					"url":    webhookURL,
					"events": []string{"message", "session.status"},
					"hmac": map[string]interface{}{
						"key": s.webhookSecret,
					},
				},
			},
		},
	}

	resp, err := s.doRequest(ctx, "POST", "/api/sessions", payload)
	if err != nil {
		return nil, err
	}

	var session models.WahaSession
	if err := json.Unmarshal(resp, &session); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := &models.WahaInstance{
		InstanceName: session.Name,
		Status:       session.Status,
	}

	// Wait a moment for session to initialize then fetch QR code
	time.Sleep(500 * time.Millisecond)

	qrCode, err := s.GetQRCode(ctx, session.Name)
	if err != nil {
		// Log but don't fail - QR code might not be ready yet
		s.logger.Warn("failed to get QR code after creating instance",
			zap.String("instance", session.Name),
			zap.Error(err))
	} else if qrCode != nil {
		result.QRCode = qrCode
	}

	return result, nil
}

func (s *wahaService) DeleteInstance(ctx context.Context, instanceName string) error {
	_, err := s.doRequest(ctx, "DELETE", "/api/sessions/"+instanceName, nil)
	return err
}

func (s *wahaService) GetQRCode(ctx context.Context, instanceName string) (*models.QRCode, error) {
	// Get PNG image directly (without format=raw) and convert to base64 data URL
	resp, err := s.doRequest(ctx, "GET", "/api/"+instanceName+"/auth/qr", nil)
	if err != nil {
		return nil, err
	}

	// Convert PNG bytes to base64 data URL for frontend <img> tag
	base64Data := base64.StdEncoding.EncodeToString(resp)
	dataURL := "data:image/png;base64," + base64Data

	return &models.QRCode{
		Base64: dataURL,
	}, nil
}

func (s *wahaService) CheckStatus(ctx context.Context, instanceName string) (string, error) {
	// Check cache first
	s.statusCacheMu.RLock()
	if entry, ok := s.statusCache[instanceName]; ok && time.Now().Before(entry.expiresAt) {
		status := entry.status
		s.statusCacheMu.RUnlock()
		return status, nil
	}
	s.statusCacheMu.RUnlock()

	resp, err := s.doRequest(ctx, "GET", "/api/sessions/"+instanceName, nil)
	if err != nil {
		return "", err
	}

	var session models.WahaSession
	if err := json.Unmarshal(resp, &session); err != nil {
		return "", fmt.Errorf("failed to parse session: %w", err)
	}

	// Cache the status for 5 seconds
	s.statusCacheMu.Lock()
	s.statusCache[instanceName] = &statusCacheEntry{
		status:    session.Status,
		expiresAt: time.Now().Add(5 * time.Second),
	}
	s.statusCacheMu.Unlock()

	return session.Status, nil
}

func (s *wahaService) ReconnectInstance(ctx context.Context, instanceName string) (*models.QRCode, error) {
	// Stop the session first
	_, _ = s.doRequest(ctx, "POST", "/api/sessions/"+instanceName+"/stop", nil)

	// Start it again
	_, err := s.doRequest(ctx, "POST", "/api/sessions/"+instanceName+"/start", nil)
	if err != nil {
		return nil, err
	}

	// Get QR code
	return s.GetQRCode(ctx, instanceName)
}

func (s *wahaService) LogoutInstance(ctx context.Context, instanceName string) error {
	_, err := s.doRequest(ctx, "POST", "/api/sessions/"+instanceName+"/logout", nil)
	return err
}

// Messaging

// checkSessionWorking verifies the session is in WORKING state before operations.
// Returns an error if the session is not ready for sending messages.
func (s *wahaService) checkSessionWorking(ctx context.Context, instanceName string) error {
	status, err := s.CheckStatus(ctx, instanceName)
	if err != nil {
		return fmt.Errorf("failed to check session status: %w", err)
	}

	if status != "WORKING" {
		s.logger.Warn("session not in working state",
			zap.String("instance", instanceName),
			zap.String("status", status))
		return fmt.Errorf("session '%s' is not working: current status is %s", instanceName, status)
	}

	return nil
}

func (s *wahaService) SendMessage(ctx context.Context, instanceName, groupID, text string, mentions []string) error {
	// Check session is working before sending
	if err := s.checkSessionWorking(ctx, instanceName); err != nil {
		return err
	}

	payload := models.WahaSendTextRequest{
		Session:  instanceName,
		ChatID:   groupID,
		Text:     text,
		Mentions: mentions,
	}

	_, err := s.doRequest(ctx, "POST", "/api/sendText", payload)
	return err
}

func (s *wahaService) SendMessageWithImage(ctx context.Context, instanceName, groupID, text, imageURL string) error {
	// Check session is working before sending
	if err := s.checkSessionWorking(ctx, instanceName); err != nil {
		return err
	}

	payload := models.WahaSendImageRequest{
		Session: instanceName,
		ChatID:  groupID,
		Caption: text,
		File:    &models.WahaFile{URL: imageURL},
	}

	_, err := s.doRequest(ctx, "POST", "/api/sendImage", payload)
	return err
}

func (s *wahaService) SendAudio(ctx context.Context, instanceName, groupID, audioBase64, mimetype string, ptt bool) error {
	// Check session is working before sending
	if err := s.checkSessionWorking(ctx, instanceName); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"session": instanceName,
		"chatId":  groupID,
		"file": map[string]interface{}{
			"data":     audioBase64,
			"mimetype": mimetype,
		},
	}

	if ptt {
		// Send as voice note (PTT - push to talk)
		_, err := s.doRequest(ctx, "POST", "/api/sendVoice", payload)
		return err
	}

	_, err := s.doRequest(ctx, "POST", "/api/sendFile", payload)
	return err
}

func (s *wahaService) SendPoll(ctx context.Context, instanceName, groupID, question string, options []string, selectableCount int) error {
	// Check session is working before sending
	if err := s.checkSessionWorking(ctx, instanceName); err != nil {
		return err
	}

	payload := models.WahaSendPollRequest{
		Session: instanceName,
		ChatID:  groupID,
		Poll: models.WahaPollData{
			Name:            question,
			Options:         options,
			MultipleAnswers: selectableCount > 1,
		},
	}

	_, err := s.doRequest(ctx, "POST", "/api/sendPoll", payload)
	return err
}

// Groups

func (s *wahaService) GetGroups(ctx context.Context, instanceName string) ([]models.WahaGroup, error) {
	resp, err := s.doRequest(ctx, "GET", "/api/"+instanceName+"/groups", nil)
	if err != nil {
		return nil, err
	}

	var groups []models.WahaGroup
	if err := json.Unmarshal(resp, &groups); err != nil {
		return nil, fmt.Errorf("failed to parse groups: %w", err)
	}

	return groups, nil
}

func (s *wahaService) GetGroupParticipants(ctx context.Context, instanceName, groupID string) ([]models.Contact, error) {
	resp, err := s.doRequest(ctx, "GET", "/api/"+instanceName+"/groups/"+groupID+"/participants", nil)
	if err != nil {
		return nil, err
	}

	var participants []models.WahaParticipant
	if err := json.Unmarshal(resp, &participants); err != nil {
		return nil, fmt.Errorf("failed to parse participants: %w", err)
	}

	contacts := make([]models.Contact, len(participants))
	for i, p := range participants {
		isAdmin := p.Admin == "admin" || p.Admin == "superadmin"
		contacts[i] = models.Contact{
			ID:      p.ID,
			IsAdmin: &isAdmin,
		}
	}

	return contacts, nil
}

func (s *wahaService) GetGroupInviteCode(ctx context.Context, instanceName, groupID string) (string, error) {
	resp, err := s.doRequest(ctx, "GET", "/api/"+instanceName+"/groups/"+groupID+"/invite-code", nil)
	if err != nil {
		return "", err
	}

	var result struct {
		InviteCode string `json:"inviteCode"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse invite code response: %w", err)
	}

	return result.InviteCode, nil
}

func (s *wahaService) UpdateGroupSubject(ctx context.Context, instanceName, groupID, subject string) error {
	payload := map[string]interface{}{
		"subject": subject,
	}

	_, err := s.doRequest(ctx, "PUT", "/api/"+instanceName+"/groups/"+groupID+"/subject", payload)
	return err
}

func (s *wahaService) UpdateGroupDescription(ctx context.Context, instanceName, groupID, description string) error {
	payload := map[string]interface{}{
		"description": description,
	}

	_, err := s.doRequest(ctx, "PUT", "/api/"+instanceName+"/groups/"+groupID+"/description", payload)
	return err
}

func (s *wahaService) UpdateGroupPicture(ctx context.Context, instanceName, groupID, imageBase64 string) error {
	payload := map[string]interface{}{
		"file": map[string]interface{}{
			"data":     imageBase64,
			"mimetype": "image/jpeg",
		},
	}

	_, err := s.doRequest(ctx, "PUT", "/api/"+instanceName+"/groups/"+groupID+"/picture", payload)
	return err
}

func (s *wahaService) UpdateGroupSettings(ctx context.Context, instanceName, groupID, action string) error {
	// action: "announcement" or "not_announcement"
	payload := map[string]interface{}{
		"adminsOnly": action == "announcement",
	}

	_, err := s.doRequest(ctx, "PUT", "/api/"+instanceName+"/groups/"+groupID+"/settings/security/messages-admin-only", payload)
	return err
}

// Webhook

func (s *wahaService) SetWebhook(ctx context.Context, instanceName, webhookURL string, events []string) error {
	// Check if webhook is already ensured
	s.webhookEnsuredMu.RLock()
	if ensuredAt, ok := s.webhookEnsured[instanceName]; ok && time.Since(ensuredAt) < 10*time.Minute {
		s.webhookEnsuredMu.RUnlock()
		return nil
	}
	s.webhookEnsuredMu.RUnlock()

	payload := map[string]interface{}{
		"url":    webhookURL,
		"events": events,
	}

	if s.webhookSecret != "" {
		payload["hmac"] = map[string]interface{}{
			"key": s.webhookSecret,
		}
	}

	_, err := s.doRequest(ctx, "PUT", "/api/sessions/"+instanceName+"/webhooks", payload)
	if err != nil {
		return err
	}

	// Mark as ensured
	s.webhookEnsuredMu.Lock()
	s.webhookEnsured[instanceName] = time.Now()
	s.webhookEnsuredMu.Unlock()

	return nil
}

// Media

func (s *wahaService) DownloadMedia(ctx context.Context, instanceName string, messagePayload interface{}) ([]byte, error) {
	// Extract media ID from payload
	payloadMap, ok := messagePayload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid message payload type")
	}

	media, ok := payloadMap["media"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no media in payload")
	}

	mediaID, ok := media["id"].(string)
	if !ok {
		return nil, fmt.Errorf("no media ID in payload")
	}

	resp, err := s.doRequest(ctx, "GET", "/api/"+instanceName+"/media/"+mediaID, nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// ClearStatusCache clears the status cache
func (s *wahaService) ClearStatusCache(instanceName string) {
	s.statusCacheMu.Lock()
	delete(s.statusCache, instanceName)
	s.statusCacheMu.Unlock()
}
