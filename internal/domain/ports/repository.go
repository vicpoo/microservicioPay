//internal/domain/ports/repository.go
package ports

import (
	"context"

	"github.com/kajve/payment-service/internal/domain/entities"
)

// OrderRepository es el puerto que el dominio/aplicación usa para
// persistir órdenes y compradores. La implementación concreta
// (Postgres, u otra) vive en infrastructure/repository.
type OrderRepository interface {
	// LoteVendible consulta el estado del osil en la tabla lotes_cafe.
	LoteVendible(ctx context.Context, idLote int) (entities.LoteVendible, error)

	CrearComprador(ctx context.Context, c *entities.Comprador) (int, error)

	CrearOrden(ctx context.Context, o *entities.Orden) (int, error)

	// ActualizarCheckoutSession guarda el session_id de Stripe una vez
	// creada la Checkout Session (paso separado de CrearOrden porque
	// el session_id solo existe después de llamar a Stripe).
	ActualizarCheckoutSession(ctx context.Context, idOrden int, sessionID string) error

	// MarcarOrdenPagada actualiza la orden a 'pagada', bloquea el lote
	// y registra el evento en historial_eventos, todo en una transacción.
	// Regresa el id_lote para poder publicar el evento de dominio.
	MarcarOrdenPagada(ctx context.Context, checkoutSessionID, paymentIntentID string) (idOrden int, idLote int, err error)

	// --- Idempotencia de webhooks ---
	EventoYaProcesado(ctx context.Context, stripeEventID string) (bool, error)
	RegistrarEventoWebhook(ctx context.Context, stripeEventID, tipoEvento string, payload []byte) error
	MarcarEventoProcesado(ctx context.Context, stripeEventID string) error
}
