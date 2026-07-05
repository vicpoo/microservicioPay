package usecases

import (
	"context"
	"fmt"

	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

type CrearOrdenInput struct {
	IDLote            int
	NombreComprador   string
	EmailComprador    string
	TelefonoComprador string
	Pais              string
}

type CrearOrdenOutput struct {
	IDOrden     int
	CheckoutURL string
}

// CrearOrdenUseCase orquesta: validar el lote -> registrar comprador
// -> crear orden -> crear sesión de pago. Solo depende de los puertos
// (ports.OrderRepository, ports.PaymentGateway), nunca de Postgres o
// Stripe directamente — eso es lo que hace esto "hexagonal".
type CrearOrdenUseCase struct {
	repo    ports.OrderRepository
	gateway ports.PaymentGateway
}

func NewCrearOrdenUseCase(repo ports.OrderRepository, gateway ports.PaymentGateway) *CrearOrdenUseCase {
	return &CrearOrdenUseCase{repo: repo, gateway: gateway}
}

func (uc *CrearOrdenUseCase) Execute(ctx context.Context, in CrearOrdenInput) (CrearOrdenOutput, error) {
	lote, err := uc.repo.LoteVendible(ctx, in.IDLote)
	if err != nil {
		return CrearOrdenOutput{}, entities.ErrLoteNoEncontrado
	}
	if !lote.Disponible {
		return CrearOrdenOutput{}, entities.ErrLoteNoDisponible
	}

	customerID, err := uc.gateway.CrearCliente(in.NombreComprador, in.EmailComprador)
	if err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error creando cliente en la pasarela de pago: %w", err)
	}

	idComprador, err := uc.repo.CrearComprador(ctx, &entities.Comprador{
		Nombre:           in.NombreComprador,
		Email:            in.EmailComprador,
		Telefono:         in.TelefonoComprador,
		Pais:             in.Pais,
		StripeCustomerID: customerID,
	})
	if err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error registrando comprador: %w", err)
	}

	idOrden, err := uc.repo.CrearOrden(ctx, &entities.Orden{
		IDLote:      in.IDLote,
		IDComprador: idComprador,
		PrecioTotal: lote.Precio,
		Moneda:      "mxn",
		Estado:      entities.EstadoPendiente,
	})
	if err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error creando la orden: %w", err)
	}

	sessionID, checkoutURL, err := uc.gateway.CrearSesionPago(customerID, idOrden, lote.NombreLote, lote.Precio, "mxn")
	if err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error creando sesión de pago: %w", err)
	}

	if err := uc.repo.ActualizarCheckoutSession(ctx, idOrden, sessionID); err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error guardando checkout session: %w", err)
	}

	return CrearOrdenOutput{IDOrden: idOrden, CheckoutURL: checkoutURL}, nil
}
