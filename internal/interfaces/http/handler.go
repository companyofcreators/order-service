package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/internal/pkg"
)

type Handler struct {
	service domain.OrderService
}

func NewHandler(service domain.OrderService) *Handler {
	return &Handler{service: service}
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

	userRole := r.Header.Get("X-User-Role")

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
	if userRole != "admin" && userRole != "moderator" && ord.CustomerID != userID {
		respondError(w, http.StatusForbidden, "доступ запрещён", "вы можете просматривать только свои заказы")
		return
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

	userRole := r.Header.Get("X-User-Role")

	limit := queryParamInt(r, "limit", 20)
	offset := queryParamInt(r, "offset", 0)

	filter := domain.OrderFilter{}

	if isInternalQuery {
		// Internal query: filter by the provided user_id
		if uid, parseErr := uuid.Parse(userIDParam); parseErr == nil {
			filter.CustomerID = &uid
		}
	} else {
		// For non-admin users, filter by their customer_id
		if userRole != "admin" && userRole != "moderator" {
			filter.CustomerID = &userID
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

	userRole := r.Header.Get("X-User-Role")
	if userRole != "admin" && userRole != "moderator" && userRole != "master" {
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
		isModeratorOrAdmin := userRole == "admin" || userRole == "moderator"
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

	ord, err := h.service.AssignOrder(r.Context(), orderID, offerID)
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

	userRole := r.Header.Get("X-User-Role")

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "недействительный ID заказа", err.Error())
		return
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
	userRole := r.Header.Get("X-User-Role")
	if userRole != "admin" {
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
	userRole := r.Header.Get("X-User-Role")
	if userRole != "admin" {
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
	userRole := r.Header.Get("X-User-Role")
	if userRole != "admin" {
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

	reviews, total, err := h.service.ListReviewsByUser(r.Context(), userID, limit, offset)
	if err != nil {
		pkg.Logger.ErrorContext(r.Context(), "list reviews failed", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "не удалось получить список отзывов", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, ListReviewsResponse{
		Reviews: reviews,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
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
	userRole := r.Header.Get("X-User-Role")
	if userRole != "admin" && userRole != "moderator" {
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
	userRole := r.Header.Get("X-User-Role")
	if userRole != "admin" && userRole != "moderator" {
		respondError(w, http.StatusForbidden, "доступ запрещён", "только админы и модераторы могут обновлять жалобы")
		return
	}

	userID, err := getUserID(r)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "не авторизован", err.Error())
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
