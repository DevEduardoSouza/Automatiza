package postgres

import (
	"automatiza-go/internal/repository"
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type rateLimitRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewRateLimitRepository creates a new rate limit repository
func NewRateLimitRepository(db *DB, logger *zap.Logger) repository.RateLimitRepository {
	return &rateLimitRepository{
		db:     db,
		logger: logger,
	}
}

// GetAttempts returns the number of attempts within the time window
func (r *rateLimitRepository) GetAttempts(ctx context.Context, identifier, identifierType string, window time.Duration) (int, error) {
	query := `
		SELECT COALESCE(attempts, 0)
		FROM password_reset_rate_limits
		WHERE identifier = $1
		  AND identifier_type = $2
		  AND window_start > NOW() - $3::interval
	`

	var attempts int
	err := r.db.pool.QueryRow(ctx, query, identifier, identifierType, window.String()).Scan(&attempts)
	if err != nil {
		// No rows means no attempts
		return 0, nil
	}

	return attempts, nil
}

// Increment increases the attempt count for an identifier
func (r *rateLimitRepository) Increment(ctx context.Context, identifier, identifierType string) error {
	query := `
		INSERT INTO password_reset_rate_limits (identifier, identifier_type, attempts, window_start)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (identifier, identifier_type) DO UPDATE
		SET attempts = CASE
			WHEN password_reset_rate_limits.window_start < NOW() - INTERVAL '15 minutes'
			THEN 1
			ELSE password_reset_rate_limits.attempts + 1
		END,
		window_start = CASE
			WHEN password_reset_rate_limits.window_start < NOW() - INTERVAL '15 minutes'
			THEN NOW()
			ELSE password_reset_rate_limits.window_start
		END
	`

	_, err := r.db.pool.Exec(ctx, query, identifier, identifierType)
	if err != nil {
		// SECURITY: Don't log raw identifier (may contain PII like email)
		r.logger.Warn("failed to increment rate limit",
			zap.String("identifier_type", identifierType),
			zap.Error(err))
		return fmt.Errorf("increment rate limit: %w", err)
	}

	return nil
}

// Reset clears the rate limit for an identifier
func (r *rateLimitRepository) Reset(ctx context.Context, identifier, identifierType string) error {
	query := `
		DELETE FROM password_reset_rate_limits
		WHERE identifier = $1 AND identifier_type = $2
	`

	_, err := r.db.pool.Exec(ctx, query, identifier, identifierType)
	if err != nil {
		return fmt.Errorf("reset rate limit: %w", err)
	}

	return nil
}

// Cleanup removes old rate limit entries
func (r *rateLimitRepository) Cleanup(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM password_reset_rate_limits
		WHERE window_start < NOW() - INTERVAL '1 hour'
	`

	result, err := r.db.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleanup rate limits: %w", err)
	}

	return result.RowsAffected(), nil
}
