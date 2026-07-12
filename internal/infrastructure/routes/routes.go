//internal/infrastructure/routes/routes.go
package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/kajve/payment-service/internal/infrastructure/controllers"
)

// NewRouter arma el árbol de rutas HTTP. Es infraestructura pura: solo
// conecta URLs con controllers, no contiene lógica de negocio.
//
// TODO (pendiente, ya existía antes): todo lo que cuelga de /admin debe
// ir detrás de un middleware de autenticación de administrador. Todavía
// no está implementado ni para órdenes ni para catálogo.
func NewRouter(
	orders *controllers.OrdersController,
	webhooks *controllers.WebhooksController,
	ordersAdmin *controllers.OrdersAdminController,
	catalogAdmin *controllers.CatalogAdminController,
	premium *controllers.PremiumController,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/orders", orders.CrearOrden)
	r.Post("/webhooks/stripe", webhooks.StripeWebhook)

	// Catálogo público (solo lectura), para que la app/tienda muestre lo
	// que hay disponible sin pasar por rutas de admin. Reusa los mismos
	// casos de uso de solo-lectura del CRUD administrativo.
	r.Get("/catalogo", catalogAdmin.ListarProductos)
	r.Get("/catalogo/{id}", catalogAdmin.ObtenerProducto)

	// Estado premium de un usuario (solo lectura). Pública por ahora,
	// igual que /catalogo — si más adelante quieres que cada quien solo
	// pueda ver su propio estado, esto debería ir detrás de auth de
	// usuario (no de admin) comparando el {id} contra el JWT.
	r.Get("/usuarios/{id}/premium", premium.VerificarPremium)

	r.Route("/admin", func(r chi.Router) {
		r.Get("/orders", ordersAdmin.ListarOrdenes)
		r.Get("/orders/{id}", ordersAdmin.ObtenerOrden)
		r.Patch("/orders/{id}/estado", ordersAdmin.ActualizarEstado)

		r.Route("/catalogo", func(r chi.Router) {
			r.Post("/", catalogAdmin.CrearProducto)
			r.Get("/", catalogAdmin.ListarProductos)
			r.Get("/{id}", catalogAdmin.ObtenerProducto)
			r.Put("/{id}", catalogAdmin.ActualizarProducto)
			r.Delete("/{id}", catalogAdmin.EliminarProducto)
		})
	})

	return r
}