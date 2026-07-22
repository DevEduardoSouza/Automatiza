package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"automatiza-go/internal/lib/cache"
	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

const (
	stateTTL            = 10 * time.Minute
	containerPollEvery  = 3 * time.Second
	containerPollBudget = 60 * time.Second
)

// Service orchestrates the per-user Instagram connection and story publishing.
// It is safe to construct even when the feature is disabled (api.Enabled()
// false): every entry point degrades to a no-op / not-connected answer.
type Service struct {
	api             *APIClient
	credsRepo       repository.InstagramCredentialsRepository
	storyRepo       repository.InstagramStoryPostRepository
	encryptionKey   string
	stateSigningKey string
	cache           *cache.Cache
	logger          *zap.Logger
}

// NewService builds the Instagram service.
func NewService(
	api *APIClient,
	credsRepo repository.InstagramCredentialsRepository,
	storyRepo repository.InstagramStoryPostRepository,
	encryptionKey, stateSigningKey string,
	c *cache.Cache,
	logger *zap.Logger,
) *Service {
	return &Service{
		api:             api,
		credsRepo:       credsRepo,
		storyRepo:       storyRepo,
		encryptionKey:   encryptionKey,
		stateSigningKey: stateSigningKey,
		cache:           c,
		logger:          logger,
	}
}

// FeatureEnabled reports whether the server has Instagram credentials configured.
func (s *Service) FeatureEnabled() bool {
	return s != nil && s.api != nil && s.api.Enabled() && s.encryptionKey != ""
}

// Status is the connection snapshot for the frontend.
type Status struct {
	FeatureEnabled bool
	Connected      bool
	Username       string
	IGUserID       string
	TokenExpiresAt string
	Recent         []models.InstagramStoryPost
}

// Status returns feature availability and this user's connection state.
func (s *Service) Status(ctx context.Context, userID string) (*Status, error) {
	out := &Status{FeatureEnabled: s.FeatureEnabled()}
	if !out.FeatureEnabled {
		return out, nil
	}
	cred, err := s.credsRepo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if cred != nil {
		out.Connected = true
		out.Username = cred.Username
		out.IGUserID = cred.IGUserID
		if cred.TokenExpiresAt != nil {
			out.TokenExpiresAt = cred.TokenExpiresAt.Format(time.RFC3339)
		}
	}
	if recent, err := s.storyRepo.ListRecentByUser(ctx, userID, 10); err == nil {
		out.Recent = recent
	}
	return out, nil
}

// BuildAuthorizeURL returns the Instagram OAuth URL with a signed, single-use
// state (userID + nonce + expiry). The nonce is stored in Redis so the callback
// can enforce single use even though it carries no session.
func (s *Service) BuildAuthorizeURL(ctx context.Context, userID string) (string, error) {
	if !s.FeatureEnabled() {
		return "", errors.New("instagram: feature not enabled")
	}
	nonce, err := randomToken(16)
	if err != nil {
		return "", err
	}
	if s.cache != nil {
		if err := s.cache.Set(ctx, nonceKey(nonce), userID, stateTTL); err != nil {
			return "", err
		}
	}
	exp := time.Now().Add(stateTTL).Unix()
	state := s.signState(userID, nonce, exp)
	return s.api.AuthorizeURL(state), nil
}

// HandleCallback validates the state, exchanges the code for a long-lived token,
// fetches the profile, encrypts and stores it. Returns a frontend redirect path.
func (s *Service) HandleCallback(ctx context.Context, code, state string) (string, error) {
	if !s.FeatureEnabled() {
		return "/instagram?error=disabled", errors.New("instagram: feature not enabled")
	}
	userID, err := s.verifyState(ctx, state)
	if err != nil {
		s.logger.Warn("instagram callback: invalid state", zap.Error(err))
		return "/instagram?error=invalid_state", err
	}
	if code == "" {
		return "/instagram?error=no_code", errors.New("instagram: empty code")
	}

	shortToken, igUserID, err := s.api.ExchangeCode(ctx, code)
	if err != nil {
		return "/instagram?error=exchange_failed", err
	}
	longToken, expiresIn, err := s.api.ExchangeLongLived(ctx, shortToken)
	if err != nil {
		return "/instagram?error=exchange_failed", err
	}
	// Prefer the id/username from /me (authoritative); fall back to the id the
	// exchange returned.
	meID, username, err := s.api.GetMe(ctx, longToken)
	if err == nil && meID != "" {
		igUserID = meID
	}

	if err := s.saveTokens(ctx, userID, igUserID, username, longToken, expiresIn); err != nil {
		return "/instagram?error=save_failed", err
	}
	return "/instagram?connected=1", nil
}

// Disconnect removes the user's stored credentials.
func (s *Service) Disconnect(ctx context.Context, userID string) error {
	if s.credsRepo == nil {
		return nil
	}
	return s.credsRepo.Delete(ctx, userID)
}

