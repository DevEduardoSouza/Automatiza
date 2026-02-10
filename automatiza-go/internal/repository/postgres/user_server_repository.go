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

type userServerRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewUserServerRepository creates a new PostgreSQL user server repository
func NewUserServerRepository(db *DB, logger *zap.Logger) repository.UserServerRepository {
	return &userServerRepository{
		db:     db,
		logger: logger,
	}
}

func (r *userServerRepository) GetByUserID(ctx context.Context, userID string) (*models.UserServerMapping, error) {
	query := `
		SELECT id, user_id, server_id, created_at, updated_at
		FROM user_servers
		WHERE user_id = $1
	`

	var mapping models.UserServerMapping
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&mapping.ID, &mapping.UserID, &mapping.ServerID, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	mapping.CreatedAt = createdAt.Format(time.RFC3339)
	mapping.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &mapping, nil
}

func (r *userServerRepository) Create(ctx context.Context, mapping *models.UserServerMapping) error {
	now := time.Now().UTC()

	query := `
		INSERT INTO user_servers (user_id, server_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	err := r.db.QueryRow(ctx, query, mapping.UserID, mapping.ServerID, now, now).Scan(&mapping.ID)
	if err != nil {
		r.logger.Error("Failed to create user server mapping",
			zap.String("userID", mapping.UserID),
			zap.String("serverID", mapping.ServerID),
			zap.Error(err))
		return err
	}

	mapping.CreatedAt = now.Format(time.RFC3339)
	mapping.UpdatedAt = now.Format(time.RFC3339)

	return nil
}

func (r *userServerRepository) Update(ctx context.Context, userID, serverID string) error {
	now := time.Now().UTC()

	query := `
		UPDATE user_servers
		SET server_id = $1, updated_at = $2
		WHERE user_id = $3
	`

	result, err := r.db.pool.Exec(ctx, query, serverID, now, userID)
	if err != nil {
		r.logger.Error("Failed to update user server mapping",
			zap.String("userID", userID),
			zap.String("serverID", serverID),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		// Create if not exists
		mapping := &models.UserServerMapping{
			UserID:   userID,
			ServerID: serverID,
		}
		return r.Create(ctx, mapping)
	}

	return nil
}

func (r *userServerRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM user_servers WHERE user_id = $1`

	_, err := r.db.pool.Exec(ctx, query, userID)
	if err != nil {
		r.logger.Error("Failed to delete user server mapping",
			zap.String("userID", userID),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *userServerRepository) CountByServer(ctx context.Context, serverID string) (int, error) {
	query := `SELECT COUNT(*) FROM user_servers WHERE server_id = $1`

	var count int
	err := r.db.QueryRow(ctx, query, serverID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetAll retrieves all user server mappings
func (r *userServerRepository) GetAll(ctx context.Context) ([]models.UserServerMapping, error) {
	query := `
		SELECT id, user_id, server_id, created_at, updated_at
		FROM user_servers
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []models.UserServerMapping
	for rows.Next() {
		var m models.UserServerMapping
		var createdAt, updatedAt time.Time

		err := rows.Scan(&m.ID, &m.UserID, &m.ServerID, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}

		m.CreatedAt = createdAt.Format(time.RFC3339)
		m.UpdatedAt = updatedAt.Format(time.RFC3339)
		mappings = append(mappings, m)
	}

	return mappings, rows.Err()
}

// GetServerStats returns the count of users per server
func (r *userServerRepository) GetServerStats(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT server_id, COUNT(*) as count
		FROM user_servers
		GROUP BY server_id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var serverID string
		var count int
		if err := rows.Scan(&serverID, &count); err != nil {
			return nil, err
		}
		stats[serverID] = count
	}

	return stats, rows.Err()
}
