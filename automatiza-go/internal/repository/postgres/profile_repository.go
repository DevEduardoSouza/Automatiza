package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type profileRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewProfileRepository creates a new PostgreSQL profile repository
func NewProfileRepository(db *DB, logger *zap.Logger) repository.ProfileRepository {
	return &profileRepository{
		db:     db,
		logger: logger,
	}
}

func (r *profileRepository) EnsureUserProfile(ctx context.Context, userID, email string) (*models.Profile, error) {
	// First, try to get existing profile
	profile, err := r.GetProfile(ctx, userID)
	if err == nil && profile != nil {
		return profile, nil
	}

	// Create new profile using upsert
	now := time.Now().UTC()
	query := `
		INSERT INTO profiles (id, email, plan, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			updated_at = EXCLUDED.updated_at
		RETURNING id, email, phone_number, plan, max_automations, billing_status,
		          billing_next_payment_at, plan_access_until, created_at, updated_at
	`

	var p models.Profile
	var phoneNumber, billingStatus *string
	var maxAutomations *int
	var billingNextPaymentAt, planAccessUntil *time.Time

	err = r.db.QueryRow(ctx, query, userID, email, models.PlanFree, now, now).Scan(
		&p.ID, &p.Email, &phoneNumber, &p.Plan, &maxAutomations, &billingStatus,
		&billingNextPaymentAt, &planAccessUntil, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to upsert profile",
			zap.String("userID", userID),
			zap.Error(err))
		return nil, err
	}

	p.PhoneNumber = phoneNumber
	p.BillingStatus = billingStatus
	p.MaxAutomations = maxAutomations
	p.BillingNextPaymentAt = billingNextPaymentAt
	p.PlanAccessUntil = planAccessUntil

	return &p, nil
}

func (r *profileRepository) GetProfile(ctx context.Context, userID string) (*models.Profile, error) {
	query := `
		SELECT id, email, phone_number, plan, max_automations, billing_status, password_hash,
		       billing_next_payment_at, plan_access_until, created_at, updated_at
		FROM profiles
		WHERE id = $1
	`

	var p models.Profile
	var phoneNumber, billingStatus, passwordHash *string
	var maxAutomations *int
	var billingNextPaymentAt, planAccessUntil *time.Time
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.ID, &p.Email, &phoneNumber, &p.Plan, &maxAutomations, &billingStatus, &passwordHash,
		&billingNextPaymentAt, &planAccessUntil, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	p.PhoneNumber = phoneNumber
	p.BillingStatus = billingStatus
	p.PasswordHash = passwordHash
	p.MaxAutomations = maxAutomations
	p.BillingNextPaymentAt = billingNextPaymentAt
	p.PlanAccessUntil = planAccessUntil
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &p, nil
}

func (r *profileRepository) UpdateProfile(ctx context.Context, userID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Build dynamic UPDATE query
	setClauses := ""
	args := make([]interface{}, 0, len(updates)+1)
	argIndex := 1

	for key, value := range updates {
		if setClauses != "" {
			setClauses += ", "
		}
		setClauses += fmt.Sprintf("%s = $%d", key, argIndex)
		args = append(args, value)
		argIndex++
	}

	// Always update updated_at
	setClauses += fmt.Sprintf(", updated_at = $%d", argIndex)
	args = append(args, time.Now().UTC())
	argIndex++

	// Add userID as the last argument
	args = append(args, userID)

	query := fmt.Sprintf("UPDATE profiles SET %s WHERE id = $%d", setClauses, argIndex)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update profile",
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	return nil
}

