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

var (
	ErrCategoryNotFound     = errors.New("category not found")
	ErrSubcategoryNotFound  = errors.New("subcategory not found")
	ErrSystemCategoryDelete = errors.New("cannot delete system category")
	ErrSystemSubcatDelete   = errors.New("cannot delete system subcategory")
)

type categoryRepository struct {
	db     *DB
	logger *zap.Logger
}

// NewCategoryRepository creates a new PostgreSQL category repository
func NewCategoryRepository(db *DB, logger *zap.Logger) repository.CategoryRepository {
	return &categoryRepository{
		db:     db,
		logger: logger,
	}
}

// ListCategories returns all categories, optionally including inactive ones
func (r *categoryRepository) ListCategories(ctx context.Context, includeInactive bool) ([]models.Category, error) {
	query := `
		SELECT id, slug, name, icon, display_order, is_active, has_refinements, is_system, created_at, updated_at
		FROM categories
	`
	if !includeInactive {
		query += ` WHERE is_active = true`
	}
	query += ` ORDER BY display_order, name`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to list categories", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var cat models.Category
		err := rows.Scan(
			&cat.ID, &cat.Slug, &cat.Name, &cat.Icon, &cat.DisplayOrder,
			&cat.IsActive, &cat.HasRefinements, &cat.IsSystem,
			&cat.CreatedAt, &cat.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// ListCategoriesWithSubcategories returns categories with their subcategories loaded
func (r *categoryRepository) ListCategoriesWithSubcategories(ctx context.Context, includeInactive bool) ([]models.Category, error) {
	// First get categories
	categories, err := r.ListCategories(ctx, includeInactive)
	if err != nil {
		return nil, err
	}

	if len(categories) == 0 {
		return categories, nil
	}

	// Build category ID list and map
	categoryIDs := make([]string, len(categories))
	categoryMap := make(map[string]*models.Category)
	for i := range categories {
		categoryIDs[i] = categories[i].ID
		categoryMap[categories[i].ID] = &categories[i]
	}

	// Query all subcategories for these categories
	query := `
		SELECT id, category_id, slug, name, display_key, group_name, display_order, is_active, is_system, created_at, updated_at
		FROM subcategories
		WHERE category_id = ANY($1)
	`
	if !includeInactive {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY display_order, name`

	rows, err := r.db.Query(ctx, query, categoryIDs)
	if err != nil {
		r.logger.Error("Failed to list subcategories", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sub models.Subcategory
		err := rows.Scan(
			&sub.ID, &sub.CategoryID, &sub.Slug, &sub.Name, &sub.DisplayKey,
			&sub.GroupName, &sub.DisplayOrder, &sub.IsActive, &sub.IsSystem,
			&sub.CreatedAt, &sub.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if cat, ok := categoryMap[sub.CategoryID]; ok {
			cat.Subcategories = append(cat.Subcategories, sub)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

// GetCategoryByID retrieves a category by its ID
func (r *categoryRepository) GetCategoryByID(ctx context.Context, id string) (*models.Category, error) {
	query := `
		SELECT id, slug, name, icon, display_order, is_active, has_refinements, is_system, created_at, updated_at
		FROM categories
		WHERE id = $1
	`

	var cat models.Category
	err := r.db.QueryRow(ctx, query, id).Scan(
		&cat.ID, &cat.Slug, &cat.Name, &cat.Icon, &cat.DisplayOrder,
		&cat.IsActive, &cat.HasRefinements, &cat.IsSystem,
		&cat.CreatedAt, &cat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get category by ID", zap.String("id", id), zap.Error(err))
		return nil, err
	}

	return &cat, nil
}

// GetCategoryBySlug retrieves a category by its slug
func (r *categoryRepository) GetCategoryBySlug(ctx context.Context, slug string) (*models.Category, error) {
	query := `
		SELECT id, slug, name, icon, display_order, is_active, has_refinements, is_system, created_at, updated_at
		FROM categories
		WHERE slug = $1
	`

	var cat models.Category
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&cat.ID, &cat.Slug, &cat.Name, &cat.Icon, &cat.DisplayOrder,
		&cat.IsActive, &cat.HasRefinements, &cat.IsSystem,
		&cat.CreatedAt, &cat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get category by slug", zap.String("slug", slug), zap.Error(err))
		return nil, err
	}

	return &cat, nil
}

// CreateCategory creates a new category
func (r *categoryRepository) CreateCategory(ctx context.Context, req *models.CreateCategoryRequest) (*models.Category, error) {
	now := time.Now().UTC()

	displayOrder := 0
	if req.DisplayOrder != nil {
		displayOrder = *req.DisplayOrder
	}

	query := `
		INSERT INTO categories (slug, name, icon, display_order, is_active, has_refinements, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, false, $6, $7)
		RETURNING id, slug, name, icon, display_order, is_active, has_refinements, is_system, created_at, updated_at
	`

	var cat models.Category
	err := r.db.QueryRow(ctx, query,
		req.Slug, req.Name, req.Icon, displayOrder, req.HasRefinements, now, now,
	).Scan(
		&cat.ID, &cat.Slug, &cat.Name, &cat.Icon, &cat.DisplayOrder,
		&cat.IsActive, &cat.HasRefinements, &cat.IsSystem,
		&cat.CreatedAt, &cat.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to create category", zap.String("slug", req.Slug), zap.Error(err))
		return nil, err
	}

	return &cat, nil
}

// UpdateCategory updates an existing category
func (r *categoryRepository) UpdateCategory(ctx context.Context, id string, req *models.UpdateCategoryRequest) (*models.Category, error) {
	now := time.Now().UTC()

	// Build dynamic update query
	setClauses := []string{"updated_at = $1"}
	args := []interface{}{now}
	argIndex := 2

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *req.Name)
		argIndex++
	}
	if req.Icon != nil {
		setClauses = append(setClauses, fmt.Sprintf("icon = $%d", argIndex))
		args = append(args, *req.Icon)
		argIndex++
	}
	if req.DisplayOrder != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_order = $%d", argIndex))
		args = append(args, *req.DisplayOrder)
		argIndex++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}
	if req.HasRefinements != nil {
		setClauses = append(setClauses, fmt.Sprintf("has_refinements = $%d", argIndex))
		args = append(args, *req.HasRefinements)
		argIndex++
	}

	query := fmt.Sprintf(`
		UPDATE categories SET %s WHERE id = $%d
		RETURNING id, slug, name, icon, display_order, is_active, has_refinements, is_system, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIndex)
	args = append(args, id)

	var cat models.Category
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&cat.ID, &cat.Slug, &cat.Name, &cat.Icon, &cat.DisplayOrder,
		&cat.IsActive, &cat.HasRefinements, &cat.IsSystem,
		&cat.CreatedAt, &cat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCategoryNotFound
		}
		r.logger.Error("Failed to update category", zap.String("id", id), zap.Error(err))
		return nil, err
	}

	return &cat, nil
}

