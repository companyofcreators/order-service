package order

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/infrastructure/kafka"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type CreateReviewHandler struct {
	reviewRepo domain.ReviewRepository
	kafkaProd  *kafka.Producer
}

func NewCreateReviewHandler(
	reviewRepo domain.ReviewRepository,
	kafkaProd *kafka.Producer,
) *CreateReviewHandler {
	return &CreateReviewHandler{
		reviewRepo: reviewRepo,
		kafkaProd:  kafkaProd,
	}
}

func (h *CreateReviewHandler) Handle(ctx context.Context, input domain.CreateReviewInput) (*domain.Review, error) {
	if input.Rating < 1 || input.Rating > 5 {
		return nil, domain.ErrInvalidRating
	}

	// Check if user can review this order
	canReview, err := h.reviewRepo.CanReview(ctx, input.OrderID, input.FromUserID)
	if err != nil {
		return nil, fmt.Errorf("check review eligibility: %w", err)
	}
	if !canReview {
		return nil, fmt.Errorf("%w: невозможно оставить отзыв на заказ %s", domain.ErrOrderNotCompleted, input.OrderID)
	}

	review := &domain.Review{
		ID:         uuid.New(),
		OrderID:    input.OrderID,
		FromUserID: input.FromUserID,
		ToUserID:   input.ToUserID,
		Rating:     input.Rating,
		Comment:    input.Comment,
	}

	if err := h.reviewRepo.Create(ctx, review); err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}

	// Publish event
	go func() {
		event := kafka.ReviewCreatedEvent{
			ReviewID:   review.ID.String(),
			OrderID:    review.OrderID.String(),
			FromUserID: review.FromUserID.String(),
			ToUserID:   review.ToUserID.String(),
			Rating:     review.Rating,
		}
		if err := h.kafkaProd.PublishReviewCreated(context.Background(), event); err != nil {
			pkg.Logger.ErrorContext(context.Background(), "failed to publish review.created event", "error", err.Error())
		}
	}()

	pkg.Logger.InfoContext(ctx, "review created",
		"review_id", review.ID.String(),
		"order_id", review.OrderID.String(),
		"rating", review.Rating,
	)

	return review, nil
}
