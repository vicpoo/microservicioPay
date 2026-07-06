// application/usecases/gestionar_ordenes.go
package usecases

import (
	"context"

	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

// --- Listar órdenes (vista admin) ---

type ListarOrdenesInput struct {
	Estado string
	IDLote int
	Limit  int
	Offset int
}

type ListarOrdenesUseCase struct {
	repo ports.OrderRepository
}

func NewListarOrdenesUseCase(repo ports.OrderRepository) *ListarOrdenesUseCase {
	return &ListarOrdenesUseCase{repo: repo}
}

func (uc *ListarOrdenesUseCase) Execute(ctx context.Context, in ListarOrdenesInput) ([]entities.OrdenConComprador, error) {
	return uc.repo.ListarOrdenes(ctx, ports.FiltroOrdenes{
		Estado: in.Estado,
		IDLote: in.IDLote,
		Limit:  in.Limit,
		Offset: in.Offset,
	})
}

// --- Obtener el detalle de una orden ---

type ObtenerOrdenUseCase struct {
	repo ports.OrderRepository
}

func NewObtenerOrdenUseCase(repo ports.OrderRepository) *ObtenerOrdenUseCase {
	return &ObtenerOrdenUseCase{repo: repo}
}

func (uc *ObtenerOrdenUseCase) Execute(ctx context.Context, idOrden int) (entities.OrdenConComprador, error) {
	return uc.repo.ObtenerOrdenPorID(ctx, idOrden)
}

// --- Actualizar el estado de una orden manualmente ---

var estadosValidos = map[entities.EstadoOrden]bool{
	entities.EstadoPendiente:   true,
	entities.EstadoPagada:      true,
	entities.EstadoCancelada:   true,
	entities.EstadoReembolsada: true,
}

type ActualizarEstadoOrdenInput struct {
	IDOrden     int
	NuevoEstado string
}

type ActualizarEstadoOrdenUseCase struct {
	repo ports.OrderRepository
}

func NewActualizarEstadoOrdenUseCase(repo ports.OrderRepository) *ActualizarEstadoOrdenUseCase {
	return &ActualizarEstadoOrdenUseCase{repo: repo}
}

func (uc *ActualizarEstadoOrdenUseCase) Execute(ctx context.Context, in ActualizarEstadoOrdenInput) error {
	estado := entities.EstadoOrden(in.NuevoEstado)
	if !estadosValidos[estado] {
		return entities.ErrEstadoInvalido
	}
	return uc.repo.ActualizarEstadoOrden(ctx, in.IDOrden, estado)
}