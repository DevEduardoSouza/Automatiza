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
	"github.com/lib/pq"
	"go.uber.org/zap"
)

type planRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewPlanRepository creates a new PostgreSQL plan repository
func NewPlanRepository(db *DB, logger *zap.Logger) repository.PlanRepository {
	return &planRepository{
		db:     db,
		logger: logger,
	}
}

// GetByKiwifyProductID finds a plan by its Kiwify product ID.
// Checks all three Kiwify product ID columns (monthly, semester, annual).
func (r *planRepository) GetByKiwifyProductID(ctx context.Context, productID string) (*models.Plan, error) {
	if productID == "" {
		return nil, nil
	}

	query := `
		SELECT id, slug, name, is_active,
		       kiwify_product_id_monthly, kiwify_product_id_semester, kiwify_product_id_annual,
		       created_at, updated_at, description,
		       price_monthly, price_semester, price_annual,
		       group_limit, storage_limit, features
		FROM plans
		WHERE is_active = true
		  AND (kiwify_product_id_monthly = $1
		       OR kiwify_product_id_semester = $1
		       OR kiwify_product_id_annual = $1)
		LIMIT 1
	`

	var plan models.Plan
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, productID).Scan(
		&plan.ID, &plan.Slug, &plan.Name, &plan.IsActive,
		&plan.KiwifyProductIDMonthly, &plan.KiwifyProductIDSemester, &plan.KiwifyProductIDAnnual,
		&createdAt, &updatedAt, &plan.Description,
		&plan.PriceMonthly, &plan.PriceSemester, &plan.PriceAnnual,
		&plan.GroupLimit, &plan.StorageLimit, pq.Array(&plan.Features),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get plan by Kiwify product ID",
			zap.String("productID", productID),
			zap.Error(err))
		return nil, err
	}

	plan.CreatedAt = createdAt.Format(time.RFC3339)
	plan.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &plan, nil
}

// GetBySlug finds a plan by its slug.
func (r *planRepository) GetBySlug(ctx context.Context, slug string) (*models.Plan, error) {
	query := `
		SELECT id, slug, name, is_active,
		       kiwify_product_id_monthly, kiwify_product_id_semester, kiwify_product_id_annual,
		       created_at, updated_at, description,
		       price_monthly, price_semester, price_annual,
		       group_limit, storage_limit, features
		FROM plans
		WHERE slug = $1
		LIMIT 1
	`

	var plan models.Plan
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, slug).Scan(
		&plan.ID, &plan.Slug, &plan.Name, &plan.IsActive,
		&plan.KiwifyProductIDMonthly, &plan.KiwifyProductIDSemester, &plan.KiwifyProductIDAnnual,
		&createdAt, &updatedAt, &plan.Description,
		&plan.PriceMonthly, &plan.PriceSemester, &plan.PriceAnnual,
		&plan.GroupLimit, &plan.StorageLimit, pq.Array(&plan.Features),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get plan by slug",
			zap.String("slug", slug),
			zap.Error(err))
		return nil, err
	}

	plan.CreatedAt = createdAt.Format(time.RFC3339)
	plan.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &plan, nil
}

