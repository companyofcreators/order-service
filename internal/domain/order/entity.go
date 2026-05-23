package order

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	StatusCreated     OrderStatus = "created"
	StatusNegotiation OrderStatus = "negotiation"
	StatusAssigned    OrderStatus = "assigned"
	StatusInProgress  OrderStatus = "in_progress"
	StatusCompleted   OrderStatus = "completed"
	StatusCancelled   OrderStatus = "cancelled"
)

type Order struct {
	ID              uuid.UUID  `json:"id"`
	CustomerID      uuid.UUID  `json:"customer_id"`
	AcceptedOfferID *uuid.UUID `json:"accepted_offer_id,omitempty"`
	CategoryID      uuid.UUID  `json:"category_id"`
	Status          OrderStatus `json:"status"`
	Price           float64    `json:"price"`
	FinalPrice      *float64   `json:"final_price,omitempty"`
	Currency        string     `json:"currency"`
	Address         string     `json:"address"`
	Latitude        float64    `json:"latitude"`
	Longitude       float64    `json:"longitude"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type OrderStatusHistory struct {
	ID        uuid.UUID    `json:"id"`
	OrderID   uuid.UUID    `json:"order_id"`
	OldStatus *OrderStatus `json:"old_status,omitempty"`
	NewStatus OrderStatus  `json:"new_status"`
	ChangedBy uuid.UUID    `json:"changed_by"`
	CreatedAt time.Time    `json:"created_at"`
}

type Category struct {
	ID        uuid.UUID  `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	CreatedAt time.Time  `json:"created_at"`
}

type CategoryTree struct {
	Category
	Children []*CategoryTree `json:"children,omitempty"`
}

type Review struct {
	ID         uuid.UUID `json:"id"`
	OrderID    uuid.UUID `json:"order_id"`
	FromUserID uuid.UUID `json:"from_user_id"`
	ToUserID   uuid.UUID `json:"to_user_id"`
	Rating     int       `json:"rating"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

type ComplaintStatus string

const (
	ComplaintPending  ComplaintStatus = "pending"
	ComplaintReview   ComplaintStatus = "in_review"
	ComplaintResolved ComplaintStatus = "resolved"
	ComplaintRejected ComplaintStatus = "rejected"
)

type Complaint struct {
	ID             uuid.UUID       `json:"id"`
	ReporterUserID uuid.UUID       `json:"reporter_user_id"`
	TargetUserID   uuid.UUID       `json:"target_user_id"`
	OrderID        *uuid.UUID      `json:"order_id,omitempty"`
	Message        string          `json:"message"`
	Status         ComplaintStatus `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
}

var ValidStatusTransitions = map[OrderStatus][]OrderStatus{
	StatusCreated:     {StatusNegotiation, StatusAssigned, StatusCancelled},
	StatusNegotiation: {StatusAssigned, StatusCancelled},
	StatusAssigned:    {StatusInProgress, StatusCancelled},
	StatusInProgress:  {StatusCompleted, StatusCancelled},
	StatusCompleted:   {},
	StatusCancelled:   {},
}

func IsValidTransition(from, to OrderStatus) bool {
	validTargets, ok := ValidStatusTransitions[from]
	if !ok {
		return false
	}
	for _, target := range validTargets {
		if target == to {
			return true
		}
	}
	return false
}

func IsCancellable(status OrderStatus) bool {
	return status == StatusCreated || status == StatusNegotiation || status == StatusAssigned
}
