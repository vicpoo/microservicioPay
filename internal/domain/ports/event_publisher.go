package ports

import (
	"context"

	"github.com/kajve/payment-service/internal/domain/entities"
)

// EventPublisher es el puerto hacia el bus de eventos (RabbitMQ, ADR-001
// del SAD). Permite que Gestión de Osiles se entere de una venta sin
// que este servicio conozca los detalles de ese otro BC.
type EventPublisher interface {
	PublicarOsilVendido(ctx context.Context, evento entities.EventoOsilVendido) error
}
