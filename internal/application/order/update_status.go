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

type UpdateStatusHandler struct {
	orderRepo domain.OrderRepository
	kafkaProd *kafka.Producer
}

func NewUpdateStatusHandler(
	orderRepo domain.OrderRepository,
	kafkaProd *kafka.Producer,
) *UpdateStatusHandler {
	return &UpdateStatusHandler{
		orderRepo: orderRepo,
		kafkaProd: kafkaProd,
	}
}

func (h *UpdateStatusHandler) Handle(ctx context.Context, input domain.UpdateStatusInput) (*domain.Order, error) {
	ord, err := h.orderRepo.FindByID(ctx, input.OrderID)
	if err != nil {
		return nil, err
	}

	if !domain.IsValidTransition(ord.Status, input.NewStatus) {
		return nil, fmt.Errorf("%w: недопустимый переход из %s в %s",
			domain.ErrInvalidTransition, ord.Status, input.NewStatus)
	}

	oldStatus := ord.Status
	ord.Status = input.NewStatus

	if err := h.orderRepo.Update(ctx, ord); err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	// Record status history
	history := &domain.OrderStatusHistory{
		ID:        uuid.New(),
		OrderID:   ord.ID,
		OldStatus: &oldStatus,
		NewStatus: input.NewStatus,
		ChangedBy: input.ChangedBy,
	}
	if err := h.orderRepo.AddStatusHistory(ctx, history); err != nil {
		pkg.Logger.WarnContext(ctx, "failed to record status history", "error", err.Error())
	}

	// Publish appropriate event
	go h.publishStatusEvent(ord, input.ChangedBy)

	pkg.Logger.InfoContext(ctx, "order status updated",
		"order_id", ord.ID.String(),
		"from", string(oldStatus),
		"to", string(input.NewStatus),
	)

	return ord, nil
}

func (h *UpdateStatusHandler) publishStatusEvent(ord *domain.Order, changedBy uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch ord.Status {
	case domain.StatusAssigned:
		event := kafka.OrderAssignedEvent{
			OrderID:    ord.ID.String(),
			CustomerID: ord.CustomerID.String(),
			MasterID:   changedBy.String(),
		}
		if err := h.kafkaProd.PublishOrderAssigned(ctx, event); err != nil {
			pkg.Logger.ErrorContext(ctx, "failed to publish order.assigned event", "error", err.Error())
		}

	case domain.StatusCompleted:
		masterID := ""
		if ord.AcceptedOfferID != nil {
			masterID = ord.AcceptedOfferID.String()
		}
		finalPrice := 0.0
		if ord.FinalPrice != nil {
			finalPrice = *ord.FinalPrice
		}
		event := kafka.OrderCompletedEvent{
			OrderID:    ord.ID.String(),
			CustomerID: ord.CustomerID.String(),
			MasterID:   masterID,
			FinalPrice: finalPrice,
		}
		if err := h.kafkaProd.PublishOrderCompleted(ctx, event); err != nil {
			pkg.Logger.ErrorContext(ctx, "failed to publish order.completed event", "error", err.Error())
		}

	case domain.StatusCancelled:
		event := kafka.OrderCancelledEvent{
			OrderID:     ord.ID.String(),
			CancelledBy: changedBy.String(),
			Reason:      "заказ отменён",
		}
		if err := h.kafkaProd.PublishOrderCancelled(ctx, event); err != nil {
			pkg.Logger.ErrorContext(ctx, "failed to publish order.cancelled event", "error", err.Error())
		}
	}
}
