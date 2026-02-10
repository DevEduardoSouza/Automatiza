package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrSessionNotFound indicates the session does not exist.
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = errors.New("session expired")

	// ErrSessionRevoked indicates the session has been revoked.
	ErrSessionRevoked = errors.New("session revoked")

	// ErrInvalidRefreshToken indicates the refresh token is invalid.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// Session represents an authenticated user session.
type Session struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	RefreshTokenHash  string     `json:"refresh_token_hash"`
	CSRFTokenHash     string     `json:"csrf_token_hash"`
	DeviceFingerprint string     `json:"device_fingerprint,omitempty"`
	IPAddress         string     `json:"ip_address,omitempty"`
	UserAgent         string     `json:"user_agent,omitempty"`
	ExpiresAt         time.Time  `json:"expires_at"`
	CreatedAt         time.Time  `json:"created_at"`
	LastActivityAt    time.Time  `json:"last_activity_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsRevoked returns true if the session has been revoked.
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsValid returns true if the session is active (not expired, not revoked).
func (s *Session) IsValid() bool {
	return !s.IsExpired() && !s.IsRevoked()
}

// SessionStore defines the interface for session storage.
type SessionStore interface {
	// Create creates a new session and returns the session ID and refresh token.
	Create(ctx context.Context, userID string, metadata SessionMetadata) (*Session, string, string, error)

	// Get retrieves a session by ID.
	Get(ctx context.Context, sessionID string) (*Session, error)

	// GetByUserID retrieves all sessions for a user.
	GetByUserID(ctx context.Context, userID string) ([]*Session, error)

	// Refresh refreshes a session using the refresh token, rotating tokens.
	Refresh(ctx context.Context, sessionID, refreshToken string) (*Session, string, string, error)

	// UpdateActivity updates the last activity timestamp.
	UpdateActivity(ctx context.Context, sessionID string) error

	// Revoke revokes a session.
	Revoke(ctx context.Context, sessionID string) error

	// RevokeAllForUser revokes all sessions for a user.
	RevokeAllForUser(ctx context.Context, userID string) (int, error)

	// RevokeAllExcept revokes all sessions for a user except the specified one.
	RevokeAllExcept(ctx context.Context, userID, exceptSessionID string) (int, error)

	// ValidateCSRF validates the CSRF token for a session.
	ValidateCSRF(ctx context.Context, sessionID, csrfToken string) (bool, error)

	// GetCSRFToken returns a new CSRF token for the session.
	GetCSRFToken(ctx context.Context, sessionID string) (string, error)

	// Cleanup removes expired sessions.
	Cleanup(ctx context.Context) error
}

// SessionMetadata contains optional metadata for session creation.
type SessionMetadata struct {
	DeviceFingerprint string
	IPAddress         string
	UserAgent         string
}

// RedisSessionStore implements SessionStore using Redis for caching
// and database for persistence.
type RedisSessionStore struct {
	redis        *redis.Client
	db           SessionDB
	cookieConfig *CookieConfig
	prefix       string
}

// SessionDB defines the database operations for sessions.
type SessionDB interface {
	// CreateSession inserts a new session into the database.
	CreateSession(ctx context.Context, session *Session) error

	// GetSession retrieves a session by ID from the database.
	GetSession(ctx context.Context, sessionID string) (*Session, error)

	// GetSessionsByUserID retrieves all sessions for a user.
	GetSessionsByUserID(ctx context.Context, userID string) ([]*Session, error)

	// UpdateSession updates a session in the database.
	UpdateSession(ctx context.Context, session *Session) error

	// RevokeSession marks a session as revoked.
	RevokeSession(ctx context.Context, sessionID string, revokedAt time.Time) error

	// RevokeSessionsByUserID revokes all sessions for a user.
	RevokeSessionsByUserID(ctx context.Context, userID string, revokedAt time.Time) (int, error)

	// RevokeSessionsExcept revokes all sessions for a user except one.
	RevokeSessionsExcept(ctx context.Context, userID, exceptSessionID string, revokedAt time.Time) (int, error)

	// DeleteExpiredSessions removes expired sessions from the database.
	DeleteExpiredSessions(ctx context.Context) error
}

// NewRedisSessionStore creates a new Redis-backed session store.
func NewRedisSessionStore(redisClient *redis.Client, db SessionDB, cookieConfig *CookieConfig) *RedisSessionStore {
	return &RedisSessionStore{
		redis:        redisClient,
		db:           db,
		cookieConfig: cookieConfig,
		prefix:       "session",
	}
}

// Create creates a new session.
func (s *RedisSessionStore) Create(ctx context.Context, userID string, metadata SessionMetadata) (*Session, string, string, error) {
	// Generate session ID
	sessionID, err := GenerateSessionID()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Generate refresh token
	refreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Generate CSRF token
	csrfToken, err := GenerateCSRFToken()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	now := time.Now()
	session := &Session{
		ID:                sessionID,
		UserID:            userID,
		RefreshTokenHash:  HashToken(refreshToken),
		CSRFTokenHash:     HashToken(csrfToken),
		DeviceFingerprint: metadata.DeviceFingerprint,
		IPAddress:         sanitizeIP(metadata.IPAddress),
		UserAgent:         truncateUserAgent(metadata.UserAgent),
		ExpiresAt:         now.Add(s.cookieConfig.RefreshMaxAge),
		CreatedAt:         now,
		LastActivityAt:    now,
	}

	// Save to database
	if err := s.db.CreateSession(ctx, session); err != nil {
		return nil, "", "", fmt.Errorf("failed to create session in database: %w", err)
	}

	// Cache in Redis
	if err := s.cacheSession(ctx, session); err != nil {
		// Log error but don't fail - database is source of truth
		// The session will be fetched from DB on next request
		_ = err
	}

	return session, refreshToken, csrfToken, nil
}

// Get retrieves a session by ID.
func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	// Try cache first
	session, err := s.getFromCache(ctx, sessionID)
	if err == nil && session != nil {
		if !session.IsValid() {
			return nil, ErrSessionExpired
		}
		return session, nil
	}

	// Fetch from database
	session, err = s.db.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	if session.IsRevoked() {
		return nil, ErrSessionRevoked
	}
	if session.IsExpired() {
		return nil, ErrSessionExpired
	}

	// Update cache
	_ = s.cacheSession(ctx, session)

	return session, nil
}

// GetByUserID retrieves all active sessions for a user.
func (s *RedisSessionStore) GetByUserID(ctx context.Context, userID string) ([]*Session, error) {
	sessions, err := s.db.GetSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Filter to active sessions only
	active := make([]*Session, 0, len(sessions))
	for _, sess := range sessions {
		if sess.IsValid() {
			active = append(active, sess)
		}
	}

	return active, nil
}

// Refresh refreshes a session using the refresh token.
func (s *RedisSessionStore) Refresh(ctx context.Context, sessionID, refreshToken string) (*Session, string, string, error) {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, "", "", err
	}

	// Validate refresh token
	if !ConstantTimeCompareHash(refreshToken, session.RefreshTokenHash) {
		return nil, "", "", ErrInvalidRefreshToken
	}

	// Generate new tokens
	newRefreshToken, err := GenerateRefreshToken()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	newCSRFToken, err := GenerateCSRFToken()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	// Update session
	now := time.Now()
	session.RefreshTokenHash = HashToken(newRefreshToken)
	session.CSRFTokenHash = HashToken(newCSRFToken)
	session.ExpiresAt = now.Add(s.cookieConfig.RefreshMaxAge)
	session.LastActivityAt = now

	// Save to database
	if err := s.db.UpdateSession(ctx, session); err != nil {
		return nil, "", "", fmt.Errorf("failed to update session: %w", err)
	}

	// Update cache
	_ = s.cacheSession(ctx, session)

	return session, newRefreshToken, newCSRFToken, nil
}

// UpdateActivity updates the last activity timestamp.
func (s *RedisSessionStore) UpdateActivity(ctx context.Context, sessionID string) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	session.LastActivityAt = time.Now()

	// Update cache only - batch update to DB periodically
	return s.cacheSession(ctx, session)
}

// Revoke revokes a session.
func (s *RedisSessionStore) Revoke(ctx context.Context, sessionID string) error {
	now := time.Now()

	// Update database
	if err := s.db.RevokeSession(ctx, sessionID, now); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	// Remove from cache
	s.removeFromCache(ctx, sessionID)

	return nil
}

// RevokeAllForUser revokes all sessions for a user.
func (s *RedisSessionStore) RevokeAllForUser(ctx context.Context, userID string) (int, error) {
	now := time.Now()

	// Get all sessions to clear from cache
	sessions, err := s.db.GetSessionsByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Update database
	count, err := s.db.RevokeSessionsByUserID(ctx, userID, now)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke sessions: %w", err)
	}

	// Remove from cache
	for _, sess := range sessions {
		s.removeFromCache(ctx, sess.ID)
	}

	return count, nil
}

// RevokeAllExcept revokes all sessions for a user except the specified one.
func (s *RedisSessionStore) RevokeAllExcept(ctx context.Context, userID, exceptSessionID string) (int, error) {
	now := time.Now()

	// Get all sessions to clear from cache
	sessions, err := s.db.GetSessionsByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get sessions: %w", err)
	}

	// Update database
	count, err := s.db.RevokeSessionsExcept(ctx, userID, exceptSessionID, now)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke sessions: %w", err)
	}

	// Remove from cache (except current)
	for _, sess := range sessions {
		if sess.ID != exceptSessionID {
			s.removeFromCache(ctx, sess.ID)
		}
	}

	return count, nil
}

// ValidateCSRF validates the CSRF token for a session.
func (s *RedisSessionStore) ValidateCSRF(ctx context.Context, sessionID, csrfToken string) (bool, error) {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return false, err
	}

	return ConstantTimeCompareHash(csrfToken, session.CSRFTokenHash), nil
}

// GetCSRFToken generates and returns a new CSRF token for the session.
func (s *RedisSessionStore) GetCSRFToken(ctx context.Context, sessionID string) (string, error) {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		return "", err
	}

	// Generate new CSRF token
	csrfToken, err := GenerateCSRFToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	// Update session
	session.CSRFTokenHash = HashToken(csrfToken)

	// Save to database
	if err := s.db.UpdateSession(ctx, session); err != nil {
		return "", fmt.Errorf("failed to update session: %w", err)
	}

	// Update cache
	_ = s.cacheSession(ctx, session)

	return csrfToken, nil
}

// Cleanup removes expired sessions.
func (s *RedisSessionStore) Cleanup(ctx context.Context) error {
	return s.db.DeleteExpiredSessions(ctx)
}

// cacheSession stores a session in Redis cache.
func (s *RedisSessionStore) cacheSession(ctx context.Context, session *Session) error {
	key := s.cacheKey(session.ID)
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return nil
	}

	return s.redis.SetEx(ctx, key, data, ttl).Err()
}

// getFromCache retrieves a session from Redis cache.
func (s *RedisSessionStore) getFromCache(ctx context.Context, sessionID string) (*Session, error) {
	key := s.cacheKey(sessionID)
	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// removeFromCache removes a session from Redis cache.
func (s *RedisSessionStore) removeFromCache(ctx context.Context, sessionID string) {
	key := s.cacheKey(sessionID)
	_ = s.redis.Del(ctx, key)
}

// cacheKey generates the Redis key for a session.
func (s *RedisSessionStore) cacheKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", s.prefix, sessionID)
}

// sanitizeIP extracts and validates the IP address.
func sanitizeIP(ip string) string {
	if ip == "" {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		// Try to extract from potential host:port format
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			parsed = net.ParseIP(host)
		}
	}
	if parsed == nil {
		return ""
	}
	return parsed.String()
}

// truncateUserAgent limits the user agent string length.
func truncateUserAgent(ua string) string {
	const maxLength = 500
	if len(ua) > maxLength {
		return ua[:maxLength]
	}
	return ua
}
