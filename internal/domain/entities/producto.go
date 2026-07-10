// internal/domain/entities/producto.go
package entities

import "time"

type TipoProducto string

const (
	TipoCamaCafe    TipoProducto = "cama_cafe"
	TipoSuscripcion TipoProducto = "suscripcion"
)

// Producto es un renglón del catálogo administrable. Puede ser una
// "cama de café" en venta o un plan de suscripción (ej. Premium).
// El admin lo da de alta/edita vía el CRUD; el comprador solo lo lee.
type Producto struct {
	ID              int
	TipoProducto    TipoProducto
	Nombre          string
	Descripcion     string
	Precio          float64
	Moneda          string
	ImagenURL       string
	Activo          bool

	// Específico de cama_cafe
	IDLote   *int // opcional: trazabilidad hacia el lote físico ya secado
	Variedad string
	PesoKg   float64
	Stock    int

	// Específico de suscripcion
	PlanSuscripcion string
	DuracionDias    int
	LotesMax        int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// EsVendible aplica la regla de negocio: ¿este producto se puede comprar
// ahora mismo? Para camas de café requiere stock disponible; para
// suscripciones basta con que esté activo.
func (p *Producto) EsVendible() bool {
	if !p.Activo {
		return false
	}
	if p.TipoProducto == TipoCamaCafe {
		return p.Stock > 0
	}
	return true
}