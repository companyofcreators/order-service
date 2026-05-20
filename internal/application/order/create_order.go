package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/infrastructure/kafka"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type CreateOrderHandler struct {
	orderRepo   domain.OrderRepository
	historyRepo domain.OrderRepository // uses same interface for status history
	kafkaProd   *kafka.Producer
}

func NewCreateOrderHandler(
	orderRepo domain.OrderRepository,
	kafkaProd *kafka.Producer,
) *CreateOrderHandler {
	return &CreateOrderHandler{
		orderRepo: orderRepo,
		kafkaProd: kafkaProd,
	}
}

func (h *CreateOrderHandler) Handle(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
	if input.Title == "" {
		return nil, fmt.Errorf("%w: название обязательно", domain.ErrOrderNotFound)
	}
	if input.Price <= 0 {
		return nil, fmt.Errorf("цена должна быть больше нуля")
	}

	orderEntity := &domain.Order{
		ID:          uuid.New(),
		CustomerID:  input.CustomerID,
		CategoryID:  input.CategoryID,
		Status:      domain.StatusCreated,
		Price:       input.Price,
		Currency:    input.Currency,
		Address:     input.Address,
		Latitude:    input.Latitude,
		Longitude:   input.Longitude,
		Title:       input.Title,
		Description: input.Description,
	}

	if orderEntity.Currency == "" {
		orderEntity.Currency = "RUB"
	}

	if err := h.orderRepo.Create(ctx, orderEntity); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Record initial status history
	history := &domain.OrderStatusHistory{
		ID:        uuid.New(),
		OrderID:   orderEntity.ID,
		NewStatus: domain.StatusCreated,
		ChangedBy: input.CustomerID,
	}
	if err := h.orderRepo.AddStatusHistory(ctx, history); err != nil {
		pkg.Logger.WarnContext(ctx, "failed to record initial status history", "error", err.Error())
	}

	// Publish event (non-blocking)
	go func() {
		event := kafka.OrderCreatedEvent{
			OrderID:    orderEntity.ID.String(),
			CustomerID: orderEntity.CustomerID.String(),
			CategoryID: orderEntity.CategoryID.String(),
			Status:     string(orderEntity.Status),
			Price:      orderEntity.Price,
			Title:      orderEntity.Title,
		}
		if err := h.kafkaProd.PublishOrderCreated(context.Background(), event); err != nil {
			pkg.Logger.ErrorContext(context.Background(), "failed to publish order.created event", "error", err.Error())
		}
	}()

	pkg.Logger.InfoContext(ctx, "order created",
		"order_id", orderEntity.ID.String(),
		"customer_id", orderEntity.CustomerID.String(),
	)

	return orderEntity, nil
}