// GetActivePlans returns all active plans.
func (r *planRepository) GetActivePlans(ctx context.Context) ([]models.Plan, error) {
	query := `
		SELECT id, slug, name, is_active,
		       kiwify_product_id_monthly, kiwify_product_id_semester, kiwify_product_id_annual,
		       created_at, updated_at, description,
		       price_monthly, price_semester, price_annual,
		       group_limit, storage_limit, features
		FROM plans
		WHERE is_active = true
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to get active plans", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var plans []models.Plan
	for rows.Next() {
		var plan models.Plan
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&plan.ID, &plan.Slug, &plan.Name, &plan.IsActive,
			&plan.KiwifyProductIDMonthly, &plan.KiwifyProductIDSemester, &plan.KiwifyProductIDAnnual,
			&createdAt, &updatedAt, &plan.Description,
			&plan.PriceMonthly, &plan.PriceSemester, &plan.PriceAnnual,
			&plan.GroupLimit, &plan.StorageLimit, pq.Array(&plan.Features),
		)
		if err != nil {
			return nil, err
		}

		plan.CreatedAt = createdAt.Format(time.RFC3339)
		plan.UpdatedAt = updatedAt.Format(time.RFC3339)
		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

// GetByID finds a plan by its ID.
func (r *planRepository) GetByID(ctx context.Context, id string) (*models.Plan, error) {
	query := `
		SELECT id, slug, name, is_active,
		       kiwify_product_id_monthly, kiwify_product_id_semester, kiwify_product_id_annual,
		       created_at, updated_at, description,
		       price_monthly, price_semester, price_annual,
		       group_limit, storage_limit, features
		FROM plans
		WHERE id = $1
		LIMIT 1
	`

	var plan models.Plan
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(
		&plan.ID, &plan.Slug, &plan.Name, &plan.IsActive,
		&plan.KiwifyProductIDMonthly, &plan.KiwifyProductIDSemester, &plan.KiwifyProductIDAnnual,
		&createdAt, &updatedAt, &plan.Description,
		&plan.PriceMonthly, &plan.PriceSemester, &plan.PriceAnnual,
		&plan.GroupLimit, &plan.StorageLimit, pq.Array(&plan.Features),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get plan by ID",
			zap.String("id", id),
			zap.Error(err))
		return nil, err
	}

	plan.CreatedAt = createdAt.Format(time.RFC3339)
	plan.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &plan, nil
}

// GetAllPlans returns all plans (including inactive).
func (r *planRepository) GetAllPlans(ctx context.Context) ([]models.Plan, error) {
	query := `
		SELECT id, slug, name, is_active,
		       kiwify_product_id_monthly, kiwify_product_id_semester, kiwify_product_id_annual,
		       created_at, updated_at, description,
		       price_monthly, price_semester, price_annual,
		       group_limit, storage_limit, features
		FROM plans
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to get all plans", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var plans []models.Plan
	for rows.Next() {
		var plan models.Plan
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&plan.ID, &plan.Slug, &plan.Name, &plan.IsActive,
			&plan.KiwifyProductIDMonthly, &plan.KiwifyProductIDSemester, &plan.KiwifyProductIDAnnual,
			&createdAt, &updatedAt, &plan.Description,
			&plan.PriceMonthly, &plan.PriceSemester, &plan.PriceAnnual,
			&plan.GroupLimit, &plan.StorageLimit, pq.Array(&plan.Features),
		)
		if err != nil {
			return nil, err
		}

		plan.CreatedAt = createdAt.Format(time.RFC3339)
		plan.UpdatedAt = updatedAt.Format(time.RFC3339)
		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

// Create creates a new plan.
func (r *planRepository) Create(ctx context.Context, plan *models.Plan) (*models.Plan, error) {
	now := time.Now().UTC()

	query := `
		INSERT INTO plans (slug, name, is_active,
		                   kiwify_product_id_monthly, kiwify_product_id_semester, kiwify_product_id_annual,
		                   description, price_monthly, price_semester, price_annual,
		                   group_limit, storage_limit, features,
		                   created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at
	`

	var id string
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(ctx, query,
		plan.Slug, plan.Name, plan.IsActive,
		plan.KiwifyProductIDMonthly, plan.KiwifyProductIDSemester, plan.KiwifyProductIDAnnual,
		plan.Description, plan.PriceMonthly, plan.PriceSemester, plan.PriceAnnual,
		plan.GroupLimit, plan.StorageLimit, pq.Array(plan.Features),
		now, now,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		r.logger.Error("Failed to create plan",
			zap.String("slug", plan.Slug),
			zap.Error(err))
		return nil, err
	}

	plan.ID = id
	plan.CreatedAt = createdAt.Format(time.RFC3339)
	plan.UpdatedAt = updatedAt.Format(time.RFC3339)

	return plan, nil
}

// Update updates an existing plan.
func (r *planRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	now := time.Now().UTC()

	// Build dynamic update query
	setClauses := []string{"updated_at = $1"}
	args := []interface{}{now}
	argIndex := 2

	if slug, ok := updates["slug"].(string); ok && slug != "" {
		setClauses = append(setClauses, fmt.Sprintf("slug = $%d", argIndex))
		args = append(args, slug)
		argIndex++
	}
	if name, ok := updates["name"].(string); ok && name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, name)
		argIndex++
	}
	if isActive, ok := updates["is_active"].(bool); ok {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, isActive)
		argIndex++
	}
	if productIDMonthly, ok := updates["kiwify_product_id_monthly"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("kiwify_product_id_monthly = $%d", argIndex))
		args = append(args, productIDMonthly)
		argIndex++
	}
	if productIDSemester, ok := updates["kiwify_product_id_semester"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("kiwify_product_id_semester = $%d", argIndex))
		args = append(args, productIDSemester)
		argIndex++
	}
	if productIDAnnual, ok := updates["kiwify_product_id_annual"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("kiwify_product_id_annual = $%d", argIndex))
		args = append(args, productIDAnnual)
		argIndex++
	}
	if description, ok := updates["description"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, description)
		argIndex++
	}
	if priceMonthly, ok := updates["price_monthly"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("price_monthly = $%d", argIndex))
		args = append(args, priceMonthly)
		argIndex++
	}
	if priceSemester, ok := updates["price_semester"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("price_semester = $%d", argIndex))
		args = append(args, priceSemester)
		argIndex++
	}
	if priceAnnual, ok := updates["price_annual"]; ok {
		setClauses = append(setClauses, fmt.Sprintf("price_annual = $%d", argIndex))
		args = append(args, priceAnnual)
		argIndex++
	}
	if groupLimit, ok := updates["group_limit"].(int32); ok {
		setClauses = append(setClauses, fmt.Sprintf("group_limit = $%d", argIndex))
		args = append(args, groupLimit)
		argIndex++
	}
	if storageLimit, ok := updates["storage_limit"].(int64); ok {
		setClauses = append(setClauses, fmt.Sprintf("storage_limit = $%d", argIndex))
		args = append(args, storageLimit)
		argIndex++
	}
	if features, ok := updates["features"].([]string); ok {
		setClauses = append(setClauses, fmt.Sprintf("features = $%d", argIndex))
		args = append(args, pq.Array(features))
		argIndex++
	}

	query := fmt.Sprintf("UPDATE plans SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argIndex)
	args = append(args, id)

	_, err := r.db.pool.Exec(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to update plan",
			zap.String("id", id),
			zap.Error(err))
		return err
	}

	return nil
}

// Delete deletes a plan.
func (r *planRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM plans WHERE id = $1`

	result, err := r.db.pool.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete plan",
			zap.String("id", id),
			zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("plan not found")
	}

	return nil
}
