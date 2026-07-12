//internal/domain/ports/repository.go
package ports

import (
	"context"

	"github.com/kajve/payment-service/internal/domain/entities"
)

// OrderRepository es el puerto que el dominio/aplicación usa para
// persistir órdenes, compradores, catálogo y el estado premium de los
// usuarios. La implementación concreta (Postgres, u otra) vive en
// infrastructure/repository.
type OrderRepository interface {
	// --- Catálogo ---

	// ProductoVendible consulta el estado de un producto del catálogo
	// (cama_cafe o suscripcion) para decidir si se puede vender ahora.
	ProductoVendible(ctx context.Context, idProducto int) (entities.Producto, error)

	CrearProducto(ctx context.Context, p *entities.Producto) (int, error)
	ObtenerProducto(ctx context.Context, idProducto int) (entities.Producto, error)
	ListarProductos(ctx context.Context, filtro FiltroCatalogo) ([]entities.Producto, error)
	ActualizarProducto(ctx context.Context, p *entities.Producto) error
	EliminarProducto(ctx context.Context, idProducto int) error

	// --- Compradores / Órdenes ---

	CrearComprador(ctx context.Context, c *entities.Comprador) (int, error)

	CrearOrden(ctx context.Context, o *entities.Orden) (int, error)

	// ActualizarCheckoutSession guarda el session_id de Stripe una vez
	// creada la Checkout Session (paso separado de CrearOrden porque
	// el session_id solo existe después de llamar a Stripe).
	ActualizarCheckoutSession(ctx context.Context, idOrden int, sessionID string) error

	// MarcarOrdenPagada actualiza la orden a 'pagada' y, según el
	// tipo_orden:
	//   - cama_cafe: descuenta stock del producto (y bloquea el lote
	//     asociado si lo tiene, para trazabilidad).
	//   - suscripcion: activa el campo es_premium del usuario y le fija
	//     premium_hasta = NOW() + duracion_dias del plan comprado.
	// Todo en una sola transacción. Regresa el id_lote (si aplica, para
	// publicar el evento de dominio osil.vendido) o 0 si no hay lote.
	MarcarOrdenPagada(ctx context.Context, checkoutSessionID, paymentIntentID string) (idOrden int, idLote int, err error)

	// --- Idempotencia de webhooks ---
	EventoYaProcesado(ctx context.Context, stripeEventID string) (bool, error)
	RegistrarEventoWebhook(ctx context.Context, stripeEventID, tipoEvento string, payload []byte) error
	MarcarEventoProcesado(ctx context.Context, stripeEventID string) error

	ListarOrdenes(ctx context.Context, filtro FiltroOrdenes) ([]entities.OrdenConComprador, error)
	ObtenerOrdenPorID(ctx context.Context, idOrden int) (entities.OrdenConComprador, error)
	ActualizarEstadoOrden(ctx context.Context, idOrden int, nuevoEstado entities.EstadoOrden) error

	// --- Premium (usuarios) ---

	// ActivarPremiumUsuario prende usuarios.es_premium y fija
	// premium_hasta = NOW() + duracionDias. Cada activación resetea el
	// plazo desde ahora (no se acumula con uno previo vigente).
	ActivarPremiumUsuario(ctx context.Context, idUsuario int, duracionDias int) error

	// ExpirarPremiumsVencidos apaga es_premium para todos los usuarios
	// cuyo premium_hasta ya pasó. Se llama periódicamente desde main.go.
	ExpirarPremiumsVencidos(ctx context.Context) (int, error)

	// EsPremium se auto-corrige (si ya venció, apaga el boolean antes de
	// leerlo) y regresa el estado premium actual del usuario.
	EsPremium(ctx context.Context, idUsuario int) (bool, error)
}

type FiltroOrdenes struct {
	Estado string
	IDLote int
	Limit  int
	Offset int
}

type FiltroCatalogo struct {
	TipoProducto string // "" = todos, "cama_cafe" o "suscripcion"
	SoloActivos  bool
	Limit        int
	Offset       int
}