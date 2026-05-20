package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type ReviewRepository struct {
	pool *sqlx.DB
}

func NewReviewRepository(pool *sqlx.DB) *ReviewRepository {
	return &ReviewRepository{pool: pool}
}

func (r *ReviewRepository) Create(ctx context.Context, rev *order.Review) error {
	query := `
		INSERT INTO reviews (id, order_id, from_user_id, to_user_id, rating, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	if rev.CreatedAt.IsZero() {
		rev.CreatedAt = time.Now()
	}

	_, err := r.pool.ExecContext(ctx, query,
		rev.ID, rev.OrderID, rev.FromUserID, rev.ToUserID,
		rev.Rating, rev.Comment, rev.CreatedAt,
	)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to create review", "error", err.Error())
		return fmt.Errorf("create review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*order.Review, int, error) {
	// Count all reviews for user (both given and received)
	countQuery := `SELECT COUNT(*) FROM reviews WHERE to_user_id = $1 OR from_user_id = $1`
	var total int
	if err := r.pool.QueryRowContext(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reviews: %w", err)
	}

	query := `
		SELECT id, order_id, from_user_id, to_user_id, rating, comment, created_at
		FROM reviews
		WHERE to_user_id = $1 OR from_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reviews by user: %w", err)
	}
	defer rows.Close()

	reviews := make([]*order.Review, 0)
	for rows.Next() {
		rev := &order.Review{}
		if err := rows.Scan(&rev.ID, &rev.OrderID, &rev.FromUserID, &rev.ToUserID, &rev.Rating, &rev.Comment, &rev.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, rev)
	}

	return reviews, total, nil
}

func (r *ReviewRepository) ListByOrder(ctx context.Context, orderID uuid.UUID) ([]*order.Review, error) {
	query := `
		SELECT id, order_id, from_user_id, to_user_id, rating, comment, created_at
		FROM reviews
		WHERE order_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("list reviews by order: %w", err)
	}
	defer rows.Close()

	reviews := make([]*order.Review, 0)
	for rows.Next() {
		rev := &order.Review{}
		if err := rows.Scan(&rev.ID, &rev.OrderID, &rev.FromUserID, &rev.ToUserID, &rev.Rating, &rev.Comment, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		reviews = append(reviews, rev)
	}

	return reviews, nil
}

func (r *ReviewRepository) CanReview(ctx context.Context, orderID, fromUserID uuid.UUID) (bool, error) {
	// Check order is completed
	var status string
	orderQuery := `SELECT status FROM orders WHERE id = $1`
	err := r.pool.QueryRowContext(ctx, orderQuery, orderID).Scan(&status)
	if err != nil {
		return false, fmt.Errorf("check order status for review: %w", err)
	}
	if status != "completed" {
		return false, nil
	}

	// Check user is participant (either customer or the master if offer accepted)
	var customerID uuid.UUID
	var acceptedOfferID *uuid.UUID
	participantQuery := `SELECT customer_id, accepted_offer_id FROM orders WHERE id = $1`
	if err := r.pool.QueryRowContext(ctx, participantQuery, orderID).Scan(&customerID, &acceptedOfferID); err != nil {
		return false, fmt.Errorf("check participants: %w", err)
	}

	isCustomer := customerID == fromUserID
	// We check if the user has already reviewed this order
	var exists bool
	existsQuery := `SELECT EXISTS(SELECT 1 FROM reviews WHERE order_id = $1 AND from_user_id = $2)`
	if err := r.pool.QueryRowContext(ctx, existsQuery, orderID, fromUserID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check existing review: %w", err)
	}
	if exists {
		return false, nil
	}

	// User must be the customer of the order
	// For master reviews, the master would need to be determined via accepted_offer_id
	return isCustomer, nil
}
