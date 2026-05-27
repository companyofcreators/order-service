package http

import (
	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
)

// Create Order
type CreateOrderRequest struct {
	CategoryID  uuid.UUID `json:"category_id"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	Address     string    `json:"address"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Latitude    float64  `json:"latitude"`
	Longitude   float64  `json:"longitude"`
}

type CreateOrderResponse struct {
	Order *domain.Order `json:"order"`
}

// Get Order
type GetOrderResponse struct {
	Order *domain.Order `json:"order"`
}

// List Orders
type ListOrdersResponse struct {
	Orders []*domain.Order `json:"orders"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// Update Status
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

type UpdateStatusResponse struct {
	Order *domain.Order `json:"order"`
}

// Complete Order
type CompleteOrderResponse struct {
	Order *domain.Order `json:"order"`
}

// Cancel Order
type CancelOrderResponse struct {
	Order *domain.Order `json:"order"`
}

// Status History
type StatusHistoryResponse struct {
	History []*domain.OrderStatusHistory `json:"history"`
}

// Category
type CreateCategoryRequest struct {
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Slug     string     `json:"slug,omitempty"`
}

type UpdateCategoryRequest struct {
	Name     string     `json:"name,omitempty"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Slug     string     `json:"slug,omitempty"`
}

type CategoryResponse struct {
	Category *domain.Category `json:"category"`
}

type CategoriesResponse struct {
	Categories interface{} `json:"categories"`
}

// Review
type CreateReviewRequest struct {
	OrderID  uuid.UUID `json:"order_id"`
	ToUserID uuid.UUID `json:"to_user_id"`
	Rating   int       `json:"rating"`
	Comment  string    `json:"comment"`
}

type CreateReviewResponse struct {
	Review *domain.Review `json:"review"`
}

type ListReviewsResponse struct {
	Reviews []*domain.Review `json:"reviews"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// Complaint
type CreateComplaintRequest struct {
	TargetUserID uuid.UUID  `json:"target_user_id"`
	OrderID      *uuid.UUID `json:"order_id,omitempty"`
	Message      string     `json:"message"`
}

type CreateComplaintResponse struct {
	Complaint *domain.Complaint `json:"complaint"`
}

type UpdateComplaintRequest struct {
	Status string `json:"status"`
}

type ListComplaintsResponse struct {
	Complaints []*domain.Complaint `json:"complaints"`
	Total      int                 `json:"total"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
}

// Error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// Health
type HealthResponse struct {
	Status string `json:"status"`
}
