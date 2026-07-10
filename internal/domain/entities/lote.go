//internal/domain/entities/lote.go
package entities

import (
	"errors"
	"time"
)

var (
	ErrLoteNoEncontrado     = errors.New("el lote no existe o no está finalizado")
	ErrLoteNoDisponible     = errors.New("el lote no está disponible para venta")
	ErrOrdenYaProcesada     = errors.New("la orden ya fue procesada")
	ErrFirmaWebhookInvalida = errors.New("firma de webhook inválida")
	ErrOrdenNoEncontrada    = errors.New("la orden no existe")
	ErrEstadoInvalido       = errors.New("estado de orden inválido")

	// --- Nuevos: catálogo y suscripciones ---
	ErrProductoNoEncontrado  = errors.New("el producto no existe")
	ErrProductoNoDisponible  = errors.New("el producto no está disponible (inactivo o sin stock)")
	ErrProductoTipoInvalido  = errors.New("tipo de producto inválido")
	ErrUsuarioRequerido      = errors.New("id_usuario es requerido para comprar una suscripción")
	ErrDatosProductoInvalidos = errors.New("faltan datos requeridos para este tipo de producto")
)

// EventoOsilVendido es el evento de dominio que el BC de Ventas publica
// hacia el bus de eventos para que Gestión de Osiles reaccione
// (Published Language, ADR-001 del SAD). Solo aplica a ventas de tipo
// cama_cafe; las suscripciones no generan este evento.
type EventoOsilVendido struct {
	IDLote      int
	IDOrden     int
	Comprador   string
	PrecioTotal float64
	FechaPago   time.Time
}