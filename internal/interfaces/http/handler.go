package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

// RoleChecker checks if a user has a specific role by querying the user-service database.
type RoleChecker interface {
	HasRole(ctx context.Context, userID, role string) (bool, error)
}

type Handler struct {
	service     domain.OrderService
	roleChecker RoleChecker
}

func NewHandler(service domain.OrderService, roleChecker RoleChecker) *Handler {
	return &Handler{service: service, roleChecker: roleChecker}
}

// HealthCheck handles GET /internal/health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// CreateOrder handles POST /internal/orders
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	if req.Latitude == 0 && req.Longitude == 0 {
		respondError(w, http.StatusBadRequest, "latitude и longitude обязательны", "укажите координаты заказа")
		return
	}

	input := domain.CreateOrderInput{
		CustomerID:  userID,
		CategoryID:  req.CategoryID,
		Price:       req.Price,
		Currency:    req.Currency,
		Address:     req.Address,
		Title:       req.Title,
		Description: req.Description,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
	}

	ord, err := h.service.CreateOrder(r.Context(), input)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "create order failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "failed to create order", err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, CreateOrderResponse{Order: ord})
}

// GetOrder handles GET /internal/orders/{id}
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
	}

	ord, err := h.service.GetOrder(r.Context(), orderID)
	if err != nil {
		if err == domain.ErrOrderNotFound {
			respondError(w, http.StatusNotFound, "заказ не найден", err.Error())
			return
		}
		pkg.Logger.ErrorContext(r.Context(), "get order failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить заказ", err.Error())
		return
	}

	// Customer can only view their own orders; admin/moderator can view all

	// Access: customer, assigned master, marketplace view (master), staff
	isOwner := ord.CustomerID == userID
	isAssignedMaster := ord.AssignedMasterID != nil && *ord.AssignedMasterID == userID
	isMarketplace := ord.Status == domain.StatusCreated && ord.AssignedMasterID == nil && ord.CustomerID != userID
	if isMarketplace {
		isMaster, _ := h.roleChecker.HasRole(r.Context(), userID.String(), "master")
		if !isMaster || r.URL.Query().Get("active_role") != "master" {
			isMarketplace = false
		}
	}
	if !isOwner && !isAssignedMaster && !isMarketplace {
		isStaff, _ := h.roleChecker.HasRole(r.Context(), userID.String(), "admin")
		if !isStaff {
			isStaff, _ = h.roleChecker.HasRole(r.Context(), userID.String(), "moderator")
		}
		if !isStaff {
			respondError(w, http.StatusForbidden, "доступ запрещён", "вы можете просматривать только свои заказы")
			return
		}
	}
	respondJSON(w, http.StatusOK, GetOrderResponse{Order: ord})
}

