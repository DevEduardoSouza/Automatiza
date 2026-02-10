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

type emailTemplateRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewEmailTemplateRepository creates a new PostgreSQL email template repository
func NewEmailTemplateRepository(db *DB, logger *zap.Logger) repository.EmailTemplateRepository {
	return &emailTemplateRepository{
		db:     db,
		logger: logger,
	}
}

func (r *emailTemplateRepository) GetByName(ctx context.Context, name string) (*models.EmailTemplate, error) {
	query := `
		SELECT id, name, subject, body, is_active, created_at, updated_at, description
		FROM email_templates
		WHERE name = $1 AND is_active = true
	`

	var template models.EmailTemplate
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, name).Scan(
		&template.ID, &template.Name, &template.Subject, &template.Body, &template.IsActive, &createdAt, &updatedAt, &template.Description,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	template.CreatedAt = createdAt.Format(time.RFC3339)
	template.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &template, nil
}

func (r *emailTemplateRepository) GetAll(ctx context.Context) ([]models.EmailTemplate, error) {
	query := `
		SELECT id, name, subject, body, is_active, created_at, updated_at, description
		FROM email_templates
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []models.EmailTemplate
	for rows.Next() {
		var t models.EmailTemplate
		var createdAt, updatedAt time.Time

		err := rows.Scan(&t.ID, &t.Name, &t.Subject, &t.Body, &t.IsActive, &createdAt, &updatedAt, &t.Description)
		if err != nil {
			return nil, err
		}

		t.CreatedAt = createdAt.Format(time.RFC3339)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		templates = append(templates, t)
	}

	return templates, rows.Err()
}

// GetByID retrieves an email template by ID
func (r *emailTemplateRepository) GetByID(ctx context.Context, id string) (*models.EmailTemplate, error) {
	query := `
		SELECT id, name, subject, body, is_active, created_at, updated_at, description
		FROM email_templates
		WHERE id = $1
	`

	var template models.EmailTemplate
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(
		&template.ID, &template.Name, &template.Subject, &template.Body, &template.IsActive, &createdAt, &updatedAt, &template.Description,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	template.CreatedAt = createdAt.Format(time.RFC3339)
	template.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &template, nil
}

// Create creates a new email template
func (r *emailTemplateRepository) Create(ctx context.Context, template *models.EmailTemplate) (*models.EmailTemplate, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO email_templates (name, subject, body, is_active, created_at, updated_at, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, template.Name, template.Subject, template.Body, template.IsActive, now, now, template.Description).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		r.logger.Error("Failed to create email template",
			zap.String("name", template.Name),
			zap.Error(err))
		return nil, err
	}

	template.ID = id
	template.CreatedAt = createdAt.Format(time.RFC3339)
	template.UpdatedAt = updatedAt.Format(time.RFC3339)

	return template, nil
}

// Update updates an email template by ID
func (r *emailTemplateRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	now := time.Now().UTC()

	// Build dynamic update query
	setClauses := []string{"updated_at = $1"}
	args := []interface{}{now}
	argIndex := 2

	if name, ok := updates["name"].(string); ok && name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, name)
		argIndex++
	}
	if subject, ok := updates["subject"].(string); ok && subject != "" {
		setClauses = append(setClauses, fmt.Sprintf("subject = $%d", argIndex))
		args = append(args, subject)
		argIndex++
	}
	if body, ok := updates["body"].(string); ok && body != "" {
		setClauses = append(setClauses, fmt.Sprintf("body = $%d", argIndex))
		args = append(args, body)
		argIndex++
	}
	if isActive, ok := updates["is_active"].(bool); ok {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, isActive)
		argIndex++
	}
	if description, ok := updates["description"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, description)
		argIndex++
	}

	query := fmt.Sprintf("UPDATE email_templates SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argIndex)
	args = append(args, id)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update email template",
			zap.String("id", id),
			zap.Error(err))
		return err
	}

	return nil
}

// SetActive sets the active status of an email template
func (r *emailTemplateRepository) SetActive(ctx context.Context, id string, isActive bool) error {
	query := `
		UPDATE email_templates
		SET is_active = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := r.db.pool.Exec(ctx, query, isActive, time.Now().UTC(), id)
	if err != nil {
		r.logger.Error("Failed to set email template active status",
			zap.String("id", id),
			zap.Bool("isActive", isActive),
			zap.Error(err))
		return err
	}

	return nil
}

// Delete removes an email template
func (r *emailTemplateRepository) Delete(ctx context.Context, name string) error {
	query := `DELETE FROM email_templates WHERE name = $1`

	_, err := r.db.pool.Exec(ctx, query, name)
	return err
}
