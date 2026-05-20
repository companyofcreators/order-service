package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
)

type ListOrdersHandler struct {
	orderRepo domain.OrderRepository
}

func NewListOrdersHandler(orderRepo domain.OrderRepository) *ListOrdersHandler {
	return &ListOrdersHandler{orderRepo: orderRepo}
}

type ListOrdersQuery struct {
	CustomerID *uuid.UUID
	Status     *domain.OrderStatus
	CategoryID *uuid.UUID
	Limit      int
	Offset     int
}

type ListOrdersResult struct {
	Orders []*domain.Order `json:"orders"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

func (h *ListOrdersHandler) Handle(ctx context.Context, query ListOrdersQuery) (*ListOrdersResult, error) {
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	filter := domain.OrderFilter{
		CustomerID: query.CustomerID,
		Status:     query.Status,
		CategoryID: query.CategoryID,
	}

	orders, total, err := h.orderRepo.List(ctx, filter, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	return &ListOrdersResult{
		Orders: orders,
		Total:  total,
		Limit:  query.Limit,
		Offset: query.Offset,
	}, nil
}