// ListOrders handles GET /internal/orders
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	// Detect internal service-to-service query (user_id query param)
	userIDParam := r.URL.Query().Get("user_id")
	isInternalQuery := userIDParam != ""

	var userID uuid.UUID
	var err error

	if !isInternalQuery {
		userID, err = getUserID(r)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
			return
		}
	}

	limit := queryParamInt(r, "limit", 20)
	offset := queryParamInt(r, "offset", 0)

	filter := domain.OrderFilter{}

	if isInternalQuery {
		// Internal query: filter by the provided user_id
		if uid, parseErr := uuid.Parse(userIDParam); parseErr == nil {
			filter.CustomerID = &uid
		}
	} else {
		// Check roles from DB (not stale JWT). Customers see their own orders,
		// masters see the active marketplace feed, staff can see all orders.
		isStaff := false
		isMaster := false
		if h.roleChecker != nil {
			var checkErr error
			isStaff, checkErr = h.roleChecker.HasRole(r.Context(), userID.String(), "admin")
			if checkErr != nil {
				isStaff = false
			}
			if !isStaff {
				isStaff, checkErr = h.roleChecker.HasRole(r.Context(), userID.String(), "moderator")
				if checkErr != nil {
					isStaff = false
				}
			}
			isMaster, checkErr = h.roleChecker.HasRole(r.Context(), userID.String(), "master")
			if checkErr != nil {
				isMaster = false
			}
		}
		if !isStaff && !isMaster {
			filter.CustomerID = &userID
		} else if isMaster && !isStaff {
			filter.ActiveOnly = true
		} else {
			// Admin/moderator can optionally filter
			if custIDStr := r.URL.Query().Get("customer_id"); custIDStr != "" {
				custID, err := uuid.Parse(custIDStr)
				if err == nil {
					filter.CustomerID = &custID
				}
			}
		}
	}

	// Handle active=true query param — filters to active (non-terminal) statuses
	if r.URL.Query().Get("active") == "true" {
		filter.ActiveOnly = true
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.OrderStatus(statusStr)
		filter.Status = &status
	}

	if catIDStr := r.URL.Query().Get("category_id"); catIDStr != "" {
		catID, err := uuid.Parse(catIDStr)
		if err == nil {
			filter.CategoryID = &catID
		}
	}

	orders, total, err := h.service.ListOrders(r.Context(), filter, limit, offset)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "list orders failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить список заказов", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, ListOrdersResponse{
		Orders: orders,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// UpdateStatus handles PATCH /internal/orders/{id}/status
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	isStaffOrMaster, err := h.hasAnyRole(r.Context(), userID.String(), "admin", "moderator", "master", "user")
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "failed to check role for status update", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось проверить роль пользователя", err.Error())
		return
	}
	if !isStaffOrMaster {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы, модераторы или мастера могут обновлять статус")
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	// When transitioning to "cancelled", enforce the same ownership check as CancelOrder.
	// Only the order's customer or an admin/moderator may cancel the order.
	if req.Status == string(domain.StatusCancelled) {
		ord, err := h.service.GetOrder(r.Context(), orderID)
		if err != nil {
			if err == domain.ErrOrderNotFound {
				respondError(w, http.StatusNotFound, "заказ не найден", err.Error())
			} else {
				pkg.Logger.ErrorContext(r.Context(), "get order failed", "error", err.Error())
				respondError(w, http.StatusInternalServerError, "не удалось получить заказ", err.Error())
			}
			return
		}
		isCustomer := ord.CustomerID == userID
		isModeratorOrAdmin, _ := h.hasAnyRole(r.Context(), userID.String(), "admin", "moderator")
		if !isCustomer && !isModeratorOrAdmin {
			respondError(w, http.StatusForbidden, "доступ запрещён", "только заказчик, админ или модератор может отменить этот заказ")
			return
		}
	}

	input := domain.UpdateStatusInput{
		OrderID:   orderID,
		NewStatus: domain.OrderStatus(req.Status),
		ChangedBy: userID,
	}

	ord, err := h.service.UpdateStatus(r.Context(), input)
	if err != nil {
		switch err {
		case domain.ErrOrderNotFound:
			respondError(w, http.StatusNotFound, "заказ не найден", err.Error())
		case domain.ErrInvalidTransition:
			respondError(w, http.StatusConflict, "недопустимый переход статуса", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "update status failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to update status", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, UpdateStatusResponse{Order: ord})
}

// AssignOrder handles POST /internal/orders/{id}/assign — called by offer-service.
func (h *Handler) AssignOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
	}

	var req struct {
		OfferID string `json:"offer_id"`
		MasterID string `json:"master_id"`
		FinalPrice float64 `json:"final_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	offerID, err := uuid.Parse(req.OfferID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID предложения", err.Error())
		return
	}

	masterID, err := uuid.Parse(req.MasterID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID мастера", err.Error())
		return
	}

	ord, err := h.service.AssignOrder(r.Context(), orderID, offerID, masterID, req.FinalPrice)
	if err != nil {
		switch err {
		case domain.ErrOrderNotFound:
			respondError(w, http.StatusNotFound, "заказ не найден", err.Error())
		case domain.ErrInvalidTransition:
			respondError(w, http.StatusConflict, "недопустимый переход статуса", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "assign order failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "не удалось назначить заказ", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"order": ord})
}

// CancelOrder handles POST /internal/orders/{id}/cancel
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
	}

	// Resolve actual role from DB (not stale JWT header)
	userRole := ""
	if h.roleChecker != nil {
		isAdmin, _ := h.roleChecker.HasRole(r.Context(), userID.String(), "admin")
		if isAdmin {
			userRole = "admin"
		} else {
			isModerator, _ := h.roleChecker.HasRole(r.Context(), userID.String(), "moderator")
			if isModerator {
				userRole = "moderator"
			}
		}
	}

	input := domain.CancelOrderInput{
		OrderID:  orderID,
		UserID:   userID,
		UserRole: userRole,
	}

	ord, err := h.service.CancelOrder(r.Context(), input)
	if err != nil {
		switch err {
		case domain.ErrOrderNotFound:
			respondError(w, http.StatusNotFound, "заказ не найден", err.Error())
		case domain.ErrOrderNotCancellable:
			respondError(w, http.StatusConflict, "заказ не может быть отменён", err.Error())
		case domain.ErrForbidden:
			respondError(w, http.StatusForbidden, "доступ запрещён", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "cancel order failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to cancel order", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, CancelOrderResponse{Order: ord})
}

// CompleteOrder handles POST /internal/orders/{id}/complete
func (h *Handler) CompleteOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
	}

	input := domain.CompleteOrderInput{
		OrderID:    orderID,
		CustomerID: userID,
	}

	ord, err := h.service.CompleteOrder(r.Context(), input)
	if err != nil {
		switch err {
		case domain.ErrOrderNotFound:
			respondError(w, http.StatusNotFound, "заказ не найден", err.Error())
		case domain.ErrOrderNotCompletable:
			respondError(w, http.StatusConflict, "заказ не может быть завершён", err.Error())
		case domain.ErrForbidden:
			respondError(w, http.StatusForbidden, "доступ запрещён", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "complete order failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to complete order", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, CompleteOrderResponse{Order: ord})
}

// GetStatusHistory handles GET /internal/orders/{id}/history
func (h *Handler) GetStatusHistory(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
	}

	history, err := h.service.GetStatusHistory(r.Context(), orderID)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "get status history failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить историю статусов", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, StatusHistoryResponse{History: history})
}

// ListCategories handles GET /internal/categories
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	treeView := r.URL.Query().Get("tree") == "true"

	categories, err := h.service.ListCategories(r.Context(), treeView)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "list categories failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить список категорий", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, CategoriesResponse{Categories: categories})
}

// CreateCategory handles POST /internal/categories
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}
	isAdmin, err := h.roleChecker.HasRole(r.Context(), userID.String(), "admin")
	if err != nil || !isAdmin {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы могут создавать категории")
		return
	}

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	input := domain.CreateCategoryInput{
		Name:     req.Name,
		ParentID: req.ParentID,
		Slug:     req.Slug,
	}

	cat, err := h.service.CreateCategory(r.Context(), input)
	if err != nil {
		if err == domain.ErrCategorySlugExists {
			respondError(w, http.StatusConflict, "категория уже существует", err.Error())
			return
		}
		pkg.Logger.ErrorContext(r.Context(), "create category failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось создать категорию", err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, CategoryResponse{Category: cat})
}

// UpdateCategory handles PATCH /internal/categories/{id}
func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}
	isAdmin, err := h.roleChecker.HasRole(r.Context(), userID.String(), "admin")
	if err != nil || !isAdmin {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы могут обновлять категории")
		return
	}

	categoryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid category id", err.Error())
		return
	}

	var req UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	input := domain.UpdateCategoryInput{
		ID:       categoryID,
		Name:     req.Name,
		ParentID: req.ParentID,
		Slug:     req.Slug,
	}

	cat, err := h.service.UpdateCategory(r.Context(), input)
	if err != nil {
		switch err {
		case domain.ErrCategoryNotFound:
			respondError(w, http.StatusNotFound, "категория не найдена", err.Error())
		case domain.ErrCategorySlugExists:
			respondError(w, http.StatusConflict, "slug уже существует", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "update category failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to update category", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, CategoryResponse{Category: cat})
}

// DeleteCategory handles DELETE /internal/categories/{id}
func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}
	isAdmin, err := h.roleChecker.HasRole(r.Context(), userID.String(), "admin")
	if err != nil || !isAdmin {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы могут удалять категории")
		return
	}

	categoryID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid category id", err.Error())
		return
	}

	if err := h.service.DeleteCategory(r.Context(), categoryID); err != nil {
		switch err {
		case domain.ErrCategoryNotFound:
			respondError(w, http.StatusNotFound, "категория не найдена", err.Error())
		case domain.ErrCategoryHasChildren:
			respondError(w, http.StatusConflict, "нельзя удалить категорию с подкатегориями", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "delete category failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to delete category", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateReview handles POST /internal/reviews
func (h *Handler) CreateReview(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	var req CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	input := domain.CreateReviewInput{
		OrderID:    req.OrderID,
		FromUserID: userID,
		ToUserID:   req.ToUserID,
		Rating:     req.Rating,
		Comment:    req.Comment,
	}

	review, err := h.service.CreateReview(r.Context(), input)
	if err != nil {
		switch {
		case err == domain.ErrInvalidRating:
			respondError(w, http.StatusBadRequest, "недопустимая оценка", err.Error())
		case errors.Is(err, domain.ErrOrderNotCompleted):
			respondError(w, http.StatusConflict, "невозможно оставить отзыв", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "create review failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to create review", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusCreated, CreateReviewResponse{Review: review})
}

// ListReviews handles GET /internal/reviews/user/{id}
func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID пользователя", err.Error())
		return
	}

	limit := queryParamInt(r, "limit", 20)
	offset := queryParamInt(r, "offset", 0)

	byMe := r.URL.Query().Get("from") == "true"
	role := r.URL.Query().Get("role")
	reviews, total, avgRating, err := h.service.ListReviewsByUser(r.Context(), userID, byMe, role, limit, offset)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "list reviews failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить список отзывов", err.Error())
		return
	}

	enrichReviewsWithProfiles(r.Context(), reviews)

	respondJSON(w, http.StatusOK, ListReviewsResponse{
		Reviews:    reviews,
		Total:      total,
		AvgRating:  avgRating,
		Limit:      limit,
		Offset:     offset,
	})
}

// CreateComplaint handles POST /internal/complaints
func (h *Handler) CreateComplaint(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}

	var req CreateComplaintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	input := domain.CreateComplaintInput{
		ReporterUserID: userID,
		TargetUserID:   req.TargetUserID,
		OrderID:        req.OrderID,
		Message:        req.Message,
	}

	complaint, err := h.service.CreateComplaint(r.Context(), input)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "create complaint failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось создать жалобу", err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, CreateComplaintResponse{Complaint: complaint})
}

// ListComplaints handles GET /internal/complaints
func (h *Handler) ListComplaints(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}
	isStaff, err := h.hasAnyRole(r.Context(), userID.String(), "admin", "moderator")
	if err != nil || !isStaff {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы и модераторы могут просматривать жалобы")
		return
	}

	limit := queryParamInt(r, "limit", 20)
	offset := queryParamInt(r, "offset", 0)

	var status *domain.ComplaintStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		s := domain.ComplaintStatus(statusStr)
		status = &s
	}

	complaints, total, err := h.service.ListComplaints(r.Context(), status, limit, offset)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "list complaints failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить список жалоб", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, ListComplaintsResponse{
		Complaints: complaints,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	})
}

// UpdateComplaint handles PATCH /internal/complaints/{id}
func (h *Handler) UpdateComplaint(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
		return
	}
	isStaff, err := h.hasAnyRole(r.Context(), userID.String(), "admin", "moderator")
	if err != nil || !isStaff {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы и модераторы могут обновлять жалобы")
		return
	}

	complaintID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID жалобы", err.Error())
		return
	}

	var req UpdateComplaintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "некорректное тело запроса", err.Error())
		return
	}

	input := domain.UpdateComplaintInput{
		ComplaintID: complaintID,
		Status:      domain.ComplaintStatus(req.Status),
		ModeratorID: userID,
	}

	complaint, err := h.service.UpdateComplaint(r.Context(), input)
	if err != nil {
		switch {
		case err == domain.ErrComplaintNotFound:
			respondError(w, http.StatusNotFound, "жалоба не найдена", err.Error())
		case err == domain.ErrInvalidComplaintStatus:
			respondError(w, http.StatusConflict, "недопустимый переход статуса", err.Error())
		default:
			pkg.Logger.ErrorContext(r.Context(), "update complaint failed", "error", err.Error())
			respondError(w, http.StatusInternalServerError, "failed to update complaint", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, CreateComplaintResponse{Complaint: complaint})
}

// hasAnyRole checks if the user has any of the specified roles (from DB, not JWT).
func (h *Handler) hasAnyRole(ctx context.Context, userID string, roles ...string) (bool, error) {
	if h.roleChecker == nil {
		return false, nil
	}
	for _, role := range roles {
		has, err := h.roleChecker.HasRole(ctx, userID, role)
		if err != nil {
			return false, err
		}
		if has {
			return true, nil
		}
	}
	return false, nil
}

// Helper functions

func getUserID(r *http.Request) (uuid.UUID, error) {
	userIDStr := r.Header.Get("X-User-Id")
	if userIDStr == "" {
		return uuid.Nil, domain.ErrUnauthorized
	}
	return uuid.Parse(userIDStr)
}

func queryParamInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil || i < 0 {
		return defaultVal
	}
	return i
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	body, err := json.Marshal(data)
	if err != nil {
		pkg.Logger.Error("failed to marshal response", "error", err.Error())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"внутренняя ошибка сервера"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func respondError(w http.ResponseWriter, status int, error string, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:   error,
		Message: message,
	})
}

func enrichReviewsWithProfiles(ctx context.Context, reviews []*domain.Review) {
	for _, rev := range reviews {
		url := "http://localhost:8082/internal/users/" + rev.FromUserID.String()
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		req.Header.Set("X-User-Id", "00000000-0000-0000-0000-000000000000")
		req.Header.Set("X-User-Email", "system@diploma")
		req.Header.Set("X-User-Role", "admin")
		// HMAC sign — use crypto/hmac inline
		payload := "00000000-0000-0000-0000-000000000000|system@diploma|admin"
		mac := hmac.New(sha256.New, []byte("diploma-internal-hmac-secret-key-2026"))
		mac.Write([]byte(payload))
		req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		resp, err := http.DefaultClient.Do(req)
		if err != nil { continue }
		defer resp.Body.Close()
		if resp.StatusCode != 200 { continue }
		var full struct {
			Profile *struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				AvatarURL string `json:"avatar_url"`
			} `json:"profile,omitempty"`
		}
		json.NewDecoder(resp.Body).Decode(&full)
		if full.Profile != nil {
			rev.FromName = full.Profile.FirstName + " " + full.Profile.LastName
			rev.FromAvatar = full.Profile.AvatarURL
		}
	}
}
