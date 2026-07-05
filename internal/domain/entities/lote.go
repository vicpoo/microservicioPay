package entities

import (
	"errors"
	"time"
)

var (
	ErrLoteNoEncontrado  = errors.New("el lote no existe o no está finalizado")
	ErrLoteNoDisponible  = errors.New("el lote no está disponible para venta")
	ErrOrdenYaProcesada  = errors.New("la orden ya fue procesada")
	ErrFirmaWebhookInvalida = errors.New("firma de webhook inválida")
)

// LoteVendible es la porción del agregado Osil (dueño del BC Gestión de
// Osiles) que el BC de Ventas necesita para decidir si puede vender.
// El BC de Ventas es "Conformist" respecto al modelo de Osil (SAD/Dominio,
// Sección 5.1): solo lee, nunca modifica su estructura.
type LoteVendible struct {
	IDLote      int
	NombreLote  string
	Precio      float64
	Disponible  bool
}

// EventoOsilVendido es el evento de dominio que el BC de Ventas publica
// hacia el bus de eventos para que Gestión de Osiles reaccione
// (Published Language, ADR-001 del SAD).
type EventoOsilVendido struct {
	IDLote      int
	IDOrden     int
	Comprador   string
	PrecioTotal float64
	FechaPago   time.Time
}
