package order

import (
	"context"

	"github.com/google/uuid"
)

type CreateOrderInput struct {
	CustomerID  uuid.UUID
	CategoryID  uuid.UUID
	Price       float64
	Currency    string
	Address     string
	Title       string
	Description   string
	Latitude      float64
	Longitude     float64
	AttachmentIDs []uuid.UUID
}

type UpdateStatusInput struct {
	OrderID   uuid.UUID
	NewStatus OrderStatus
	ChangedBy uuid.UUID
}

type CompleteOrderInput struct {
	OrderID    uuid.UUID
	CustomerID uuid.UUID
}

type CancelOrderInput struct {
	OrderID  uuid.UUID
	UserID   uuid.UUID
	UserRole string
}

type CreateReviewInput struct {
	OrderID    uuid.UUID
	FromUserID uuid.UUID
	ToUserID   uuid.UUID
	Rating     int
	Comment    string
}

type CreateComplaintInput struct {
	ReporterUserID uuid.UUID
	TargetUserID   uuid.UUID
	OrderID        *uuid.UUID
	Message        string
}

type UpdateComplaintInput struct {
	ComplaintID       uuid.UUID
	Status            ComplaintStatus
	ModeratorID       uuid.UUID
}

type CreateCategoryInput struct {
	Name     string
	ParentID *uuid.UUID
	Slug     string
}

type UpdateCategoryInput struct {
	ID       uuid.UUID
	Name     string
	ParentID *uuid.UUID
	Slug     string
}

type OrderService interface {
	CreateOrder(ctx context.Context, input CreateOrderInput) (*Order, error)
	GetOrder(ctx context.Context, orderID uuid.UUID) (*Order, error)
	ListOrders(ctx context.Context, filter OrderFilter, limit, offset int) ([]*Order, int, error)
	ListCustomerOrders(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*Order, int, error)
	GetStatusHistory(ctx context.Context, orderID uuid.UUID) ([]*OrderStatusHistory, error)
	UpdateStatus(ctx context.Context, input UpdateStatusInput) (*Order, error)
	AssignOrder(ctx context.Context, orderID, offerID, masterID uuid.UUID, finalPrice float64) (*Order, error)
	CompleteOrder(ctx context.Context, input CompleteOrderInput) (*Order, error)
	CancelOrder(ctx context.Context, input CancelOrderInput) (*Order, error)

	CreateCategory(ctx context.Context, input CreateCategoryInput) (*Category, error)
	UpdateCategory(ctx context.Context, input UpdateCategoryInput) (*Category, error)
	DeleteCategory(ctx context.Context, categoryID uuid.UUID) error
	ListCategories(ctx context.Context, treeView bool) (interface{}, error)

	CreateReview(ctx context.Context, input CreateReviewInput) (*Review, error)
	ListReviewsByUser(ctx context.Context, userID uuid.UUID, byMe bool, role string, limit, offset int) ([]*Review, int, float64, error)

	CreateComplaint(ctx context.Context, input CreateComplaintInput) (*Complaint, error)
	ListComplaints(ctx context.Context, status *ComplaintStatus, limit, offset int) ([]*Complaint, int, error)
	UpdateComplaint(ctx context.Context, input UpdateComplaintInput) (*Complaint, error)
}
