//internal/infrastructure/stripegateway/stripe_gateway.go
package stripegateway

import (
	"encoding/json"
	"log"
	"strconv"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/webhook"

	"github.com/kajve/payment-service/internal/domain/ports"
)

// StripeGateway implementa ports.PaymentGateway. Es el único lugar de
// todo el proyecto que importa el SDK de Stripe — si un día se cambia
// de pasarela, solo se reescribe este archivo.
type StripeGateway struct {
	successURL    string
	cancelURL     string
	webhookSecret string
}

func New(secretKey, webhookSecret, successURL, cancelURL string) *StripeGateway {
	stripe.Key = secretKey
	return &StripeGateway{
		successURL:    successURL,
		cancelURL:     cancelURL,
		webhookSecret: webhookSecret,
	}
}

func (g *StripeGateway) CrearCliente(nombre, email string) (string, error) {
	cust, err := customer.New(&stripe.CustomerParams{
		Name:  stripe.String(nombre),
		Email: stripe.String(email),
	})
	if err != nil {
		return "", err
	}
	return cust.ID, nil
}

func (g *StripeGateway) CrearSesionPago(customerID string, idOrden int, nombreLote string, precioTotal float64, moneda string) (string, string, error) {
	// oxxo (pago en efectivo en tienda) y customer_balance (transferencia
	// SPEI vía CLABE virtual) solo existen en MXN — el llamador siempre
	// manda "mxn" (ver crear_orden.go), así que no hace falta condicionar
	// la lista de métodos por moneda. Si algún día se soporta otra
	// moneda/país, esto tendría que volverse dinámico.
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
			"oxxo",
			"customer_balance",
		}),
		PaymentMethodOptions: &stripe.CheckoutSessionPaymentMethodOptionsParams{
			CustomerBalance: &stripe.CheckoutSessionPaymentMethodOptionsCustomerBalanceParams{
				FundingType: stripe.String("bank_transfer"),
				BankTransfer: &stripe.CheckoutSessionPaymentMethodOptionsCustomerBalanceBankTransferParams{
					Type: stripe.String("mx_bank_transfer"),
					// Stripe identifica requested_address_types por RED
					// de transferencia, no por país — para México es
					// "spei" (no "mx"). Los otros valores válidos son
					// aba, swift, sort_code, zengin, sepa e iban.
					RequestedAddressTypes: stripe.StringSlice([]string{"spei"}),
				},
			},
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(moneda),
					UnitAmount: stripe.Int64(int64(precioTotal * 100)),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Osil de café: " + nombreLote),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(g.successURL),
		CancelURL:  stripe.String(g.cancelURL),
		Metadata: map[string]string{
			"id_orden": strconv.Itoa(idOrden),
		},
	}

	s, err := session.New(params)
	if err != nil {
		return "", "", err
	}
	return s.ID, s.URL, nil
}

func (g *StripeGateway) VerificarYParsearWebhook(payload []byte, firma string) (ports.CheckoutSessionEvent, error) {
	event, err := webhook.ConstructEvent(payload, firma, g.webhookSecret)
	if err != nil {
		// El caso de uso convierte esto en un error genérico para la
		// respuesta HTTP (no queremos filtrar detalles al exterior),
		// pero en logs SÍ queremos el motivo real: casi siempre es que
		// STRIPE_WEBHOOK_SECRET en este proceso no coincide con el
		// "signing secret" del endpoint configurado en el Dashboard de
		// Stripe (o con el de `stripe listen`, si se prueba con el CLI).
		log.Printf("[stripe webhook] verificación de firma falló: %v", err)
		return ports.CheckoutSessionEvent{}, err
	}

	out := ports.CheckoutSessionEvent{
		EventID:   event.ID,
		EventType: string(event.Type),
	}

	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		var s stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
			return ports.CheckoutSessionEvent{}, err
		}

		out.CheckoutSessionID = s.ID
		out.MontoTotal = float64(s.AmountTotal) / 100
		if s.PaymentIntent != nil {
			out.PaymentIntentID = s.PaymentIntent.ID
		}
		if s.CustomerDetails != nil {
			out.CompradorEmail = s.CustomerDetails.Email
		}

		// checkout.session.completed dispara de inmediato incluso para
		// OXXO/transferencia, apenas se genera el voucher/CLABE — el
		// dinero todavía no llegó. Para tarjeta (y cualquier método
		// síncrono) payment_status ya viene "paid" en este mismo evento.
		// Para métodos asíncronos, la confirmación real llega después
		// en checkout.session.async_payment_succeeded.
		esPagoSincronoConfirmado := string(s.PaymentStatus) == "paid"
		esAsincronoConfirmado := event.Type == "checkout.session.async_payment_succeeded"
		out.EsPagoConfirmado = esPagoSincronoConfirmado || esAsincronoConfirmado

	case "checkout.session.async_payment_failed":
		var s stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
			return ports.CheckoutSessionEvent{}, err
		}

		out.CheckoutSessionID = s.ID
		out.EsPagoFallido = true
	}

	return out, nil
}
