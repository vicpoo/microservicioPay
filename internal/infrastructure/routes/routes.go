//internal/infrastructure/routes/routes.go
package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kajve/payment-service/internal/infrastructure/controllers"
)

// NewRouter arma el árbol de rutas HTTP. Es infraestructura pura: solo
// conecta URLs con controllers, no contiene lógica de negocio.
func NewRouter(orders *controllers.OrdersController, webhooks *controllers.WebhooksController, admin *controllers.OrdersAdminController) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/orders", orders.CrearOrden)
	r.Post("/webhooks/stripe", webhooks.StripeWebhook)

	r.Route("/admin", func(r chi.Router) {
		r.Get("/orders", admin.ListarOrdenes)
		r.Get("/orders/{id}", admin.ObtenerOrden)
		r.Patch("/orders/{id}/estado", admin.ActualizarEstado)
	})

	return r
}