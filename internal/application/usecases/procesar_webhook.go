//application/usecases/procesar_webhook.go
package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

// ProcesarWebhookUseCase verifica la firma, aplica idempotencia por
// event.ID, marca la orden como pagada (lo que dispara stock/suscripción
// según corresponda dentro del repositorio) y publica el evento de
// dominio osil.vendido SOLO cuando la compra involucró un lote físico.
// No sabe que el transporte es HTTP ni que la pasarela es Stripe —
// solo conoce los puertos.
//
// Importante para OXXO y transferencia bancaria (pagos asíncronos): NO
// se activa la orden (ni el premium del usuario) solo porque llegó
// checkout.session.completed. Solo se marca pagada cuando el gateway ya
// confirmó el dinero (evento.EsPagoConfirmado) — para tarjeta eso pasa
// en el propio completed, para OXXO/transferencia llega después vía
// checkout.session.async_payment_succeeded. Si el pago asíncrono falla o
// expira (evento.EsPagoFallido), la orden se cancela y se libera el
// stock en vez de activarse.
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

	switch {
	case evento.EsPagoConfirmado:
		// MarcarOrdenPagada ya decide internamente, dentro de una sola
		// transacción, si esto fue una cama_cafe (descuenta stock) o una
		// suscripcion (activa es_premium + premium_hasta del usuario).
		// Llegar aquí significa que el dinero YA está confirmado por
		// Stripe (tarjeta pagada, o OXXO/transferencia ya liquidados) —
		// nunca se activa el premium antes de este punto. idLote regresa
		// 0 cuando el producto comprado no tenía un lote físico asociado
		// (por ejemplo, cualquier suscripción).
		idOrden, idLote, err := uc.repo.MarcarOrdenPagada(ctx, evento.CheckoutSessionID, evento.PaymentIntentID)
		if err != nil {
			return fmt.Errorf("error marcando orden pagada: %w", err)
		}

		if idLote != 0 && uc.publisher != nil {
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

	case evento.EsPagoFallido:
		// El voucher de OXXO expiró o la transferencia nunca llegó:
		// cancelamos la orden y liberamos el stock/lote si aplicaba, en
		// vez de dejarla 'pendiente' para siempre.
		if err := uc.repo.CancelarOrdenPorCheckoutSession(ctx, evento.CheckoutSessionID); err != nil {
			return fmt.Errorf("error cancelando orden por pago fallido: %w", err)
		}
	}

	return uc.repo.MarcarEventoProcesado(ctx, evento.EventID)
}