package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type instanceRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewInstanceRepository creates a new PostgreSQL instance repository
func NewInstanceRepository(db *DB, logger *zap.Logger) repository.InstanceRepository {
	return &instanceRepository{
		db:     db,
		logger: logger,
	}
}

const instanceColumns = `id, user_id, instance_id, friendly_name, status, qr_code, is_default, phone_number, created_at, updated_at`

func scanInstance(row pgx.Row, i *models.Instance) error {
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode, &i.IsDefault, &i.PhoneNumber,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return err
	}
	i.CreatedAt = createdAt.Format(time.RFC3339)
	i.UpdatedAt = updatedAt.Format(time.RFC3339)
	return nil
}

func (r *instanceRepository) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	query := `SELECT ` + instanceColumns + `
		FROM instances
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []models.Instance
	for rows.Next() {
		var i models.Instance
		if err := scanInstance(rows, &i); err != nil {
			return nil, err
		}
		instances = append(instances, i)
	}

	return instances, rows.Err()
}

func (r *instanceRepository) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	query := `SELECT ` + instanceColumns + ` FROM instances WHERE id = $1`

	var i models.Instance
	if err := scanInstance(r.db.QueryRow(ctx, query, instanceID), &i); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

func (r *instanceRepository) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	query := `SELECT ` + instanceColumns + ` FROM instances WHERE instance_id = $1`

	var i models.Instance
	if err := scanInstance(r.db.QueryRow(ctx, query, instanceName), &i); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

func (r *instanceRepository) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	query := `SELECT ` + instanceColumns + ` FROM instances WHERE user_id = $1 AND instance_id = $2`

	var i models.Instance
	if err := scanInstance(r.db.QueryRow(ctx, query, userID, instanceName), &i); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

func (r *instanceRepository) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	query := `SELECT ` + instanceColumns + `
		FROM instances
		WHERE status IN ('connected', 'open')
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []models.Instance
	for rows.Next() {
		var i models.Instance
		if err := scanInstance(rows, &i); err != nil {
			return nil, err
		}
		instances = append(instances, i)
	}

	return instances, rows.Err()
}

