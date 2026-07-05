package stripegateway

import (
	"encoding/json"
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
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
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
		return ports.CheckoutSessionEvent{}, err
	}

	out := ports.CheckoutSessionEvent{
		EventID:   event.ID,
		EventType: string(event.Type),
	}

	if event.Type == "checkout.session.completed" {
		var s stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &s); err != nil {
			return ports.CheckoutSessionEvent{}, err
		}

		out.EsCheckoutCompletado = true
		out.CheckoutSessionID = s.ID
		out.MontoTotal = float64(s.AmountTotal) / 100
		if s.PaymentIntent != nil {
			out.PaymentIntentID = s.PaymentIntent.ID
		}
		if s.CustomerDetails != nil {
			out.CompradorEmail = s.CustomerDetails.Email
		}
	}

	return out, nil
}
