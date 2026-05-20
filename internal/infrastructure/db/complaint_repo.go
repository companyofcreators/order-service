package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type ComplaintRepository struct {
	pool *sqlx.DB
}

func NewComplaintRepository(pool *sqlx.DB) *ComplaintRepository {
	return &ComplaintRepository{pool: pool}
}

func (r *ComplaintRepository) Create(ctx context.Context, c *order.Complaint) error {
	query := `
		INSERT INTO complaints (id, reporter_user_id, target_user_id, order_id, message, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}

	_, err := r.pool.ExecContext(ctx, query,
		c.ID, c.ReporterUserID, c.TargetUserID, c.OrderID,
		c.Message, string(c.Status), c.CreatedAt,
	)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to create complaint", "error", err.Error())
		return fmt.Errorf("create complaint: %w", err)
	}
	return nil
}

func (r *ComplaintRepository) FindByID(ctx context.Context, id uuid.UUID) (*order.Complaint, error) {
	query := `
		SELECT id, reporter_user_id, target_user_id, order_id, message, status, created_at
		FROM complaints
		WHERE id = $1
	`

	c := &order.Complaint{}
	var statusStr string
	err := r.pool.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.ReporterUserID, &c.TargetUserID, &c.OrderID,
		&c.Message, &statusStr, &c.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, order.ErrComplaintNotFound
		}
		return nil, fmt.Errorf("find complaint by id: %w", err)
	}
	c.Status = order.ComplaintStatus(statusStr)
	return c, nil
}

func (r *ComplaintRepository) List(ctx context.Context, status *order.ComplaintStatus, limit, offset int) ([]*order.Complaint, int, error) {
	var conditions string
	args := []interface{}{}
	argIdx := 1

	if status != nil {
		conditions = fmt.Sprintf("WHERE status = $%d", argIdx)
		args = append(args, string(*status))
		argIdx++
	}

	// Count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM complaints %s", conditions)
	var total int
	if err := r.pool.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count complaints: %w", err)
	}

	// Fetch
	dataQuery := fmt.Sprintf(`
		SELECT id, reporter_user_id, target_user_id, order_id, message, status, created_at
		FROM complaints %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, conditions, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list complaints: %w", err)
	}
	defer rows.Close()

	complaints := make([]*order.Complaint, 0)
	for rows.Next() {
		c := &order.Complaint{}
		var statusStr string
		if err := rows.Scan(
			&c.ID, &c.ReporterUserID, &c.TargetUserID, &c.OrderID,
			&c.Message, &statusStr, &c.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan complaint: %w", err)
		}
		c.Status = order.ComplaintStatus(statusStr)
		complaints = append(complaints, c)
	}

	return complaints, total, nil
}

func (r *ComplaintRepository) Update(ctx context.Context, c *order.Complaint) error {
	query := `
		UPDATE complaints
		SET status = $2, message = $3
		WHERE id = $1
	`

	_, err := r.pool.ExecContext(ctx, query, c.ID, string(c.Status), c.Message)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to update complaint", "id", c.ID.String(), "error", err.Error())
		return fmt.Errorf("update complaint: %w", err)
	}
	return nil
}
