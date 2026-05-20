package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/infrastructure/kafka"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type CreateComplaintHandler struct {
	complaintRepo domain.ComplaintRepository
	kafkaProd     *kafka.Producer
}

func NewCreateComplaintHandler(
	complaintRepo domain.ComplaintRepository,
	kafkaProd *kafka.Producer,
) *CreateComplaintHandler {
	return &CreateComplaintHandler{
		complaintRepo: complaintRepo,
		kafkaProd:     kafkaProd,
	}
}

func (h *CreateComplaintHandler) Handle(ctx context.Context, input domain.CreateComplaintInput) (*domain.Complaint, error) {
	if input.Message == "" {
		return nil, fmt.Errorf("текст жалобы обязателен")
	}
	if input.ReporterUserID == input.TargetUserID {
		return nil, fmt.Errorf("нельзя жаловаться на себя")
	}

	complaint := &domain.Complaint{
		ID:             uuid.New(),
		ReporterUserID: input.ReporterUserID,
		TargetUserID:   input.TargetUserID,
		OrderID:        input.OrderID,
		Message:        input.Message,
		Status:         domain.ComplaintPending,
	}

	if err := h.complaintRepo.Create(ctx, complaint); err != nil {
		return nil, fmt.Errorf("create complaint: %w", err)
	}

	// Publish event
	go func() {
		orderIDStr := ""
		if complaint.OrderID != nil {
			orderIDStr = complaint.OrderID.String()
		}
		event := kafka.ComplaintCreatedEvent{
			ComplaintID: complaint.ID.String(),
			ReporterID:  complaint.ReporterUserID.String(),
			TargetID:    complaint.TargetUserID.String(),
			OrderID:     orderIDStr,
			Message:     complaint.Message,
		}
		if err := h.kafkaProd.PublishComplaintCreated(context.Background(), event); err != nil {
			pkg.Logger.ErrorContext(context.Background(), "failed to publish complaint.created event", "error", err.Error())
		}
	}()

	pkg.Logger.InfoContext(ctx, "complaint created",
		"complaint_id", complaint.ID.String(),
		"reporter_id", complaint.ReporterUserID.String(),
	)

	return complaint, nil
}
