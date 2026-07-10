//application/usecases/crear_orden.go
package usecases

import (
	"context"
	"fmt"

	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

type CrearOrdenInput struct {
	IDProducto        int
	NombreComprador   string
	EmailComprador    string
	TelefonoComprador string
	Pais              string
	// IDUsuario es requerido SOLO cuando el producto comprado es una
	// suscripción (necesitamos saber a quién activarle el plan).
	IDUsuario *int
}

type CrearOrdenOutput struct {
	IDOrden     int
	CheckoutURL string
}

// CrearOrdenUseCase orquesta: validar el producto del catálogo ->
// registrar comprador -> crear orden -> crear sesión de pago. Solo
// depende de los puertos (ports.OrderRepository, ports.PaymentGateway),
// nunca de Postgres o Stripe directamente — eso es lo que hace esto
// "hexagonal". Funciona igual para una cama de café que para un plan
// de suscripción: lo único que cambia es qué hace MarcarOrdenPagada
// después de que Stripe confirma el pago (ver procesar_webhook.go).
type CrearOrdenUseCase struct {
	repo    ports.OrderRepository
	gateway ports.PaymentGateway
}

func NewCrearOrdenUseCase(repo ports.OrderRepository, gateway ports.PaymentGateway) *CrearOrdenUseCase {
	return &CrearOrdenUseCase{repo: repo, gateway: gateway}
}

func (uc *CrearOrdenUseCase) Execute(ctx context.Context, in CrearOrdenInput) (CrearOrdenOutput, error) {
	producto, err := uc.repo.ProductoVendible(ctx, in.IDProducto)
	if err != nil {
		return CrearOrdenOutput{}, entities.ErrProductoNoEncontrado
	}
	if !producto.EsVendible() {
		return CrearOrdenOutput{}, entities.ErrProductoNoDisponible
	}
	if producto.TipoProducto == entities.TipoSuscripcion && in.IDUsuario == nil {
		return CrearOrdenOutput{}, entities.ErrUsuarioRequerido
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

	orden := &entities.Orden{
		IDProducto:  producto.ID,
		TipoOrden:   producto.TipoProducto,
		IDLote:      producto.IDLote,
		IDComprador: idComprador,
		IDUsuario:   in.IDUsuario,
		PrecioTotal: producto.Precio,
		Moneda:      "mxn",
		Estado:      entities.EstadoPendiente,
	}

	idOrden, err := uc.repo.CrearOrden(ctx, orden)
	if err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error creando la orden: %w", err)
	}

	nombreArticulo := producto.Nombre
	sessionID, checkoutURL, err := uc.gateway.CrearSesionPago(customerID, idOrden, nombreArticulo, producto.Precio, "mxn")
	if err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error creando sesión de pago: %w", err)
	}

	if err := uc.repo.ActualizarCheckoutSession(ctx, idOrden, sessionID); err != nil {
		return CrearOrdenOutput{}, fmt.Errorf("error guardando checkout session: %w", err)
	}

	return CrearOrdenOutput{IDOrden: idOrden, CheckoutURL: checkoutURL}, nil
}