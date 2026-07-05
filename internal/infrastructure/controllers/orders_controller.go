package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/kajve/payment-service/internal/application/usecases"
	"github.com/kajve/payment-service/internal/domain/entities"
)

type ordenRequest struct {
	IDLote            int    `json:"id_lote"`
	NombreComprador   string `json:"nombre_comprador"`
	EmailComprador    string `json:"email_comprador"`
	TelefonoComprador string `json:"telefono_comprador,omitempty"`
	Pais              string `json:"pais,omitempty"`
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
	if req.IDLote == 0 || req.NombreComprador == "" || req.EmailComprador == "" {
		http.Error(w, "id_lote, nombre_comprador y email_comprador son requeridos", http.StatusBadRequest)
		return
	}

	out, err := c.crearOrden.Execute(r.Context(), usecases.CrearOrdenInput{
		IDLote:            req.IDLote,
		NombreComprador:   req.NombreComprador,
		EmailComprador:    req.EmailComprador,
		TelefonoComprador: req.TelefonoComprador,
		Pais:              req.Pais,
	})
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrLoteNoEncontrado):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, entities.ErrLoteNoDisponible):
			http.Error(w, err.Error(), http.StatusConflict)
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
