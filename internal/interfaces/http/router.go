package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	domain "github.com/companyofcreators/order-service/internal/domain/order"
	"github.com/companyofcreators/order-service/pkg/header_auth"
)

// WebSocketUpgrader is the function type for upgrading HTTP connections to WebSocket.
type WebSocketUpgrader func(w http.ResponseWriter, r *http.Request)

func NewRouter(service domain.OrderService, signer *header_auth.HeaderSigner, wsUpgrader WebSocketUpgrader) chi.Router {
	r := chi.NewRouter()

	// Global middleware (applies to all routes including WebSocket)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(bodySizeLimiter(500 << 10)) // 500KB
	r.Use(middleware.Heartbeat("/ping"))

	// WebSocket endpoint — JWT validation is done inside the handler,
	// no HMAC header verification needed.
	r.Get("/ws", http.HandlerFunc(wsUpgrader))

	// Internal routes — protected by HMAC header verification.
	r.Route("/internal", func(r chi.Router) {
		r.Use(signer.VerifyMiddleware)

		h := NewHandler(service)

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

// bodySizeLimiter returns middleware that wraps http.MaxBytesReader to limit
// request body size and prevent memory exhaustion attacks.
func bodySizeLimiter(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
