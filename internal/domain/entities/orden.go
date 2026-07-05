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
type Comprador struct {
	ID               int
	Nombre           string
	Email            string
	Telefono         string
	Pais             string
	StripeCustomerID string
	CreatedAt        time.Time
}

// Orden representa la compra de UN osil completo.
type Orden struct {
	ID                      int
	IDLote                  int
	IDComprador             int
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