// GetProfileByEmail retrieves a profile by email address
func (r *profileRepository) GetProfileByEmail(ctx context.Context, email string) (*models.Profile, error) {
	query := `
		SELECT id, email, phone_number, plan, max_automations, billing_status, password_hash,
		       billing_next_payment_at, plan_access_until, created_at, updated_at
		FROM profiles
		WHERE email = $1
	`

	var p models.Profile
	var phoneNumber, billingStatus, passwordHash *string
	var maxAutomations *int
	var billingNextPaymentAt, planAccessUntil *time.Time
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, email).Scan(
		&p.ID, &p.Email, &phoneNumber, &p.Plan, &maxAutomations, &billingStatus, &passwordHash,
		&billingNextPaymentAt, &planAccessUntil, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	p.PhoneNumber = phoneNumber
	p.BillingStatus = billingStatus
	p.PasswordHash = passwordHash
	p.MaxAutomations = maxAutomations
	p.BillingNextPaymentAt = billingNextPaymentAt
	p.PlanAccessUntil = planAccessUntil
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &p, nil
}

// UpdatePassword updates the password hash for a user in user_credentials table
func (r *profileRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	now := time.Now().UTC()

	// Update user_credentials table (used by AuthService for login)
	query := `
		UPDATE user_credentials
		SET password_hash = $1, password_changed_at = $2, failed_attempts = 0, locked_until = NULL
		WHERE user_id = $3
	`

	result, err := r.db.pool.Exec(ctx, query, passwordHash, now, userID)
	if err != nil {
		r.logger.Error("Failed to update password in user_credentials",
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("update password: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user credentials not found")
	}

	// Also update profiles table for consistency
	profileQuery := `
		UPDATE profiles
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`
	_, _ = r.db.pool.Exec(ctx, profileQuery, passwordHash, now, userID)

	return nil
}

// GetProfileByEmailWithBilling retrieves a profile by email including billing fields.
// Returns the profile and billing fields separately for easier handling.
func (r *profileRepository) GetProfileByEmailWithBilling(ctx context.Context, email string) (*models.Profile, *models.ProfileBillingFields, error) {
	query := `
		SELECT id, email, phone_number, plan, max_automations, billing_status, password_hash,
		       billing_suspended_at, billing_suspension_reason, billing_suspension_kind,
		       billing_next_payment_at, billing_last_event_at, billing_last_event_type,
		       plan_access_until, created_at, updated_at
		FROM profiles
		WHERE email = $1
	`

	var p models.Profile
	var billing models.ProfileBillingFields
	var phoneNumber, billingStatus, passwordHash *string
	var maxAutomations *int
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, email).Scan(
		&p.ID, &p.Email, &phoneNumber, &p.Plan, &maxAutomations, &billingStatus, &passwordHash,
		&billing.BillingSuspendedAt, &billing.BillingSuspensionReason, &billing.BillingSuspensionKind,
		&billing.BillingNextPaymentAt, &billing.BillingLastEventAt, &billing.BillingLastEventType,
		&billing.PlanAccessUntil, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	p.PhoneNumber = phoneNumber
	p.BillingStatus = billingStatus
	p.PasswordHash = passwordHash
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	billing.BillingStatus = billingStatus

	return &p, &billing, nil
}

// CreateProfile creates a new profile with the given data.
// The profile ID should be a valid UUID.
func (r *profileRepository) CreateProfile(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	now := time.Now().UTC()
	query := `
		INSERT INTO profiles (id, email, phone_number, plan, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, phone_number, plan, max_automations, billing_status, password_hash,
		          billing_next_payment_at, plan_access_until, created_at, updated_at
	`

	var p models.Profile
	var phoneNumber, billingStatus, passwordHash *string
	var maxAutomations *int
	var billingNextPaymentAt, planAccessUntil *time.Time
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query,
		profile.ID, profile.Email, profile.PhoneNumber, profile.Plan, profile.PasswordHash, now, now,
	).Scan(
		&p.ID, &p.Email, &phoneNumber, &p.Plan, &maxAutomations, &billingStatus, &passwordHash,
		&billingNextPaymentAt, &planAccessUntil, &createdAt, &updatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to create profile",
			zap.String("email", profile.Email),
			zap.Error(err))
		return nil, fmt.Errorf("create profile: %w", err)
	}

	p.PhoneNumber = phoneNumber
	p.BillingStatus = billingStatus
	p.PasswordHash = passwordHash
	p.MaxAutomations = maxAutomations
	p.BillingNextPaymentAt = billingNextPaymentAt
	p.PlanAccessUntil = planAccessUntil
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &p, nil
}

