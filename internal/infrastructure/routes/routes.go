package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kajve/payment-service/internal/infrastructure/controllers"
)

// NewRouter arma el árbol de rutas HTTP. Es infraestructura pura: solo
// conecta URLs con controllers, no contiene lógica de negocio.
func NewRouter(orders *controllers.OrdersController, webhooks *controllers.WebhooksController) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Endpoint público (sin JWT): el comprador no tiene cuenta en kajve.
	r.Post("/orders", orders.CrearOrden)

	// El webhook no debe pasar por middlewares que consuman/alteren el
	// body antes de que el gateway verifique la firma de Stripe.
	r.Post("/webhooks/stripe", webhooks.StripeWebhook)

	return r
}
