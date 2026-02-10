package models

import "time"

// Category represents a product category for automation configuration
type Category struct {
	ID             string    `json:"id" db:"id"`
	Slug           string    `json:"slug" db:"slug"`
	Name           string    `json:"name" db:"name"`
	Icon           *string   `json:"icon,omitempty" db:"icon"`
	DisplayOrder   int       `json:"display_order" db:"display_order"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	HasRefinements bool      `json:"has_refinements" db:"has_refinements"`
	IsSystem       bool      `json:"is_system" db:"is_system"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`

	// Subcategories loaded via join (not stored in DB column)
	Subcategories []Subcategory `json:"subcategories,omitempty" db:"-"`
}

// Subcategory represents a subcategory within a category
type Subcategory struct {
	ID           string    `json:"id" db:"id"`
	CategoryID   string    `json:"category_id" db:"category_id"`
	Slug         string    `json:"slug" db:"slug"`
	Name         string    `json:"name" db:"name"`
	DisplayKey   string    `json:"display_key" db:"display_key"`
	GroupName    *string   `json:"group_name,omitempty" db:"group_name"`
	DisplayOrder int       `json:"display_order" db:"display_order"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	IsSystem     bool      `json:"is_system" db:"is_system"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// CreateCategoryRequest represents a request to create a new category
type CreateCategoryRequest struct {
	Slug           string  `json:"slug" validate:"required,max=50"`
	Name           string  `json:"name" validate:"required,max=100"`
	Icon           *string `json:"icon,omitempty"`
	DisplayOrder   *int    `json:"display_order,omitempty"`
	HasRefinements bool    `json:"has_refinements"`
}

// UpdateCategoryRequest represents a request to update an existing category
type UpdateCategoryRequest struct {
	Name           *string `json:"name,omitempty"`
	Icon           *string `json:"icon,omitempty"`
	DisplayOrder   *int    `json:"display_order,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
	HasRefinements *bool   `json:"has_refinements,omitempty"`
}

// CreateSubcategoryRequest represents a request to create a new subcategory
type CreateSubcategoryRequest struct {
	CategoryID   string  `json:"category_id" validate:"required,uuid"`
	Slug         string  `json:"slug" validate:"required,max=100"`
	Name         string  `json:"name" validate:"required,max=150"`
	DisplayKey   string  `json:"display_key" validate:"required,max=200"`
	GroupName    *string `json:"group_name,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

// UpdateSubcategoryRequest represents a request to update an existing subcategory
type UpdateSubcategoryRequest struct {
	Name         *string `json:"name,omitempty"`
	DisplayKey   *string `json:"display_key,omitempty"`
	GroupName    *string `json:"group_name,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
}
