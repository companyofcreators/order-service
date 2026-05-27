package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
)

type ListReviewsHandler struct {
	reviewRepo domain.ReviewRepository
}

func NewListReviewsHandler(reviewRepo domain.ReviewRepository) *ListReviewsHandler {
	return &ListReviewsHandler{reviewRepo: reviewRepo}
}

type ListReviewsQuery struct {
	UserID uuid.UUID
	ByMe   bool
	Role   string
	Limit  int
	Offset int
}

type ListReviewsResult struct {
	Reviews []*domain.Review `json:"reviews"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

func (h *ListReviewsHandler) Handle(ctx context.Context, query ListReviewsQuery) (*ListReviewsResult, error) {
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 20
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	reviews, total, err := h.reviewRepo.ListByUser(ctx, query.UserID, query.ByMe, query.Role, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	return &ListReviewsResult{
		Reviews: reviews,
		Total:   total,
		Limit:   query.Limit,
		Offset:  query.Offset,
	}, nil
}
