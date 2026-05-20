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

type CompleteOrderHandler struct {
	orderRepo domain.OrderRepository
	kafkaProd *kafka.Producer
}

func NewCompleteOrderHandler(
	orderRepo domain.OrderRepository,
	kafkaProd *kafka.Producer,
) *CompleteOrderHandler {
	return &CompleteOrderHandler{
		orderRepo: orderRepo,
		kafkaProd: kafkaProd,
	}
}

func (h *CompleteOrderHandler) Handle(ctx context.Context, input domain.CompleteOrderInput) (*domain.Order, error) {
	ord, err := h.orderRepo.FindByID(ctx, input.OrderID)
	if err != nil {
		return nil, err
	}

	// Only the customer can complete the order
	if ord.CustomerID != input.CustomerID {
		return nil, fmt.Errorf("%w: только заказчик может завершить заказ", domain.ErrForbidden)
	}

	if ord.Status != domain.StatusInProgress {
		return nil, fmt.Errorf("%w: заказ должен быть в статусе in_progress для завершения, текущий статус: %s",
			domain.ErrOrderNotCompletable, ord.Status)
	}

	oldStatus := ord.Status
	now := time.Now()
	ord.Status = domain.StatusCompleted
	ord.CompletedAt = &now
	ord.UpdatedAt = now

	if err := h.orderRepo.Update(ctx, ord); err != nil {
		return nil, fmt.Errorf("complete order: %w", err)
	}

	// Record history
	history := &domain.OrderStatusHistory{
		ID:        uuid.New(),
		OrderID:   ord.ID,
		OldStatus: &oldStatus,
		NewStatus: domain.StatusCompleted,
		ChangedBy: input.CustomerID,
	}
	if err := h.orderRepo.AddStatusHistory(ctx, history); err != nil {
		pkg.Logger.WarnContext(ctx, "failed to record status history", "error", err.Error())
	}

	// Publish event
	go func() {
		masterID := ""
		if ord.AcceptedOfferID != nil {
			masterID = ord.AcceptedOfferID.String()
		}
		finalPrice := 0.0
		if ord.FinalPrice != nil {
			finalPrice = *ord.FinalPrice
		} else {
			finalPrice = ord.Price
		}
		event := kafka.OrderCompletedEvent{
			OrderID:    ord.ID.String(),
			CustomerID: ord.CustomerID.String(),
			MasterID:   masterID,
			FinalPrice: finalPrice,
		}
		if err := h.kafkaProd.PublishOrderCompleted(context.Background(), event); err != nil {
			pkg.Logger.ErrorContext(context.Background(), "failed to publish order.completed event", "error", err.Error())
		}
	}()

	pkg.Logger.InfoContext(ctx, "order completed",
		"order_id", ord.ID.String(),
		"customer_id", ord.CustomerID.String(),
	)

	return ord, nil
}