// DeleteCategory deletes a category (only if not is_system)
func (r *categoryRepository) DeleteCategory(ctx context.Context, id string) error {
	// First check if it's a system category
	cat, err := r.GetCategoryByID(ctx, id)
	if err != nil {
		return err
	}
	if cat == nil {
		return ErrCategoryNotFound
	}
	if cat.IsSystem {
		return ErrSystemCategoryDelete
	}

	query := `DELETE FROM categories WHERE id = $1 AND is_system = false`
	result, err := r.db.pool.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete category", zap.String("id", id), zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}

	return nil
}

// GetSubcategoriesByCategoryID retrieves all subcategories for a category
func (r *categoryRepository) GetSubcategoriesByCategoryID(ctx context.Context, categoryID string) ([]models.Subcategory, error) {
	query := `
		SELECT id, category_id, slug, name, display_key, group_name, display_order, is_active, is_system, created_at, updated_at
		FROM subcategories
		WHERE category_id = $1
		ORDER BY display_order, name
	`

	rows, err := r.db.Query(ctx, query, categoryID)
	if err != nil {
		r.logger.Error("Failed to get subcategories by category ID", zap.String("categoryID", categoryID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var subcategories []models.Subcategory
	for rows.Next() {
		var sub models.Subcategory
		err := rows.Scan(
			&sub.ID, &sub.CategoryID, &sub.Slug, &sub.Name, &sub.DisplayKey,
			&sub.GroupName, &sub.DisplayOrder, &sub.IsActive, &sub.IsSystem,
			&sub.CreatedAt, &sub.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		subcategories = append(subcategories, sub)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subcategories, nil
}

// GetSubcategoryByID retrieves a subcategory by its ID
func (r *categoryRepository) GetSubcategoryByID(ctx context.Context, id string) (*models.Subcategory, error) {
	query := `
		SELECT id, category_id, slug, name, display_key, group_name, display_order, is_active, is_system, created_at, updated_at
		FROM subcategories
		WHERE id = $1
	`

	var sub models.Subcategory
	err := r.db.QueryRow(ctx, query, id).Scan(
		&sub.ID, &sub.CategoryID, &sub.Slug, &sub.Name, &sub.DisplayKey,
		&sub.GroupName, &sub.DisplayOrder, &sub.IsActive, &sub.IsSystem,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error("Failed to get subcategory by ID", zap.String("id", id), zap.Error(err))
		return nil, err
	}

	return &sub, nil
}

// CreateSubcategory creates a new subcategory
func (r *categoryRepository) CreateSubcategory(ctx context.Context, req *models.CreateSubcategoryRequest) (*models.Subcategory, error) {
	now := time.Now().UTC()

	displayOrder := 0
	if req.DisplayOrder != nil {
		displayOrder = *req.DisplayOrder
	}

	query := `
		INSERT INTO subcategories (category_id, slug, name, display_key, group_name, display_order, is_active, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, false, $7, $8)
		RETURNING id, category_id, slug, name, display_key, group_name, display_order, is_active, is_system, created_at, updated_at
	`

	var sub models.Subcategory
	err := r.db.QueryRow(ctx, query,
		req.CategoryID, req.Slug, req.Name, req.DisplayKey, req.GroupName, displayOrder, now, now,
	).Scan(
		&sub.ID, &sub.CategoryID, &sub.Slug, &sub.Name, &sub.DisplayKey,
		&sub.GroupName, &sub.DisplayOrder, &sub.IsActive, &sub.IsSystem,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Failed to create subcategory",
			zap.String("categoryID", req.CategoryID),
			zap.String("slug", req.Slug),
			zap.Error(err))
		return nil, err
	}

	return &sub, nil
}

// UpdateSubcategory updates an existing subcategory
func (r *categoryRepository) UpdateSubcategory(ctx context.Context, id string, req *models.UpdateSubcategoryRequest) (*models.Subcategory, error) {
	now := time.Now().UTC()

	// Build dynamic update query
	setClauses := []string{"updated_at = $1"}
	args := []interface{}{now}
	argIndex := 2

	if req.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, *req.Name)
		argIndex++
	}
	if req.DisplayKey != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_key = $%d", argIndex))
		args = append(args, *req.DisplayKey)
		argIndex++
	}
	if req.GroupName != nil {
		setClauses = append(setClauses, fmt.Sprintf("group_name = $%d", argIndex))
		args = append(args, *req.GroupName)
		argIndex++
	}
	if req.DisplayOrder != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_order = $%d", argIndex))
		args = append(args, *req.DisplayOrder)
		argIndex++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIndex))
		args = append(args, *req.IsActive)
		argIndex++
	}

	query := fmt.Sprintf(`
		UPDATE subcategories SET %s WHERE id = $%d
		RETURNING id, category_id, slug, name, display_key, group_name, display_order, is_active, is_system, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIndex)
	args = append(args, id)

	var sub models.Subcategory
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&sub.ID, &sub.CategoryID, &sub.Slug, &sub.Name, &sub.DisplayKey,
		&sub.GroupName, &sub.DisplayOrder, &sub.IsActive, &sub.IsSystem,
		&sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSubcategoryNotFound
		}
		r.logger.Error("Failed to update subcategory", zap.String("id", id), zap.Error(err))
		return nil, err
	}

	return &sub, nil
}

// DeleteSubcategory deletes a subcategory (only if not is_system)
func (r *categoryRepository) DeleteSubcategory(ctx context.Context, id string) error {
	// First check if it's a system subcategory
	sub, err := r.GetSubcategoryByID(ctx, id)
	if err != nil {
		return err
	}
	if sub == nil {
		return ErrSubcategoryNotFound
	}
	if sub.IsSystem {
		return ErrSystemSubcatDelete
	}

	query := `DELETE FROM subcategories WHERE id = $1 AND is_system = false`
	result, err := r.db.pool.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete subcategory", zap.String("id", id), zap.Error(err))
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrSubcategoryNotFound
	}

	return nil
}
