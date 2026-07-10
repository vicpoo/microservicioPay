//internal/infrastructure/controllers/orders_controller.go
package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kajve/payment-service/internal/application/usecases"
	"github.com/kajve/payment-service/internal/domain/entities"
)

type ordenRequest struct {
	IDProducto        int    `json:"id_producto"`
	NombreComprador   string `json:"nombre_comprador"`
	EmailComprador    string `json:"email_comprador"`
	TelefonoComprador string `json:"telefono_comprador,omitempty"`
	Pais              string `json:"pais,omitempty"`
	// IDUsuario es requerido SOLO si el id_producto comprado es de tipo
	// "suscripcion" (necesitamos saber a qué usuario de kajve activarle
	// el plan). Para camas de café se puede omitir.
	IDUsuario *int `json:"id_usuario,omitempty"`
}

// OrdersController es un adaptador de entrada: traduce HTTP <-> caso de
// uso. No tiene lógica de negocio, solo parseo/validación de transporte
// y códigos de estado HTTP.
type OrdersController struct {
	crearOrden *usecases.CrearOrdenUseCase
}

func NewOrdersController(crearOrden *usecases.CrearOrdenUseCase) *OrdersController {
	return &OrdersController{crearOrden: crearOrden}
}

func (c *OrdersController) CrearOrden(w http.ResponseWriter, r *http.Request) {
	var req ordenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}
	if req.IDProducto == 0 || req.NombreComprador == "" || req.EmailComprador == "" {
		http.Error(w, "id_producto, nombre_comprador y email_comprador son requeridos", http.StatusBadRequest)
		return
	}

	out, err := c.crearOrden.Execute(r.Context(), usecases.CrearOrdenInput{
		IDProducto:        req.IDProducto,
		NombreComprador:   req.NombreComprador,
		EmailComprador:    req.EmailComprador,
		TelefonoComprador: req.TelefonoComprador,
		Pais:              req.Pais,
		IDUsuario:         req.IDUsuario,
	})
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrProductoNoEncontrado):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, entities.ErrProductoNoDisponible):
			http.Error(w, err.Error(), http.StatusConflict)
		case errors.Is(err, entities.ErrUsuarioRequerido):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "error interno creando la orden", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id_orden":     out.IDOrden,
		"checkout_url": out.CheckoutURL,
	})
}