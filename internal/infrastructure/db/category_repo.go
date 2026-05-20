package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type CategoryRepository struct {
	pool *sqlx.DB
}

func NewCategoryRepository(pool *sqlx.DB) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) Create(ctx context.Context, c *order.Category) error {
	query := `
		INSERT INTO categories (id, parent_id, name, slug, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}

	_, err := r.pool.ExecContext(ctx, query, c.ID, c.ParentID, c.Name, c.Slug, c.CreatedAt)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to create category", "error", err.Error())
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) FindByID(ctx context.Context, id uuid.UUID) (*order.Category, error) {
	query := `
		SELECT id, parent_id, name, slug, created_at
		FROM categories
		WHERE id = $1
	`

	c := &order.Category{}
	err := r.pool.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, order.ErrCategoryNotFound
		}
		return nil, fmt.Errorf("find category by id: %w", err)
	}
	return c, nil
}

func (r *CategoryRepository) FindBySlug(ctx context.Context, slug string) (*order.Category, error) {
	query := `
		SELECT id, parent_id, name, slug, created_at
		FROM categories
		WHERE slug = $1
	`

	c := &order.Category{}
	err := r.pool.QueryRowContext(ctx, query, slug).Scan(
		&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, order.ErrCategoryNotFound
		}
		return nil, fmt.Errorf("find category by slug: %w", err)
	}
	return c, nil
}

func (r *CategoryRepository) List(ctx context.Context, parentID *uuid.UUID) ([]*order.Category, error) {
	var query string
	var args []interface{}

	if parentID == nil {
		query = `SELECT id, parent_id, name, slug, created_at FROM categories ORDER BY name ASC`
	} else {
		query = `SELECT id, parent_id, name, slug, created_at FROM categories WHERE parent_id = $1 ORDER BY name ASC`
		args = append(args, *parentID)
	}

	rows, err := r.pool.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := make([]*order.Category, 0)
	for rows.Next() {
		c := &order.Category{}
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, c)
	}

	return categories, nil
}

func (r *CategoryRepository) ListTree(ctx context.Context) ([]*order.Category, error) {
	// Fetch all categories and build tree in application layer
	return r.List(ctx, nil)
}

func (r *CategoryRepository) Update(ctx context.Context, c *order.Category) error {
	query := `
		UPDATE categories
		SET parent_id = $2, name = $3, slug = $4
		WHERE id = $1
	`

	_, err := r.pool.ExecContext(ctx, query, c.ID, c.ParentID, c.Name, c.Slug)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to update category", "id", c.ID.String(), "error", err.Error())
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM categories WHERE id = $1`

	_, err := r.pool.ExecContext(ctx, query, id)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to delete category", "id", id.String(), "error", err.Error())
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

func (r *CategoryRepository) HasChildren(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM categories WHERE parent_id = $1)`

	var hasChildren bool
	err := r.pool.QueryRowContext(ctx, query, id).Scan(&hasChildren)
	if err != nil {
		return false, fmt.Errorf("check category children: %w", err)
	}
	return hasChildren, nil
}
