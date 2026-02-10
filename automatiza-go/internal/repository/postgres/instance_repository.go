package postgres

import (
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
	"context"
	"errors"
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

func (r *instanceRepository) GetUserInstances(ctx context.Context, userID string) ([]models.Instance, error) {
	query := `
		SELECT id, user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at
		FROM instances
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []models.Instance
	for rows.Next() {
		var i models.Instance
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		i.CreatedAt = createdAt.Format(time.RFC3339)
		i.UpdatedAt = updatedAt.Format(time.RFC3339)
		instances = append(instances, i)
	}

	return instances, rows.Err()
}

func (r *instanceRepository) GetInstanceByID(ctx context.Context, instanceID int) (*models.Instance, error) {
	query := `
		SELECT id, user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at
		FROM instances
		WHERE id = $1
	`

	var i models.Instance
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query, instanceID).Scan(
		&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	i.CreatedAt = createdAt.Format(time.RFC3339)
	i.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &i, nil
}

func (r *instanceRepository) GetInstanceByName(ctx context.Context, instanceName string) (*models.Instance, error) {
	query := `
		SELECT id, user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at
		FROM instances
		WHERE instance_id = $1
	`

	var i models.Instance
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query, instanceName).Scan(
		&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	i.CreatedAt = createdAt.Format(time.RFC3339)
	i.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &i, nil
}

func (r *instanceRepository) GetInstanceByUserAndName(ctx context.Context, userID, instanceName string) (*models.Instance, error) {
	query := `
		SELECT id, user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at
		FROM instances
		WHERE user_id = $1 AND instance_id = $2
	`

	var i models.Instance
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query, userID, instanceName).Scan(
		&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	i.CreatedAt = createdAt.Format(time.RFC3339)
	i.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &i, nil
}

func (r *instanceRepository) GetAllConnectedInstances(ctx context.Context) ([]models.Instance, error) {
	query := `
		SELECT id, user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at
		FROM instances
		WHERE status IN ('connected', 'open')
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []models.Instance
	for rows.Next() {
		var i models.Instance
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		i.CreatedAt = createdAt.Format(time.RFC3339)
		i.UpdatedAt = updatedAt.Format(time.RFC3339)
		instances = append(instances, i)
	}

	return instances, rows.Err()
}

func (r *instanceRepository) UpsertInstance(ctx context.Context, userID string, data *models.Instance) (*models.Instance, error) {
	now := time.Now().UTC()
	query := `
		INSERT INTO instances (user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, instance_id) DO UPDATE SET
			friendly_name = COALESCE(EXCLUDED.friendly_name, instances.friendly_name),
			status = EXCLUDED.status,
			qr_code = EXCLUDED.qr_code,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, instance_id, friendly_name, status, qr_code, created_at, updated_at
	`

	var i models.Instance
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, query,
		userID, data.InstanceID, data.FriendlyName, data.Status, data.QRCode, now, now,
	).Scan(
		&i.ID, &i.UserID, &i.InstanceID, &i.FriendlyName, &i.Status, &i.QRCode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to upsert instance",
			zap.String("userID", userID),
			zap.String("instanceID", data.InstanceID),
			zap.Error(err))
		return nil, err
	}

	i.CreatedAt = createdAt.Format(time.RFC3339)
	i.UpdatedAt = updatedAt.Format(time.RFC3339)
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