func (r *instanceRepository) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	// is_default is intentionally not set on the upsert itself: on INSERT it
	// defaults to FALSE and on UPDATE the existing flag is preserved by
	// omitting it from SET. Auto-promotion to default happens below in the
	// same transaction so a fresh user never ends up with zero defaults.
	upsertQuery := `
		INSERT INTO instances (user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, instance_id) DO UPDATE SET
			friendly_name = COALESCE(EXCLUDED.friendly_name, instances.friendly_name),
			status = EXCLUDED.status,
			qr_code = EXCLUDED.qr_code,
			updated_at = EXCLUDED.updated_at
		RETURNING ` + instanceColumns

	var i models.Instance
	if err := scanInstance(tx.QueryRow(ctx, upsertQuery,
		userID, data.InstanceID, data.FriendlyName, data.Status, data.QRCode, now, now,
	), &i); err != nil {
		r.logger.Error("Failed to upsert instance",
			zap.String("userID", userID),
			zap.String("instanceID", data.InstanceID),
			zap.Error(err))
		return nil, err
	}

	// Auto-promote: if the user has no default chip after this upsert,
	// mark this row as default. Idempotent — re-running on a user that
	// already has a default is a no-op. The unique partial index on
	// (user_id) WHERE is_default keeps concurrent racers honest.
	if !i.IsDefault {
		var hasDefault bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM instances WHERE user_id = $1 AND is_default = TRUE)`,
			userID,
		).Scan(&hasDefault); err != nil {
			return nil, err
		}
		if !hasDefault {
			if _, err := tx.Exec(ctx,
				`UPDATE instances SET is_default = TRUE, updated_at = $1 WHERE id = $2`,
				now, i.ID,
			); err != nil {
				return nil, err
			}
			i.IsDefault = true
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *instanceRepository) UpdateInstanceStatus(ctx context.Context, instanceID int, status string) error {
	query := `
		UPDATE instances
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.pool.Exec(ctx, query, status, time.Now().UTC(), instanceID)
	if err != nil {
		r.logger.Error("Failed to update instance status",
			zap.Int("instanceID", instanceID),
			zap.String("status", status),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *instanceRepository) DeleteInstance(ctx context.Context, instanceID int) error {
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the row and capture whether it was the default and which user
	// owned it, so we can pick a successor in the same transaction.
	var userID string
	var wasDefault bool
	err = tx.QueryRow(ctx,
		`SELECT user_id, is_default FROM instances WHERE id = $1 FOR UPDATE`,
		instanceID,
	).Scan(&userID, &wasDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		// Already gone — treat as success so idempotent retries don't error.
		return tx.Commit(ctx)
	}
	if err != nil {
		r.logger.Error("Failed to load instance for delete",
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM instances WHERE id = $1`, instanceID); err != nil {
		r.logger.Error("Failed to delete instance",
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return err
	}

	// Promote a successor when the deleted row was the user's default.
	// Prefer a connected chip; fall back to the oldest remaining row.
	// If the user has no chips left this is a no-op.
	if wasDefault {
		if _, err := tx.Exec(ctx,
			`UPDATE instances SET is_default = TRUE, updated_at = $1
			 WHERE id = (
				SELECT id FROM instances
				WHERE user_id = $2
				ORDER BY (status = 'connected') DESC, created_at ASC
				LIMIT 1
			 )`,
			time.Now().UTC(), userID,
		); err != nil {
			r.logger.Error("Failed to promote successor default",
				zap.String("userID", userID),
				zap.Error(err))
			return err
		}
	}

	return tx.Commit(ctx)
}

// UpdateInstanceQRCode updates only the QR code for an instance
func (r *instanceRepository) UpdateInstanceQRCode(ctx context.Context, instanceID int, qrCode string) error {
	query := `
		UPDATE instances
		SET qr_code = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.pool.Exec(ctx, query, qrCode, time.Now().UTC(), instanceID)
	return err
}

// ClearInstanceQRCode clears the QR code for an instance
func (r *instanceRepository) ClearInstanceQRCode(ctx context.Context, instanceID int) error {
	query := `
		UPDATE instances
		SET qr_code = NULL, updated_at = $1
		WHERE id = $2
	`

	_, err := r.db.pool.Exec(ctx, query, time.Now().UTC(), instanceID)
	return err
}

func (r *instanceRepository) SetDefaultInstance(ctx context.Context, userID string, instanceID int) error {
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()

	// Clear any existing default for this user. Cheap when no row matches.
	if _, err := tx.Exec(ctx,
		`UPDATE instances SET is_default = FALSE, updated_at = $1 WHERE user_id = $2 AND is_default = TRUE`,
		now, userID,
	); err != nil {
		return err
	}

	// Promote the requested instance. Ownership check is enforced inline.
	res, err := tx.Exec(ctx,
		`UPDATE instances SET is_default = TRUE, updated_at = $1 WHERE id = $2 AND user_id = $3`,
		now, instanceID, userID,
	)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("instance %d not found or not owned by user %s", instanceID, userID)
	}

	return tx.Commit(ctx)
}

func (r *instanceRepository) CountUserInstances(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM instances WHERE user_id = $1`, userID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *instanceRepository) UpdateInstancePhoneNumber(ctx context.Context, instanceID int, phoneNumber string) error {
	trimmed := strings.TrimSpace(phoneNumber)
	if trimmed == "" {
		return nil
	}
	// Only write when the value would actually change. Avoids churning
	// updated_at on every webhook tick once the chip is connected.
	_, err := r.db.pool.Exec(ctx,
		`UPDATE instances
		 SET phone_number = $1, updated_at = $2
		 WHERE id = $3 AND (phone_number IS NULL OR phone_number <> $1)`,
		trimmed, time.Now().UTC(), instanceID,
	)
	if err != nil {
		r.logger.Error("Failed to update instance phone_number",
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return err
	}
	return nil
}

// SupersedePriorInstances marks past instance rows of the same user as
// `superseded` once a fresh session connects. Hits two cohorts the UI
// no longer needs to display:
//
//  1. **Same phone, not currently connected**: previous chip rows of the
//     same number that got replaced by a new scan.
//  2. **NULL/empty phone, disconnected**: zombie scan attempts that
//     never authenticated past SCAN_QR_CODE — produced ~91% of the rows
//     in `instances` before this fix.
//
// Excludes `connected` rows so a multi-phone account keeps its other
// live chips untouched. Excludes already-`superseded` rows for
// idempotency — a webhook tick that re-runs the supersede pass should
// be a no-op, not churn `updated_at`.
//
// Returns the number of rows updated (useful for the worker log).
func (r *instanceRepository) SupersedePriorInstances(ctx context.Context, userID string, currentInstanceID int, phoneNumber string) (int, error) {
	trimmed := strings.TrimSpace(phoneNumber)
	if trimmed == "" {
		// Without a captured phone number we can't safely match prior
		// rows of the same chip — bail out instead of zapping every
		// disconnected row of the user.
		return 0, nil
	}
	res, err := r.db.pool.Exec(ctx,
		`UPDATE instances
		 SET status = $1, updated_at = $2
		 WHERE user_id = $3
		   AND id <> $4
		   AND status NOT IN ($1, $5)
		   AND (
		     phone_number = $6
		     OR phone_number IS NULL
		     OR phone_number = ''
		   )`,
		models.InstanceStatusSuperseded,
		time.Now().UTC(),
		userID,
		currentInstanceID,
		models.InstanceStatusConnected,
		trimmed,
	)
	if err != nil {
		r.logger.Error("Failed to supersede prior instances",
			zap.String("userID", userID),
			zap.Int("currentInstanceID", currentInstanceID),
			zap.Error(err))
		return 0, err
	}
	return int(res.RowsAffected()), nil
}

func (r *instanceRepository) UpdateInstanceLabel(ctx context.Context, userID string, instanceID int, friendlyName string) error {
	// Treat empty/whitespace input as "clear" — store NULL so reads fall back
	// to instance_id at the UI layer instead of showing an empty label.
	var label any
	trimmed := strings.TrimSpace(friendlyName)
	if trimmed == "" {
		label = nil
	} else {
		label = trimmed
	}

	res, err := r.db.pool.Exec(ctx,
		`UPDATE instances SET friendly_name = $1, updated_at = $2 WHERE id = $3 AND user_id = $4`,
		label, time.Now().UTC(), instanceID, userID,
	)
	if err != nil {
		r.logger.Error("Failed to update instance label",
			zap.String("userID", userID),
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return err
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("instance %d not found or not owned by user %s", instanceID, userID)
	}
	return nil
}
