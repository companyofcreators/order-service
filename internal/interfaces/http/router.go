package http

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
)

func NewRouter(service domain.OrderService) chi.Router {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))

	h := NewHandler(service)

	r.Route("/internal", func(r chi.Router) {
		// Health
		r.Get("/health", h.HealthCheck)

		// Orders
		r.Route("/orders", func(r chi.Router) {
			r.Post("/", h.CreateOrder)
			r.Get("/", h.ListOrders)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.GetOrder)
				r.Patch("/status", h.UpdateStatus)
				r.Post("/assign", h.AssignOrder)
				r.Post("/cancel", h.CancelOrder)
				r.Post("/complete", h.CompleteOrder)
				r.Get("/history", h.GetStatusHistory)
			})
		})

		// Categories
		r.Route("/categories", func(r chi.Router) {
			r.Get("/", h.ListCategories)
			r.Post("/", h.CreateCategory)

			r.Route("/{id}", func(r chi.Router) {
				r.Patch("/", h.UpdateCategory)
				r.Delete("/", h.DeleteCategory)
			})
		})

		// Reviews
		r.Route("/reviews", func(r chi.Router) {
			r.Post("/", h.CreateReview)
			r.Get("/user/{id}", h.ListReviews)
		})

		// Complaints
		r.Route("/complaints", func(r chi.Router) {
			r.Post("/", h.CreateComplaint)
			r.Get("/", h.ListComplaints)

			r.Route("/{id}", func(r chi.Router) {
				r.Patch("/", h.UpdateComplaint)
			})
		})
	})

	return r
}
