// application/usecases/catalogo_usecases.go
package usecases

import (
	"context"

	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

// --- Crear producto ---

type CrearProductoInput struct {
	TipoProducto string // "cama_cafe" | "suscripcion"
	Nombre       string
	Descripcion  string
	Precio       float64
	Moneda       string
	ImagenURL    string
	Activo       *bool // nil = true por default

	// cama_cafe
	IDLote   *int
	Variedad string
	PesoKg   float64
	Stock    int

	// suscripcion
	PlanSuscripcion string
	DuracionDias    int
	LotesMax        int
}

type CrearProductoUseCase struct {
	repo ports.OrderRepository
}

func NewCrearProductoUseCase(repo ports.OrderRepository) *CrearProductoUseCase {
	return &CrearProductoUseCase{repo: repo}
}

func (uc *CrearProductoUseCase) Execute(ctx context.Context, in CrearProductoInput) (int, error) {
	tipo := entities.TipoProducto(in.TipoProducto)
	if tipo != entities.TipoCamaCafe && tipo != entities.TipoSuscripcion {
		return 0, entities.ErrProductoTipoInvalido
	}

	activo := true
	if in.Activo != nil {
		activo = *in.Activo
	}

	p := &entities.Producto{
		TipoProducto: tipo,
		Nombre:       in.Nombre,
		Descripcion:  in.Descripcion,
		Precio:       in.Precio,
		Moneda:       in.Moneda,
		ImagenURL:    in.ImagenURL,
		Activo:       activo,
	}
	if p.Moneda == "" {
		p.Moneda = "MXN"
	}

	switch tipo {
	case entities.TipoCamaCafe:
		if in.PesoKg <= 0 {
			return 0, entities.ErrDatosProductoInvalidos
		}
		p.IDLote = in.IDLote
		p.Variedad = in.Variedad
		p.PesoKg = in.PesoKg
		p.Stock = in.Stock
		if p.Stock == 0 {
			p.Stock = 1
		}
	case entities.TipoSuscripcion:
		if in.PlanSuscripcion == "" || in.DuracionDias <= 0 {
			return 0, entities.ErrDatosProductoInvalidos
		}
		p.PlanSuscripcion = in.PlanSuscripcion
		p.DuracionDias = in.DuracionDias
		p.LotesMax = in.LotesMax
	}

	return uc.repo.CrearProducto(ctx, p)
}

// --- Listar productos ---

type ListarProductosUseCase struct {
	repo ports.OrderRepository
}

func NewListarProductosUseCase(repo ports.OrderRepository) *ListarProductosUseCase {
	return &ListarProductosUseCase{repo: repo}
}

func (uc *ListarProductosUseCase) Execute(ctx context.Context, filtro ports.FiltroCatalogo) ([]entities.Producto, error) {
	return uc.repo.ListarProductos(ctx, filtro)
}

// --- Obtener producto ---

type ObtenerProductoUseCase struct {
	repo ports.OrderRepository
}

func NewObtenerProductoUseCase(repo ports.OrderRepository) *ObtenerProductoUseCase {
	return &ObtenerProductoUseCase{repo: repo}
}

func (uc *ObtenerProductoUseCase) Execute(ctx context.Context, idProducto int) (entities.Producto, error) {
	return uc.repo.ObtenerProducto(ctx, idProducto)
}

// --- Actualizar producto ---

type ActualizarProductoInput struct {
	ID          int
	Nombre      string
	Descripcion string
	Precio      float64
	Moneda      string
	ImagenURL   string
	Activo      bool

	Variedad string
	PesoKg   float64
	Stock    int

	PlanSuscripcion string
	DuracionDias    int
	LotesMax        int
}

type ActualizarProductoUseCase struct {
	repo ports.OrderRepository
}

func NewActualizarProductoUseCase(repo ports.OrderRepository) *ActualizarProductoUseCase {
	return &ActualizarProductoUseCase{repo: repo}
}

func (uc *ActualizarProductoUseCase) Execute(ctx context.Context, in ActualizarProductoInput) error {
	actual, err := uc.repo.ObtenerProducto(ctx, in.ID)
	if err != nil {
		return entities.ErrProductoNoEncontrado
	}

	actual.Nombre = in.Nombre
	actual.Descripcion = in.Descripcion
	actual.Precio = in.Precio
	if in.Moneda != "" {
		actual.Moneda = in.Moneda
	}
	actual.ImagenURL = in.ImagenURL
	actual.Activo = in.Activo

	switch actual.TipoProducto {
	case entities.TipoCamaCafe:
		actual.Variedad = in.Variedad
		actual.PesoKg = in.PesoKg
		actual.Stock = in.Stock
	case entities.TipoSuscripcion:
		actual.PlanSuscripcion = in.PlanSuscripcion
		actual.DuracionDias = in.DuracionDias
		actual.LotesMax = in.LotesMax
	}

	return uc.repo.ActualizarProducto(ctx, &actual)
}

// --- Eliminar producto ---

type EliminarProductoUseCase struct {
	repo ports.OrderRepository
}

func NewEliminarProductoUseCase(repo ports.OrderRepository) *EliminarProductoUseCase {
	return &EliminarProductoUseCase{repo: repo}
}

func (uc *EliminarProductoUseCase) Execute(ctx context.Context, idProducto int) error {
	return uc.repo.EliminarProducto(ctx, idProducto)
}