// CreateUserCredentials creates the user_credentials entry for authentication.
// This must be called after CreateProfile to enable user login.
func (r *profileRepository) CreateUserCredentials(ctx context.Context, userID, passwordHash string) error {
	now := time.Now().UTC()
	query := `
		INSERT INTO user_credentials (user_id, password_hash, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			password_changed_at = EXCLUDED.created_at
	`

	_, err := r.db.pool.Exec(ctx, query, userID, passwordHash, now)
	if err != nil {
		r.logger.Error("Failed to create user credentials",
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("create user credentials: %w", err)
	}

	return nil
}

// UpdateBillingStatus updates the billing-related fields for a profile.
func (r *profileRepository) UpdateBillingStatus(ctx context.Context, userID string, billing *models.ProfileBillingFields) error {
	now := time.Now().UTC()
	query := `
		UPDATE profiles
		SET billing_status = $1,
		    billing_suspended_at = $2,
		    billing_suspension_reason = $3,
		    billing_suspension_kind = $4,
		    billing_next_payment_at = $5,
		    billing_last_event_at = $6,
		    billing_last_event_type = $7,
		    plan_access_until = $8,
		    updated_at = $9
		WHERE id = $10
	`

	result, err := r.db.pool.Exec(ctx, query,
		billing.BillingStatus,
		billing.BillingSuspendedAt,
		billing.BillingSuspensionReason,
		billing.BillingSuspensionKind,
		billing.BillingNextPaymentAt,
		billing.BillingLastEventAt,
		billing.BillingLastEventType,
		billing.PlanAccessUntil,
		now,
		userID,
	)
	if err != nil {
		r.logger.Error("Failed to update billing status",
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("update billing status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("profile not found")
	}

	return nil
}

// PauseUserAutomations pauses all active automations for a user.
// This is used when a user's billing status changes to suspended.
func (r *profileRepository) PauseUserAutomations(ctx context.Context, userID string) error {
	now := time.Now().UTC()
	query := `
		UPDATE automations
		SET status = 'paused', updated_at = $1
		WHERE user_id = $2 AND status = 'active'
	`

	_, err := r.db.pool.Exec(ctx, query, now, userID)
	if err != nil {
		r.logger.Error("Failed to pause user automations",
			zap.String("userID", userID),
			zap.Error(err))
		return fmt.Errorf("pause automations: %w", err)
	}

	return nil
}

// ListProfiles returns profiles with cursor-based pagination
// Cursor format: base64(created_at:id)
func (r *profileRepository) ListProfiles(ctx context.Context, cursor string, limit int) ([]*models.Profile, string, error) {
	var query string
	var args []interface{}

	if cursor == "" {
		// First page
		query = `
			SELECT id, email, phone_number, plan, max_automations, billing_status,
			       password_hash, billing_next_payment_at, plan_access_until, created_at, updated_at
			FROM profiles
			ORDER BY created_at DESC, id DESC
			LIMIT $1
		`
		args = []interface{}{limit + 1} // +1 to detect hasMore
	} else {
		// Subsequent pages
		cursorTime, cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}

		query = `
			SELECT id, email, phone_number, plan, max_automations, billing_status,
			       password_hash, billing_next_payment_at, plan_access_until, created_at, updated_at
			FROM profiles
			WHERE (created_at, id) < ($1, $2)
			ORDER BY created_at DESC, id DESC
			LIMIT $3
		`
		args = []interface{}{cursorTime, cursorID, limit + 1}
	}

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query profiles: %w", err)
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		p, err := scanProfileRow(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("rows error: %w", err)
	}

	// Check if there's a next page
	var nextCursor string
	if len(profiles) > limit {
		profiles = profiles[:limit] // Remove the extra
		last := profiles[len(profiles)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}

	return profiles, nextCursor, nil
}

// SearchProfiles searches profiles by email or phone with cursor pagination
func (r *profileRepository) SearchProfiles(ctx context.Context, searchQuery, cursor string, limit int) ([]*models.Profile, string, error) {
	searchPattern := "%" + searchQuery + "%"

	var query string
	var args []interface{}

	if cursor == "" {
		query = `
			SELECT id, email, phone_number, plan, max_automations, billing_status,
			       password_hash, billing_next_payment_at, plan_access_until, created_at, updated_at
			FROM profiles
			WHERE email ILIKE $1 OR phone_number ILIKE $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		`
		args = []interface{}{searchPattern, limit + 1}
	} else {
		cursorTime, cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}

		query = `
			SELECT id, email, phone_number, plan, max_automations, billing_status,
			       password_hash, billing_next_payment_at, plan_access_until, created_at, updated_at
			FROM profiles
			WHERE (email ILIKE $1 OR phone_number ILIKE $1)
			  AND (created_at, id) < ($2, $3)
			ORDER BY created_at DESC, id DESC
			LIMIT $4
		`
		args = []interface{}{searchPattern, cursorTime, cursorID, limit + 1}
	}

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("query search: %w", err)
	}
	defer rows.Close()

	var profiles []*models.Profile
	for rows.Next() {
		p, err := scanProfileRow(rows)
		if err != nil {
			return nil, "", fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, p)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("rows error: %w", err)
	}

	var nextCursor string
	if len(profiles) > limit {
		profiles = profiles[:limit]
		last := profiles[len(profiles)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}

	return profiles, nextCursor, nil
}

// Helper to scan a profile row
func scanProfileRow(rows pgx.Rows) (*models.Profile, error) {
	var p models.Profile
	var phoneNumber, billingStatus, passwordHash *string
	var maxAutomations *int
	var billingNextPaymentAt, planAccessUntil *time.Time
	var createdAt, updatedAt time.Time

	err := rows.Scan(
		&p.ID, &p.Email, &phoneNumber, &p.Plan, &maxAutomations, &billingStatus,
		&passwordHash, &billingNextPaymentAt, &planAccessUntil, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	p.PhoneNumber = phoneNumber
	p.BillingStatus = billingStatus
	p.PasswordHash = passwordHash
	p.MaxAutomations = maxAutomations
	p.BillingNextPaymentAt = billingNextPaymentAt
	p.PlanAccessUntil = planAccessUntil
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &p, nil
}

// encodeCursor creates a cursor from created_at and id
func encodeCursor(createdAt, id string) string {
	return base64.URLEncoding.EncodeToString([]byte(createdAt + ":" + id))
}

// decodeCursor parses a cursor into created_at time and id
func decodeCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", err
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor format")
	}

	t, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid timestamp in cursor: %w", err)
	}

	return t, parts[1], nil
}

// GetMaxAutomations returns the maximum number of automations a user can create.
// Returns the profile's max_automations value, or 5 as default if not set or profile not found.
func (r *profileRepository) GetMaxAutomations(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COALESCE(max_automations, 5)
		FROM profiles
		WHERE id = $1
	`

	var maxAutomations int
	err := r.db.QueryRow(ctx, query, userID).Scan(&maxAutomations)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return default if profile not found
			return 5, nil
		}
		r.logger.Error("Failed to get max automations",
			zap.String("userID", userID),
			zap.Error(err))
		return 0, fmt.Errorf("failed to get max automations: %w", err)
	}

	return maxAutomations, nil
}

// CountUserAutomations returns the count of active automations for a user.
func (r *profileRepository) CountUserAutomations(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM automations
		WHERE user_id = $1 AND status = 'active'
	`

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count user automations",
			zap.String("userID", userID),
			zap.Error(err))
		return 0, fmt.Errorf("failed to count automations: %w", err)
	}

	return count, nil
}
