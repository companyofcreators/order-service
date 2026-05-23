package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/companyofcreators/order-service/internal/config"
	usecases "github.com/companyofcreators/order-service/internal/application/order"
	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/infrastructure/db"
	"github.com/companyofcreators/order-service/internal/infrastructure/kafka"
	wsinfra "github.com/companyofcreators/order-service/internal/infrastructure/ws"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type Container struct {
	cfg       *config.Config
	pool      *sqlx.DB
	kafkaProd *kafka.Producer

	// Repositories
	OrderRepo     domain.OrderRepository
	CategoryRepo  domain.CategoryRepository
	ReviewRepo    domain.ReviewRepository
	ComplaintRepo domain.ComplaintRepository

	// Service
	OrderService domain.OrderService

	// WebSocket
	WSHub    *wsinfra.Hub
}

func NewContainer(cfg *config.Config) (*Container, error) {
	// Database
	pool, err := db.NewPostgresPool(cfg.DBDSN, cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	// Kafka
	prod := kafka.NewProducer(cfg)

	// Repositories
	orderRepo := db.NewOrderRepository(pool)
	categoryRepo := db.NewCategoryRepository(pool)
	reviewRepo := db.NewReviewRepository(pool)
	complaintRepo := db.NewComplaintRepository(pool)

	// WebSocket Hub
	wsHub := wsinfra.NewHub(pkg.Logger)

	// Service
	svc := NewOrderService(orderRepo, categoryRepo, reviewRepo, complaintRepo, prod, wsHub)

	return &Container{
		cfg:           cfg,
		pool:          pool,
		kafkaProd:     prod,
		OrderRepo:     orderRepo,
		CategoryRepo:  categoryRepo,
		ReviewRepo:    reviewRepo,
		ComplaintRepo: complaintRepo,
		OrderService:  svc,
		WSHub:         wsHub,
	}, nil
}

func (c *Container) Shutdown(ctx context.Context) {
	pkg.Logger.InfoContext(ctx, "shutting down order service...")

	if c.kafkaProd != nil {
		if err := c.kafkaProd.Close(); err != nil {
			pkg.Logger.ErrorContext(ctx, "failed to close kafka producer", "error", err.Error())
		}
	}

	if c.pool != nil {
		c.pool.Close()
	}

	pkg.Logger.InfoContext(ctx, "order service shutdown complete")
}

// OrderService implementation

type orderService struct {
	orderRepo     domain.OrderRepository
	categoryRepo  domain.CategoryRepository
	reviewRepo    domain.ReviewRepository
	complaintRepo domain.ComplaintRepository
	kafkaProd     *kafka.Producer
	wsHub         *wsinfra.Hub

	createOrderHandler      *usecases.CreateOrderHandler
	getOrderInternalHandler *usecases.GetOrderInternalHandler
	listOrdersHandler       *usecases.ListOrdersHandler
	updateStatusHandler     *usecases.UpdateStatusHandler
	completeOrderHandler    *usecases.CompleteOrderHandler
	cancelOrderHandler      *usecases.CancelOrderHandler
	createReviewHandler     *usecases.CreateReviewHandler
	listReviewsHandler      *usecases.ListReviewsHandler
	manageCategories        *usecases.ManageCategoriesHandler
	createComplaintHandler  *usecases.CreateComplaintHandler
	manageComplaints        *usecases.ManageComplaintsHandler
}

func NewOrderService(
	orderRepo domain.OrderRepository,
	categoryRepo domain.CategoryRepository,
	reviewRepo domain.ReviewRepository,
	complaintRepo domain.ComplaintRepository,
	kafkaProd *kafka.Producer,
	wsHub *wsinfra.Hub,
) domain.OrderService {
	return &orderService{
		orderRepo:               orderRepo,
		categoryRepo:            categoryRepo,
		reviewRepo:              reviewRepo,
		complaintRepo:           complaintRepo,
		kafkaProd:               kafkaProd,
		wsHub:                   wsHub,
		createOrderHandler:      usecases.NewCreateOrderHandler(orderRepo, kafkaProd),
		getOrderInternalHandler: usecases.NewGetOrderInternalHandler(orderRepo),
		listOrdersHandler:       usecases.NewListOrdersHandler(orderRepo),
		updateStatusHandler:     usecases.NewUpdateStatusHandler(orderRepo, kafkaProd),
		completeOrderHandler:    usecases.NewCompleteOrderHandler(orderRepo, kafkaProd),
		cancelOrderHandler:      usecases.NewCancelOrderHandler(orderRepo, kafkaProd),
		createReviewHandler:     usecases.NewCreateReviewHandler(reviewRepo, kafkaProd),
		listReviewsHandler:      usecases.NewListReviewsHandler(reviewRepo),
		manageCategories:        usecases.NewManageCategoriesHandler(categoryRepo),
		createComplaintHandler:  usecases.NewCreateComplaintHandler(complaintRepo, kafkaProd),
		manageComplaints:        usecases.NewManageComplaintsHandler(complaintRepo),
	}
}

func (s *orderService) broadcastOrder(eventType string, order *domain.Order) {
	if s.wsHub == nil {
		return
	}
	data, err := json.Marshal(order)
	if err != nil {
		return
	}
	s.wsHub.Broadcast(eventType, data)
}

func (s *orderService) CreateOrder(ctx context.Context, input domain.CreateOrderInput) (*domain.Order, error) {
	order, err := s.createOrderHandler.Handle(ctx, input)
	if err != nil {
		return nil, err
	}
	s.broadcastOrder("order.created", order)
	return order, nil
}

func (s *orderService) GetOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	return s.getOrderInternalHandler.Handle(ctx, orderID)
}

