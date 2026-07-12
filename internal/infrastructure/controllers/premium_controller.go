// internal/infrastructure/controllers/premium_controller.go
package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kajve/payment-service/internal/application/usecases"
)

// PremiumController expone lectura del estado premium de un usuario.
// Es de solo lectura: la activación real ocurre dentro de
// MarcarOrdenPagada cuando Stripe confirma el pago (ver procesar_webhook.go),
// nunca desde aquí.
type PremiumController struct {
	verificar *usecases.VerificarPremiumUseCase
}

func NewPremiumController(verificar *usecases.VerificarPremiumUseCase) *PremiumController {
	return &PremiumController{verificar: verificar}
}

type premiumResponse struct {
	IDUsuario int  `json:"id_usuario"`
	EsPremium bool `json:"es_premium"`
}

// GET /usuarios/{id}/premium
func (c *PremiumController) VerificarPremium(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	esPremium, err := c.verificar.Execute(r.Context(), id)
	if err != nil {
		http.Error(w, "error interno verificando premium", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(premiumResponse{IDUsuario: id, EsPremium: esPremium})
}