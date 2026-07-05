package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

// ProcesarWebhookUseCase verifica la firma, aplica idempotencia por
// event.ID, marca la orden como pagada y publica el evento de dominio
// osil.vendido. No sabe que el transporte es HTTP ni que la pasarela
// es Stripe — solo conoce los puertos.
type ProcesarWebhookUseCase struct {
	repo      ports.OrderRepository
	gateway   ports.PaymentGateway
	publisher ports.EventPublisher
}

func NewProcesarWebhookUseCase(repo ports.OrderRepository, gateway ports.PaymentGateway, publisher ports.EventPublisher) *ProcesarWebhookUseCase {
	return &ProcesarWebhookUseCase{repo: repo, gateway: gateway, publisher: publisher}
}

func (uc *ProcesarWebhookUseCase) Execute(ctx context.Context, payload []byte, firma string) error {
	evento, err := uc.gateway.VerificarYParsearWebhook(payload, firma)
	if err != nil {
		return entities.ErrFirmaWebhookInvalida
	}

	yaProcesado, _ := uc.repo.EventoYaProcesado(ctx, evento.EventID)
	if yaProcesado {
		return nil // idempotencia: ya lo procesamos, no hacer nada
	}
	if err := uc.repo.RegistrarEventoWebhook(ctx, evento.EventID, evento.EventType, payload); err != nil {
		return fmt.Errorf("error registrando evento de webhook: %w", err)
	}

	if evento.EsCheckoutCompletado {
		idOrden, idLote, err := uc.repo.MarcarOrdenPagada(ctx, evento.CheckoutSessionID, evento.PaymentIntentID)
		if err != nil {
			return fmt.Errorf("error marcando orden pagada: %w", err)
		}

		if uc.publisher != nil {
			_ = uc.publisher.PublicarOsilVendido(ctx, entities.EventoOsilVendido{
				IDLote:      idLote,
				IDOrden:     idOrden,
				Comprador:   evento.CompradorEmail,
				PrecioTotal: evento.MontoTotal,
				FechaPago:   time.Now(),
			})
			// Nota: si la publicación falla, el pago YA quedó
			// registrado en BD (fuente de verdad). Falta un mecanismo
			// de reintento/outbox para no perder el evento — ver README.
		}
	}

	return uc.repo.MarcarEventoProcesado(ctx, evento.EventID)
}
