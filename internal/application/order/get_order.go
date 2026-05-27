package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
)

type GetOrderHandler struct {
	orderRepo domain.OrderRepository
}

func NewGetOrderHandler(orderRepo domain.OrderRepository) *GetOrderHandler {
	return &GetOrderHandler{orderRepo: orderRepo}
}

func (h *GetOrderHandler) Handle(ctx context.Context, orderID, customerID uuid.UUID) (*domain.Order, error) {
	ord, err := h.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Only the customer who created the order (or admin via different path) can view
	// Customer can view their own orders
	if ord.CustomerID != customerID {
		return nil, fmt.Errorf("%w: you can only view your own orders", domain.ErrForbidden)
	}

	return ord, nil
}

type GetOrderInternalHandler struct {
	orderRepo domain.OrderRepository
}

func NewGetOrderInternalHandler(orderRepo domain.OrderRepository) *GetOrderInternalHandler {
	return &GetOrderInternalHandler{orderRepo: orderRepo}
}

// Handle returns an order by ID without ownership validation (for internal/service use)
func (h *GetOrderInternalHandler) Handle(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	return h.orderRepo.FindByID(ctx, orderID)
}
