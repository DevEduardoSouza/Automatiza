package postgres

import (
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type passwordResetTokenRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewPasswordResetTokenRepository creates a new password reset token repository
func NewPasswordResetTokenRepository(db *DB, logger *zap.Logger) repository.PasswordResetTokenRepository {
	return &passwordResetTokenRepository{
		db:     db,
		logger: logger,
	}
}

// Create stores a new password reset token
func (r *passwordResetTokenRepository) Create(ctx context.Context, token *repository.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.pool.Exec(ctx, query,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.IPAddress,
		token.UserAgent,
	)
	if err != nil {
		r.logger.Error("failed to create password reset token",
			zap.String("userID", token.UserID),
			zap.Error(err))
		return fmt.Errorf("create token: %w", err)
	}

	return nil
}

// GetByHash retrieves a token by its hash
func (r *passwordResetTokenRepository) GetByHash(ctx context.Context, tokenHash []byte) (*repository.PasswordResetToken, error) {
	query := `
		SELECT t.id, t.user_id, t.token_hash, t.expires_at, t.used_at, t.created_at,
		       t.ip_address, t.user_agent, p.email
		FROM password_reset_tokens t
		JOIN profiles p ON p.id = t.user_id
		WHERE t.token_hash = $1
	`

	var token repository.PasswordResetToken
	var ipAddress, userAgent *string

	err := r.db.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
		&ipAddress,
		&userAgent,
		&token.Email,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("get token by hash: %w", err)
	}

	if ipAddress != nil {
		token.IPAddress = *ipAddress
	}
	if userAgent != nil {
		token.UserAgent = *userAgent
	}

	return &token, nil
}

// MarkUsed marks a token as used
func (r *passwordResetTokenRepository) MarkUsed(ctx context.Context, tokenHash []byte) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE token_hash = $1 AND used_at IS NULL
	`

	result, err := r.db.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("mark token used: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("token not found or already used")
	}

	return nil
}

// InvalidateUserTokens marks all tokens for a user as used (expired)
func (r *passwordResetTokenRepository) InvalidateUserTokens(ctx context.Context, userID string) error {
	query := `
		UPDATE password_reset_tokens
		SET used_at = NOW()
		WHERE user_id = $1 AND used_at IS NULL
	`

	_, err := r.db.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("invalidate user tokens: %w", err)
	}

	return nil
}

// CleanupExpired removes expired tokens
func (r *passwordResetTokenRepository) CleanupExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM password_reset_tokens
		WHERE expires_at < NOW() - INTERVAL '1 day'
		   OR used_at < NOW() - INTERVAL '1 day'
	`

	result, err := r.db.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired tokens: %w", err)
	}

	return result.RowsAffected(), nil
}