func (s *orderService) ListOrders(ctx context.Context, filter domain.OrderFilter, limit, offset int) ([]*domain.Order, int, error) {
	result, err := s.listOrdersHandler.Handle(ctx, usecases.ListOrdersQuery{
		CustomerID: filter.CustomerID,
		Status:     filter.Status,
		CategoryID: filter.CategoryID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return result.Orders, result.Total, nil
}

func (s *orderService) ListCustomerOrders(ctx context.Context, customerID uuid.UUID, limit, offset int) ([]*domain.Order, int, error) {
	result, err := s.listOrdersHandler.Handle(ctx, usecases.ListOrdersQuery{
		CustomerID: &customerID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return result.Orders, result.Total, nil
}

func (s *orderService) GetStatusHistory(ctx context.Context, orderID uuid.UUID) ([]*domain.OrderStatusHistory, error) {
	return s.orderRepo.GetStatusHistory(ctx, orderID)
}

func (s *orderService) UpdateStatus(ctx context.Context, input domain.UpdateStatusInput) (*domain.Order, error) {
	order, err := s.updateStatusHandler.Handle(ctx, input)
	if err != nil {
		return nil, err
	}
	s.broadcastOrder("order.updated", order)
	return order, nil
}

func (s *orderService) AssignOrder(ctx context.Context, orderID, offerID uuid.UUID) (*domain.Order, error) {
	ord, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, domain.ErrOrderNotFound
	}
	if ord.Status != domain.StatusCreated {
		return nil, domain.ErrInvalidTransition
	}

	oldStatus := ord.Status
	ord.Status = domain.StatusAssigned
	ord.AcceptedOfferID = &offerID
	if err := s.orderRepo.Update(ctx, ord); err != nil {
		return nil, err
	}

	// Record status history.
	_ = s.orderRepo.AddStatusHistory(ctx, &domain.OrderStatusHistory{
		OrderID:   orderID,
		OldStatus: &oldStatus,
		NewStatus: domain.StatusAssigned,
		ChangedBy: uuid.Nil, // system action
	})

	// Publish Kafka event.
	if s.kafkaProd != nil {
		_ = s.kafkaProd.PublishOrderAssigned(ctx, kafka.OrderAssignedEvent{
			OrderID:    orderID.String(),
			MasterID:   "",
			CustomerID: ord.CustomerID.String(),
		})
	}

	s.broadcastOrder("order.updated", ord)

	return ord, nil
}

func (s *orderService) CompleteOrder(ctx context.Context, input domain.CompleteOrderInput) (*domain.Order, error) {
	order, err := s.completeOrderHandler.Handle(ctx, input)
	if err != nil {
		return nil, err
	}
	s.broadcastOrder("order.updated", order)
	return order, nil
}

func (s *orderService) CancelOrder(ctx context.Context, input domain.CancelOrderInput) (*domain.Order, error) {
	order, err := s.cancelOrderHandler.Handle(ctx, input)
	if err != nil {
		return nil, err
	}
	s.broadcastOrder("order.updated", order)
	return order, nil
}

func (s *orderService) CreateCategory(ctx context.Context, input domain.CreateCategoryInput) (*domain.Category, error) {
	return s.manageCategories.Create(ctx, input)
}

func (s *orderService) UpdateCategory(ctx context.Context, input domain.UpdateCategoryInput) (*domain.Category, error) {
	return s.manageCategories.Update(ctx, input)
}

func (s *orderService) DeleteCategory(ctx context.Context, categoryID uuid.UUID) error {
	return s.manageCategories.Delete(ctx, categoryID)
}

func (s *orderService) ListCategories(ctx context.Context, treeView bool) (interface{}, error) {
	return s.manageCategories.List(ctx, treeView)
}

func (s *orderService) CreateReview(ctx context.Context, input domain.CreateReviewInput) (*domain.Review, error) {
	return s.createReviewHandler.Handle(ctx, input)
}

func (s *orderService) ListReviewsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Review, int, error) {
	result, err := s.listReviewsHandler.Handle(ctx, usecases.ListReviewsQuery{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return result.Reviews, result.Total, nil
}

func (s *orderService) CreateComplaint(ctx context.Context, input domain.CreateComplaintInput) (*domain.Complaint, error) {
	return s.createComplaintHandler.Handle(ctx, input)
}

func (s *orderService) ListComplaints(ctx context.Context, status *domain.ComplaintStatus, limit, offset int) ([]*domain.Complaint, int, error) {
	result, err := s.manageComplaints.List(ctx, usecases.ListComplaintsQuery{
		Status: status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return result.Complaints, result.Total, nil
}

func (s *orderService) UpdateComplaint(ctx context.Context, input domain.UpdateComplaintInput) (*domain.Complaint, error) {
	return s.manageComplaints.Update(ctx, input)
}
