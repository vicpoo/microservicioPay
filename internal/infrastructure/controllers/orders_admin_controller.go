// internal/infrastructure/controllers/orders_admin_controller.go
package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"log"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kajve/payment-service/internal/application/usecases"
	"github.com/kajve/payment-service/internal/domain/entities"
)

// OrdersAdminController expone lectura y administración de órdenes.
// Debe montarse SIEMPRE detrás de un middleware de autenticación de
// administrador — a diferencia de OrdersController (guest checkout),
// aquí se expone PII (email, teléfono) de compradores.
type OrdersAdminController struct {
	listar     *usecases.ListarOrdenesUseCase
	obtener    *usecases.ObtenerOrdenUseCase
	actualizar *usecases.ActualizarEstadoOrdenUseCase
}

func NewOrdersAdminController(
	listar *usecases.ListarOrdenesUseCase,
	obtener *usecases.ObtenerOrdenUseCase,
	actualizar *usecases.ActualizarEstadoOrdenUseCase,
) *OrdersAdminController {
	return &OrdersAdminController{listar: listar, obtener: obtener, actualizar: actualizar}
}

// GET /admin/orders?estado=pagada&id_lote=3&limit=50&offset=0
func (c *OrdersAdminController) ListarOrdenes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	idLote, _ := strconv.Atoi(q.Get("id_lote"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	ordenes, err := c.listar.Execute(r.Context(), usecases.ListarOrdenesInput{
		Estado: q.Get("estado"),
		IDLote: idLote,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Printf("ERROR listando órdenes: %v", err) // <- AGREGAR ESTO
		http.Error(w, "error interno listando órdenes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ordenes)
}

// GET /admin/orders/{id}
func (c *OrdersAdminController) ObtenerOrden(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	orden, err := c.obtener.Execute(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrOrdenNoEncontrada) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "error interno obteniendo la orden", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orden)
}

type actualizarEstadoRequest struct {
	Estado string `json:"estado"`
}

// PATCH /admin/orders/{id}/estado    body: {"estado": "cancelada"}
func (c *OrdersAdminController) ActualizarEstado(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	var req actualizarEstadoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}

	err = c.actualizar.Execute(r.Context(), usecases.ActualizarEstadoOrdenInput{
		IDOrden:     id,
		NuevoEstado: req.Estado,
	})
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrOrdenNoEncontrada):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, entities.ErrEstadoInvalido):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "error interno actualizando el estado", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}