package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type OrderRepository struct {
	pool *sqlx.DB
}

func NewOrderRepository(pool *sqlx.DB) *OrderRepository {
	return &OrderRepository{pool: pool}
}

func (r *OrderRepository) Create(ctx context.Context, o *order.Order) error {
	query := `
		INSERT INTO orders (id, customer_id, category_id, status, price, currency, address, latitude, longitude, title, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	now := time.Now()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = now
	}
	if o.Currency == "" {
		o.Currency = "RUB"
	}

	_, err := r.pool.ExecContext(ctx, query,
		o.ID, o.CustomerID, o.CategoryID, string(o.Status),
		o.Price, o.Currency, o.Address, o.Latitude, o.Longitude,
		o.Title, o.Description, o.CreatedAt, o.UpdatedAt,
	)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to create order", "error", err.Error())
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

func (r *OrderRepository) FindByID(ctx context.Context, id uuid.UUID) (*order.Order, error) {
	query := `
		SELECT id, customer_id, accepted_offer_id, category_id, status, price, final_price,
		       currency, address, latitude, longitude, title, description,
		       created_at, updated_at, completed_at
		FROM orders
		WHERE id = $1
	`

	o := &order.Order{}
	var statusStr string
	err := r.pool.QueryRowContext(ctx, query, id).Scan(
		&o.ID, &o.CustomerID, &o.AcceptedOfferID, &o.CategoryID, &statusStr,
		&o.Price, &o.FinalPrice, &o.Currency, &o.Address,
		&o.Latitude, &o.Longitude, &o.Title, &o.Description,
		&o.CreatedAt, &o.UpdatedAt, &o.CompletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, order.ErrOrderNotFound
		}
		return nil, fmt.Errorf("find order by id: %w", err)
	}
	o.Status = order.OrderStatus(statusStr)
	return o, nil
}

func (r *OrderRepository) ListByCustomer(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*order.Order, int, error) {
	filter := order.OrderFilter{CustomerID: &customerID}
	return r.List(ctx, filter, limit, offset)
}

func (r *OrderRepository) List(ctx context.Context, filter order.OrderFilter, limit, offset int) ([]*order.Order, int, error) {
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

	if filter.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argIdx))
		args = append(args, *filter.CustomerID)
		argIdx++
	}

	if filter.ActiveOnly {
		conditions = append(conditions, "status IN ('created','negotiation','assigned','in_progress')")
	} else if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}

	if filter.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", argIdx))
		args = append(args, *filter.CategoryID)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM orders %s", whereClause)
	var total int
	err := r.pool.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	// Fetch page
	dataQuery := fmt.Sprintf(`
		SELECT id, customer_id, accepted_offer_id, category_id, status, price, final_price,
		       currency, address, latitude, longitude, title, description,
		       created_at, updated_at, completed_at
		FROM orders %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]*order.Order, 0)
	for rows.Next() {
		o := &order.Order{}
		var statusStr string
		if err := rows.Scan(
			&o.ID, &o.CustomerID, &o.AcceptedOfferID, &o.CategoryID, &statusStr,
			&o.Price, &o.FinalPrice, &o.Currency, &o.Address,
			&o.Latitude, &o.Longitude, &o.Title, &o.Description,
			&o.CreatedAt, &o.UpdatedAt, &o.CompletedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan order: %w", err)
		}
		o.Status = order.OrderStatus(statusStr)
		orders = append(orders, o)
	}

	return orders, total, nil
}

func (r *OrderRepository) Update(ctx context.Context, o *order.Order) error {
	query := `
		UPDATE orders
		SET accepted_offer_id = $2, status = $3, price = $4, final_price = $5,
		    currency = $6, address = $7, latitude = $8, longitude = $9,
		    title = $10, description = $11, updated_at = $12, completed_at = $13
		WHERE id = $1
	`

	o.UpdatedAt = time.Now()

	_, err := r.pool.ExecContext(ctx, query,
		o.ID, o.AcceptedOfferID, string(o.Status), o.Price, o.FinalPrice,
		o.Currency, o.Address, o.Latitude, o.Longitude,
		o.Title, o.Description, o.UpdatedAt, o.CompletedAt,
	)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to update order", "order_id", o.ID.String(), "error", err.Error())
		return fmt.Errorf("update order: %w", err)
	}
	return nil
}

func (r *OrderRepository) AddStatusHistory(ctx context.Context, h *order.OrderStatusHistory) error {
	query := `
		INSERT INTO order_status_history (id, order_id, old_status, new_status, changed_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	now := time.Now()
	if h.CreatedAt.IsZero() {
		h.CreatedAt = now
	}

	var oldStatus *string
	if h.OldStatus != nil {
		s := string(*h.OldStatus)
		oldStatus = &s
	}

	_, err := r.pool.ExecContext(ctx, query,
		h.ID, h.OrderID, oldStatus, string(h.NewStatus), h.ChangedBy, h.CreatedAt,
	)
	if err != nil {
		pkg.Logger.ErrorContext(ctx, "failed to add status history", "error", err.Error())
		return fmt.Errorf("add status history: %w", err)
	}
	return nil
}

func (r *OrderRepository) GetStatusHistory(ctx context.Context, orderID uuid.UUID) ([]*order.OrderStatusHistory, error) {
	query := `
		SELECT id, order_id, old_status, new_status, changed_by, created_at
		FROM order_status_history
		WHERE order_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("get status history: %w", err)
	}
	defer rows.Close()

	history := make([]*order.OrderStatusHistory, 0)
	for rows.Next() {
		h := &order.OrderStatusHistory{}
		var oldStatus *string
		var newStatusStr string

		if err := rows.Scan(&h.ID, &h.OrderID, &oldStatus, &newStatusStr, &h.ChangedBy, &h.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan status history: %w", err)
		}

		if oldStatus != nil {
			os := order.OrderStatus(*oldStatus)
			h.OldStatus = &os
		}
		h.NewStatus = order.OrderStatus(newStatusStr)
		history = append(history, h)
	}

	return history, nil
}
