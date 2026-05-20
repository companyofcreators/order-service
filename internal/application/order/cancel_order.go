package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/infrastructure/kafka"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type CancelOrderHandler struct {
	orderRepo domain.OrderRepository
	kafkaProd *kafka.Producer
}

func NewCancelOrderHandler(
	orderRepo domain.OrderRepository,
	kafkaProd *kafka.Producer,
) *CancelOrderHandler {
	return &CancelOrderHandler{
		orderRepo: orderRepo,
		kafkaProd: kafkaProd,
	}
}

func (h *CancelOrderHandler) Handle(ctx context.Context, input domain.CancelOrderInput) (*domain.Order, error) {
	ord, err := h.orderRepo.FindByID(ctx, input.OrderID)
	if err != nil {
		return nil, err
	}

	// Customer can cancel their own orders
	// Moderator/admin can cancel any cancellable order
	isCustomer := ord.CustomerID == input.UserID
	isModeratorOrAdmin := input.UserRole == "admin" || input.UserRole == "moderator"

	if !isCustomer && !isModeratorOrAdmin {
		return nil, fmt.Errorf("%w: только заказчик, модератор или админ может отменить этот заказ", domain.ErrForbidden)
	}

	if !domain.IsCancellable(ord.Status) {
		return nil, fmt.Errorf("%w: заказ в статусе %s", domain.ErrOrderNotCancellable, ord.Status)
	}

	oldStatus := ord.Status
	ord.Status = domain.StatusCancelled
	ord.UpdatedAt = time.Now()

	if err := h.orderRepo.Update(ctx, ord); err != nil {
		return nil, fmt.Errorf("cancel order: %w", err)
	}

	// Record history
	history := &domain.OrderStatusHistory{
		ID:        uuid.New(),
		OrderID:   ord.ID,
		OldStatus: &oldStatus,
		NewStatus: domain.StatusCancelled,
		ChangedBy: input.UserID,
	}
	if err := h.orderRepo.AddStatusHistory(ctx, history); err != nil {
		pkg.Logger.WarnContext(ctx, "failed to record status history", "error", err.Error())
	}

	// Publish event
	go func() {
		event := kafka.OrderCancelledEvent{
			OrderID:     ord.ID.String(),
			CancelledBy: input.UserID.String(),
			Reason:      "отменён пользователем",
		}
		if err := h.kafkaProd.PublishOrderCancelled(context.Background(), event); err != nil {
			pkg.Logger.ErrorContext(context.Background(), "failed to publish order.cancelled event", "error", err.Error())
		}
	}()

	pkg.Logger.InfoContext(ctx, "order cancelled",
		"order_id", ord.ID.String(),
		"cancelled_by", input.UserID.String(),
	)

	return ord, nil
}
