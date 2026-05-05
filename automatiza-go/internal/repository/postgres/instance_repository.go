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
	now := time.Now().UTC()
	// is_default is intentionally not set here: on INSERT it defaults to FALSE
	// (a fresh chip is never automatically promoted to default), and on UPDATE
	// the existing flag is preserved by omitting it from the SET clause.
	query := `
		INSERT INTO instances (user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, instance_id) DO UPDATE SET
			friendly_name = COALESCE(EXCLUDED.friendly_name, instances.friendly_name),
			status = EXCLUDED.status,
			qr_code = EXCLUDED.qr_code,
			updated_at = EXCLUDED.updated_at
		RETURNING ` + instanceColumns

	var i models.Instance
	if err := scanInstance(r.db.QueryRow(ctx, query,
		userID, data.InstanceID, data.FriendlyName, data.Status, data.QRCode, now, now,
	), &i); err != nil {
		r.logger.Error("Failed to upsert instance",
			zap.String("userID", userID),
			zap.String("instanceID", data.InstanceID),
			zap.Error(err))
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
	query := `DELETE FROM instances WHERE id = $1`

	_, err := r.db.pool.Exec(ctx, query, instanceID)
	if err != nil {
		r.logger.Error("Failed to delete instance",
			zap.Int("instanceID", instanceID),
			zap.Error(err))
		return err
	}

	return nil
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
