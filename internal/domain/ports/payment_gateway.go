//internal/domain/ports/payment_gateway.go
package ports

// CheckoutSessionEvent es la representación mínima, agnóstica de
// Stripe, de lo que la aplicación necesita del webhook para actuar.
// Evita que el caso de uso dependa de tipos del SDK de Stripe.
//
// OXXO y transferencia bancaria (customer_balance) son métodos de pago
// ASÍNCRONOS: checkout.session.completed se dispara apenas se genera el
// voucher/CLABE, no cuando el cliente realmente paga. Por eso separamos
// "la sesión se completó" de "el dinero ya llegó":
//   - EsPagoConfirmado: el dinero ya está confirmado (tarjeta pagada al
//     instante dentro de checkout.session.completed, o el evento
//     checkout.session.async_payment_succeeded para OXXO/transferencia).
//     Solo en este caso se debe activar la orden / el premium del usuario.
//   - EsPagoFallido: el pago asíncrono expiró o falló
//     (checkout.session.async_payment_failed) — hay que cancelar la orden
//     y liberar el stock/lote si aplicaba.
type CheckoutSessionEvent struct {
	EventID           string
	EventType         string
	CheckoutSessionID string
	PaymentIntentID   string
	CompradorEmail    string
	MontoTotal        float64 // ya convertido de centavos a unidad
	EsPagoConfirmado  bool
	EsPagoFallido     bool
}

// PaymentGateway es el puerto hacia la pasarela de pagos. La
// implementación concreta (Stripe) vive en infrastructure/stripegateway.
// Si mañana kajve agrega otra pasarela para este flujo, solo se
// implementa este mismo puerto.
type PaymentGateway interface {
	// CrearCliente crea/registra al comprador en la pasarela (guest,
	// sin cuenta en kajve) y regresa su ID externo.
	CrearCliente(nombre, email string) (customerID string, err error)

	// CrearSesionPago crea una sesión de pago único por el precio del
	// osil completo y regresa (sessionID, url de checkout).
	CrearSesionPago(customerID string, idOrden int, nombreLote string, precioTotal float64, moneda string) (sessionID string, checkoutURL string, err error)

	// VerificarYParsearWebhook valida la firma del webhook entrante y
	// lo traduce a un evento agnóstico de Stripe.
	VerificarYParsearWebhook(payload []byte, firma string) (CheckoutSessionEvent, error)
}
