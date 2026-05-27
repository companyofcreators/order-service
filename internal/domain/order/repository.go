package order

import (
	"context"

	"github.com/google/uuid"
)

type OrderFilter struct {
	CustomerID *uuid.UUID
	Status     *OrderStatus
	CategoryID *uuid.UUID
	ActiveOnly bool // when true, filters to active statuses: created, negotiation, assigned, in_progress
}

type OrderRepository interface {
	Create(ctx context.Context, o *Order) error
	FindByID(ctx context.Context, id uuid.UUID) (*Order, error)
	ListByCustomer(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*Order, int, error)
	List(ctx context.Context, filter OrderFilter, limit, offset int) ([]*Order, int, error)
	Update(ctx context.Context, o *Order) error
	AddStatusHistory(ctx context.Context, h *OrderStatusHistory) error
	GetStatusHistory(ctx context.Context, orderID uuid.UUID) ([]*OrderStatusHistory, error)
}

type CategoryRepository interface {
	Create(ctx context.Context, c *Category) error
	FindByID(ctx context.Context, id uuid.UUID) (*Category, error)
	FindBySlug(ctx context.Context, slug string) (*Category, error)
	List(ctx context.Context, parentID *uuid.UUID) ([]*Category, error)
	ListTree(ctx context.Context) ([]*Category, error)
	Update(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id uuid.UUID) error
	HasChildren(ctx context.Context, id uuid.UUID) (bool, error)
}

type ReviewRepository interface {
	Create(ctx context.Context, r *Review) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Review, int, error)
	ListByOrder(ctx context.Context, orderID uuid.UUID) ([]*Review, error)
	CanReview(ctx context.Context, orderID, fromUserID uuid.UUID) (bool, error)
}

type ComplaintRepository interface {
	Create(ctx context.Context, c *Complaint) error
	FindByID(ctx context.Context, id uuid.UUID) (*Complaint, error)
	List(ctx context.Context, status *ComplaintStatus, limit, offset int) ([]*Complaint, int, error)
	Update(ctx context.Context, c *Complaint) error
}
