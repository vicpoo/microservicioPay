// internal/infrastructure/controllers/catalog_admin_controller.go
package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kajve/payment-service/internal/application/usecases"
	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
)

// CatalogAdminController expone el CRUD del catálogo (camas de café y
// planes de suscripción). Debe montarse SIEMPRE detrás de un middleware
// de autenticación de administrador — igual que OrdersAdminController,
// todavía no tiene auth propia (ver TODO en routes.go).
type CatalogAdminController struct {
	crear      *usecases.CrearProductoUseCase
	listar     *usecases.ListarProductosUseCase
	obtener    *usecases.ObtenerProductoUseCase
	actualizar *usecases.ActualizarProductoUseCase
	eliminar   *usecases.EliminarProductoUseCase
}

func NewCatalogAdminController(
	crear *usecases.CrearProductoUseCase,
	listar *usecases.ListarProductosUseCase,
	obtener *usecases.ObtenerProductoUseCase,
	actualizar *usecases.ActualizarProductoUseCase,
	eliminar *usecases.EliminarProductoUseCase,
) *CatalogAdminController {
	return &CatalogAdminController{
		crear:      crear,
		listar:     listar,
		obtener:    obtener,
		actualizar: actualizar,
		eliminar:   eliminar,
	}
}

type productoRequest struct {
	TipoProducto string  `json:"tipo_producto"` // "cama_cafe" | "suscripcion"
	Nombre       string  `json:"nombre"`
	Descripcion  string  `json:"descripcion"`
	Precio       float64 `json:"precio"`
	Moneda       string  `json:"moneda,omitempty"`
	ImagenURL    string  `json:"imagen_url,omitempty"`
	Activo       *bool   `json:"activo,omitempty"`

	IDLote   *int    `json:"id_lote,omitempty"`
	Variedad string  `json:"variedad,omitempty"`
	PesoKg   float64 `json:"peso_kg,omitempty"`
	Stock    int     `json:"stock,omitempty"`

	PlanSuscripcion string `json:"plan_suscripcion,omitempty"`
	DuracionDias    int    `json:"duracion_dias,omitempty"`
	LotesMax        int    `json:"lotes_max,omitempty"`
}

type productoResponse struct {
	ID              int     `json:"id_producto"`
	TipoProducto    string  `json:"tipo_producto"`
	Nombre          string  `json:"nombre"`
	Descripcion     string  `json:"descripcion"`
	Precio          float64 `json:"precio"`
	Moneda          string  `json:"moneda"`
	ImagenURL       string  `json:"imagen_url"`
	Activo          bool    `json:"activo"`
	IDLote          *int    `json:"id_lote,omitempty"`
	Variedad        string  `json:"variedad,omitempty"`
	PesoKg          float64 `json:"peso_kg,omitempty"`
	Stock           int     `json:"stock,omitempty"`
	PlanSuscripcion string  `json:"plan_suscripcion,omitempty"`
	DuracionDias    int     `json:"duracion_dias,omitempty"`
	LotesMax        int     `json:"lotes_max,omitempty"`
}

func toProductoResponse(p entities.Producto) productoResponse {
	return productoResponse{
		ID:              p.ID,
		TipoProducto:    string(p.TipoProducto),
		Nombre:          p.Nombre,
		Descripcion:     p.Descripcion,
		Precio:          p.Precio,
		Moneda:          p.Moneda,
		ImagenURL:       p.ImagenURL,
		Activo:          p.Activo,
		IDLote:          p.IDLote,
		Variedad:        p.Variedad,
		PesoKg:          p.PesoKg,
		Stock:           p.Stock,
		PlanSuscripcion: p.PlanSuscripcion,
		DuracionDias:    p.DuracionDias,
		LotesMax:        p.LotesMax,
	}
}

// POST /admin/catalogo
func (c *CatalogAdminController) CrearProducto(w http.ResponseWriter, r *http.Request) {
	var req productoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}
	if req.Nombre == "" || req.Precio <= 0 {
		http.Error(w, "nombre y precio (>0) son requeridos", http.StatusBadRequest)
		return
	}

	id, err := c.crear.Execute(r.Context(), usecases.CrearProductoInput{
		TipoProducto:    req.TipoProducto,
		Nombre:          req.Nombre,
		Descripcion:     req.Descripcion,
		Precio:          req.Precio,
		Moneda:          req.Moneda,
		ImagenURL:       req.ImagenURL,
		Activo:          req.Activo,
		IDLote:          req.IDLote,
		Variedad:        req.Variedad,
		PesoKg:          req.PesoKg,
		Stock:           req.Stock,
		PlanSuscripcion: req.PlanSuscripcion,
		DuracionDias:    req.DuracionDias,
		LotesMax:        req.LotesMax,
	})
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrProductoTipoInvalido), errors.Is(err, entities.ErrDatosProductoInvalidos):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "error interno creando el producto", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id_producto": id})
}

// GET /admin/catalogo?tipo_producto=cama_cafe&solo_activos=true&limit=50&offset=0
func (c *CatalogAdminController) ListarProductos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	soloActivos, _ := strconv.ParseBool(q.Get("solo_activos"))

	productos, err := c.listar.Execute(r.Context(), ports.FiltroCatalogo{
		TipoProducto: q.Get("tipo_producto"),
		SoloActivos:  soloActivos,
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		http.Error(w, "error interno listando el catálogo", http.StatusInternalServerError)
		return
	}

	resp := make([]productoResponse, 0, len(productos))
	for _, p := range productos {
		resp = append(resp, toProductoResponse(p))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GET /admin/catalogo/{id}
func (c *CatalogAdminController) ObtenerProducto(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	p, err := c.obtener.Execute(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrProductoNoEncontrado) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "error interno obteniendo el producto", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toProductoResponse(p))
}

// PUT /admin/catalogo/{id}
func (c *CatalogAdminController) ActualizarProducto(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	var req productoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}

	activo := true
	if req.Activo != nil {
		activo = *req.Activo
	}

	err = c.actualizar.Execute(r.Context(), usecases.ActualizarProductoInput{
		ID:              id,
		Nombre:          req.Nombre,
		Descripcion:     req.Descripcion,
		Precio:          req.Precio,
		Moneda:          req.Moneda,
		ImagenURL:       req.ImagenURL,
		Activo:          activo,
		Variedad:        req.Variedad,
		PesoKg:          req.PesoKg,
		Stock:           req.Stock,
		PlanSuscripcion: req.PlanSuscripcion,
		DuracionDias:    req.DuracionDias,
		LotesMax:        req.LotesMax,
	})
	if err != nil {
		if errors.Is(err, entities.ErrProductoNoEncontrado) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "error interno actualizando el producto", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /admin/catalogo/{id}
func (c *CatalogAdminController) EliminarProducto(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	if err := c.eliminar.Execute(r.Context(), id); err != nil {
		if errors.Is(err, entities.ErrProductoNoEncontrado) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}