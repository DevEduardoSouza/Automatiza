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

type userRoleRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewUserRoleRepository creates a new PostgreSQL user role repository
func NewUserRoleRepository(db *DB, logger *zap.Logger) repository.UserRoleRepository {
	return &userRoleRepository{
		db:     db,
		logger: logger,
	}
}

func (r *userRoleRepository) GetUserRole(ctx context.Context, userID string) (*models.UserRole, error) {
	query := `
		SELECT id, user_id, role, created_at
		FROM user_roles
		WHERE user_id = $1
	`

	var role models.UserRole
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, userID).Scan(&role.ID, &role.UserID, &role.Role, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	role.CreatedAt = createdAt.Format(time.RFC3339)
	return &role, nil
}

func (r *userRoleRepository) SetUserRole(ctx context.Context, userID, role string) error {
	now := time.Now().UTC()

	query := `
		INSERT INTO user_roles (user_id, role, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, role) DO NOTHING
	`

	_, err := r.db.pool.Exec(ctx, query, userID, role, now)
	if err != nil {
		r.logger.Error("Failed to set user role",
			zap.String("userID", userID),
			zap.String("role", role),
			zap.Error(err))
		return err
	}

	return nil
}

func (r *userRoleRepository) IsAdmin(ctx context.Context, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM user_roles WHERE user_id = $1 AND role = 'admin')`

	var isAdmin bool
	err := r.db.QueryRow(ctx, query, userID).Scan(&isAdmin)
	if err != nil {
		return false, err
	}

	return isAdmin, nil
}

// RemoveUserRole removes a specific role from a user
func (r *userRoleRepository) RemoveUserRole(ctx context.Context, userID, role string) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role = $2`

	_, err := r.db.pool.Exec(ctx, query, userID, role)
	if err != nil {
		r.logger.Error("Failed to remove user role",
			zap.String("userID", userID),
			zap.String("role", role),
			zap.Error(err))
		return err
	}

	return nil
}

// GetAllUserRoles retrieves all roles for a user
func (r *userRoleRepository) GetAllUserRoles(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT role FROM user_roles WHERE user_id = $1`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, rows.Err()
}

// GetAdminUsers retrieves all privileged team users (admin and support roles).
func (r *userRoleRepository) GetAdminUsers(ctx context.Context) ([]models.UserRole, error) {
	query := `
		SELECT id, user_id, role, created_at
		FROM user_roles
		WHERE role IN ('admin', 'support')
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.UserRole
	for rows.Next() {
		var role models.UserRole
		var createdAt time.Time

		err := rows.Scan(&role.ID, &role.UserID, &role.Role, &createdAt)
		if err != nil {
			return nil, err
		}

		role.CreatedAt = createdAt.Format(time.RFC3339)
		roles = append(roles, role)
	}

	return roles, rows.Err()
}