// AccessToken returns the decrypted long-lived token and IG user id for a user.
// Returns ("", "", nil) when the user has not connected (so callers can skip
// fail-open).
func (s *Service) AccessToken(ctx context.Context, userID string) (string, string, error) {
	cred, err := s.credsRepo.Get(ctx, userID)
	if err != nil || cred == nil {
		return "", "", err
	}
	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(cred.EncryptedData), &blob); err != nil {
		return "", "", err
	}
	var tokens models.InstagramTokens
	if err := crypto.DecryptJSONV1(&blob, s.encryptionKey, &tokens); err != nil {
		return "", "", err
	}
	return tokens.AccessToken, cred.IGUserID, nil
}

// HasCredentials reports whether a user has connected an Instagram account
// (cheap check used by the story trigger before enqueuing work).
func (s *Service) HasCredentials(ctx context.Context, userID string) bool {
	if s.credsRepo == nil {
		return false
	}
	cred, err := s.credsRepo.Get(ctx, userID)
	return err == nil && cred != nil
}

// PublishStoryImage runs the container -> poll -> publish flow for a public
// JPEG URL and returns the published media id.
func (s *Service) PublishStoryImage(ctx context.Context, userID, publicImageURL string) (string, error) {
	token, igUserID, err := s.AccessToken(ctx, userID)
	if err != nil {
		return "", err
	}
	if token == "" || igUserID == "" {
		return "", errors.New("instagram: user not connected")
	}

	creationID, err := s.api.CreateStoryContainer(ctx, igUserID, token, publicImageURL)
	if err != nil {
		return "", err
	}

	// Poll until FINISHED. Images finish almost immediately; the budget stays
	// well under the worker's task timeout so a stuck container fails transient
	// and the asynq retry picks it up later.
	deadline := time.Now().Add(containerPollBudget)
	for {
		status, err := s.api.GetContainerStatus(ctx, creationID, token)
		if err != nil {
			return "", err
		}
		switch status {
		case "FINISHED", "PUBLISHED":
			return s.api.PublishContainer(ctx, igUserID, token, creationID)
		case "ERROR", "EXPIRED":
			return "", fmt.Errorf("instagram: container %s status %s", creationID, status)
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("instagram: container %s not ready within budget (last status %s)", creationID, status)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(containerPollEvery):
		}
	}
}

// RefreshExpiring renews long-lived tokens that expire within ~30 days. Meta
// requires the token to be at least 24h old and still valid; both hold here
// (we only connect tokens fresh, and refresh well before the 60d expiry).
func (s *Service) RefreshExpiring(ctx context.Context) error {
	if !s.FeatureEnabled() {
		return nil
	}
	before := time.Now().Add(30 * 24 * time.Hour)
	creds, err := s.credsRepo.ListExpiring(ctx, before)
	if err != nil {
		return err
	}
	for i := range creds {
		cred := &creds[i]
		token, _, err := s.AccessToken(ctx, cred.UserID)
		if err != nil || token == "" {
			s.logger.Warn("instagram refresh: cannot read token", zap.String("userID", cred.UserID), zap.Error(err))
			continue
		}
		newToken, expiresIn, err := s.api.RefreshLongLived(ctx, token)
		if err != nil {
			s.logger.Warn("instagram refresh failed", zap.String("userID", cred.UserID), zap.Error(err))
			continue
		}
		if err := s.saveTokens(ctx, cred.UserID, cred.IGUserID, cred.Username, newToken, expiresIn); err != nil {
			s.logger.Warn("instagram refresh: save failed", zap.String("userID", cred.UserID), zap.Error(err))
		}
	}
	return nil
}

func (s *Service) saveTokens(ctx context.Context, userID, igUserID, username, token string, expiresIn time.Duration) error {
	blob, err := crypto.EncryptJSONV1(models.InstagramTokens{AccessToken: token}, s.encryptionKey)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	if expiresIn <= 0 {
		expiresIn = 60 * 24 * time.Hour
	}
	return s.credsRepo.Save(ctx, userID, string(raw), igUserID, username, time.Now().Add(expiresIn))
}

// --- signed single-use state ---

func (s *Service) signState(userID, nonce string, exp int64) string {
	payload := fmt.Sprintf("%s|%s|%d", userID, nonce, exp)
	enc := base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac := s.hmac(enc)
	return enc + "." + mac
}

func (s *Service) verifyState(ctx context.Context, state string) (string, error) {
	parts := strings.SplitN(state, ".", 2)
	if len(parts) != 2 {
		return "", errors.New("instagram: malformed state")
	}
	if !hmac.Equal([]byte(parts[1]), []byte(s.hmac(parts[0]))) {
		return "", errors.New("instagram: bad state signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	fields := strings.Split(string(raw), "|")
	if len(fields) != 3 {
		return "", errors.New("instagram: malformed state payload")
	}
	userID, nonce, expStr := fields[0], fields[1], fields[2]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", errors.New("instagram: state expired")
	}
	// Single use: the nonce must still exist and belong to this user; consume it.
	if s.cache != nil {
		stored, err := s.cache.Get(ctx, nonceKey(nonce))
		if err != nil || stored != userID {
			return "", errors.New("instagram: state nonce already used or unknown")
		}
		_ = s.cache.Delete(ctx, nonceKey(nonce))
	}
	return userID, nil
}

func (s *Service) hmac(msg string) string {
	h := hmac.New(sha256.New, []byte(s.stateSigningKey))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}

func nonceKey(nonce string) string { return "ig_oauth_nonce:" + nonce }

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
