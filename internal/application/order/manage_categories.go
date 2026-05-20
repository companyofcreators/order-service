package order

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type ManageCategoriesHandler struct {
	categoryRepo domain.CategoryRepository
}

func NewManageCategoriesHandler(categoryRepo domain.CategoryRepository) *ManageCategoriesHandler {
	return &ManageCategoriesHandler{categoryRepo: categoryRepo}
}

func (h *ManageCategoriesHandler) Create(ctx context.Context, input domain.CreateCategoryInput) (*domain.Category, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("название категории обязательно")
	}

	// Generate slug from name if not provided
	slug := input.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	}

	// Check slug uniqueness
	existing, err := h.categoryRepo.FindBySlug(ctx, slug)
	if err != nil && err != domain.ErrCategoryNotFound {
		return nil, fmt.Errorf("check slug uniqueness: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrCategorySlugExists
	}

	cat := &domain.Category{
		ID:       uuid.New(),
		ParentID: input.ParentID,
		Name:     input.Name,
		Slug:     slug,
	}

	if err := h.categoryRepo.Create(ctx, cat); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}

	pkg.Logger.InfoContext(ctx, "category created",
		"category_id", cat.ID.String(),
		"name", cat.Name,
	)

	return cat, nil
}

func (h *ManageCategoriesHandler) Update(ctx context.Context, input domain.UpdateCategoryInput) (*domain.Category, error) {
	cat, err := h.categoryRepo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		cat.Name = input.Name
	}

	if input.Slug != "" {
		// Check slug uniqueness (excluding current category)
		existing, err := h.categoryRepo.FindBySlug(ctx, input.Slug)
		if err != nil && err != domain.ErrCategoryNotFound {
			return nil, fmt.Errorf("check slug uniqueness: %w", err)
		}
		if existing != nil && existing.ID != cat.ID {
			return nil, domain.ErrCategorySlugExists
		}
		cat.Slug = input.Slug
	}

	cat.ParentID = input.ParentID

	if err := h.categoryRepo.Update(ctx, cat); err != nil {
		return nil, fmt.Errorf("update category: %w", err)
	}

	pkg.Logger.InfoContext(ctx, "category updated",
		"category_id", cat.ID.String(),
		"name", cat.Name,
	)

	return cat, nil
}

func (h *ManageCategoriesHandler) Delete(ctx context.Context, categoryID uuid.UUID) error {
	// Check if category has children
	hasChildren, err := h.categoryRepo.HasChildren(ctx, categoryID)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	if hasChildren {
		return domain.ErrCategoryHasChildren
	}

	if err := h.categoryRepo.Delete(ctx, categoryID); err != nil {
		return fmt.Errorf("delete category: %w", err)
	}

	pkg.Logger.InfoContext(ctx, "category deleted", "category_id", categoryID.String())
	return nil
}

func (h *ManageCategoriesHandler) List(ctx context.Context, treeView bool) (interface{}, error) {
	if treeView {
		return h.listTree(ctx)
	}

	categories, err := h.categoryRepo.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return categories, nil
}

func (h *ManageCategoriesHandler) listTree(ctx context.Context) ([]*domain.CategoryTree, error) {
	allCats, err := h.categoryRepo.ListTree(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories for tree: %w", err)
	}

	// Build map
	catMap := make(map[uuid.UUID]*domain.CategoryTree)
	for _, c := range allCats {
		catMap[c.ID] = &domain.CategoryTree{Category: *c}
	}

	// Build tree structure
	roots := make([]*domain.CategoryTree, 0)
	for _, c := range allCats {
		treeNode := catMap[c.ID]
		if c.ParentID == nil {
			roots = append(roots, treeNode)
		} else {
			parent, ok := catMap[*c.ParentID]
			if ok {
				parent.Children = append(parent.Children, treeNode)
			} else {
				// Parent not found, treat as root
				roots = append(roots, treeNode)
			}
		}
	}

	return roots, nil
}
