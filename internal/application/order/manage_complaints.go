package order

import (
	"context"
	"fmt"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type ManageComplaintsHandler struct {
	complaintRepo domain.ComplaintRepository
}

func NewManageComplaintsHandler(complaintRepo domain.ComplaintRepository) *ManageComplaintsHandler {
	return &ManageComplaintsHandler{complaintRepo: complaintRepo}
}

type ListComplaintsQuery struct {
	Status *domain.ComplaintStatus
	Limit  int
	Offset int
}

type ListComplaintsResult struct {
	Complaints []*domain.Complaint `json:"complaints"`
	Total      int                 `json:"total"`
	Limit      int                 `json:"limit"`
	Offset     int                 `json:"offset"`
}

func (h *ManageComplaintsHandler) List(ctx context.Context, query ListComplaintsQuery) (*ListComplaintsResult, error) {
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	complaints, total, err := h.complaintRepo.List(ctx, query.Status, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list complaints: %w", err)
	}

	return &ListComplaintsResult{
		Complaints: complaints,
		Total:      total,
		Limit:      query.Limit,
		Offset:     query.Offset,
	}, nil
}

var validComplaintTransitions = map[domain.ComplaintStatus][]domain.ComplaintStatus{
	domain.ComplaintPending:  {domain.ComplaintReview, domain.ComplaintRejected},
	domain.ComplaintReview:   {domain.ComplaintResolved, domain.ComplaintRejected},
	domain.ComplaintResolved: {},
	domain.ComplaintRejected: {},
}

func (h *ManageComplaintsHandler) Update(ctx context.Context, input domain.UpdateComplaintInput) (*domain.Complaint, error) {
	complaint, err := h.complaintRepo.FindByID(ctx, input.ComplaintID)
	if err != nil {
		return nil, err
	}

	// Validate status transition
	validTargets, ok := validComplaintTransitions[complaint.Status]
	if !ok {
		return nil, domain.ErrInvalidComplaintStatus
	}

	isValid := false
	for _, target := range validTargets {
		if target == input.Status {
			isValid = true
			break
		}
	}
	if !isValid {
		return nil, fmt.Errorf("%w: недопустимый переход из %s в %s",
			domain.ErrInvalidComplaintStatus, complaint.Status, input.Status)
	}

	complaint.Status = input.Status

	if err := h.complaintRepo.Update(ctx, complaint); err != nil {
		return nil, fmt.Errorf("update complaint: %w", err)
	}

	pkg.Logger.InfoContext(ctx, "complaint updated",
		"complaint_id", complaint.ID.String(),
		"status", string(complaint.Status),
	)

	return complaint, nil
}
