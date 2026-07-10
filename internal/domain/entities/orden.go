//internal/domain/entities/orden.go
package entities

import "time"

type EstadoOrden string

const (
	EstadoPendiente   EstadoOrden = "pendiente"
	EstadoPagada      EstadoOrden = "pagada"
	EstadoCancelada   EstadoOrden = "cancelada"
	EstadoReembolsada EstadoOrden = "reembolsada"
)

// Comprador es un guest checkout: NO tiene cuenta en kajve.
// (Cuando la orden es una suscripción, además del comprador se guarda
// el IDUsuario en la Orden para saber a quién activarle el plan.)
type Comprador struct {
	ID               int
	Nombre           string
	Email            string
	Telefono         string
	Pais             string
	StripeCustomerID string
	CreatedAt        time.Time
}

// Orden representa la compra de UN producto del catálogo: puede ser una
// cama de café o un plan de suscripción.
type Orden struct {
	ID                      int
	IDProducto              int
	TipoOrden               TipoProducto
	IDLote                  *int // opcional, heredado del producto (solo trazabilidad de cama_cafe)
	IDComprador             int
	IDUsuario               *int // requerido cuando TipoOrden == TipoSuscripcion
	PrecioTotal             float64
	Moneda                  string
	Estado                  EstadoOrden
	StripeCheckoutSessionID string
	StripePaymentIntentID   string
	FechaOrden              time.Time
	FechaPago               *time.Time
}

// EstaPagada es una regla de negocio simple que vive en la entidad,
// no en infraestructura ni en el caso de uso.
func (o *Orden) EstaPagada() bool {
	return o.Estado == EstadoPagada
}

type OrdenConComprador struct {
	Orden
	NombreComprador   string
	EmailComprador    string
	TelefonoComprador string
	PaisComprador     string
	NombreProducto    string
}