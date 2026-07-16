//internal/infrastructure/controllers/webhooks_controller.go
package controllers

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/kajve/payment-service/internal/application/usecases"
	"github.com/kajve/payment-service/internal/domain/entities"
)

type WebhooksController struct {
	procesarWebhook *usecases.ProcesarWebhookUseCase
}

func NewWebhooksController(procesarWebhook *usecases.ProcesarWebhookUseCase) *WebhooksController {
	return &WebhooksController{procesarWebhook: procesarWebhook}
}

func (c *WebhooksController) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "no se pudo leer el body", http.StatusServiceUnavailable)
		return
	}

	err = c.procesarWebhook.Execute(r.Context(), payload, r.Header.Get("Stripe-Signature"))
	if err != nil {
		if errors.Is(err, entities.ErrFirmaWebhookInvalida) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// No filtramos el error real al cliente (podría traer detalles
		// internos), pero SÍ lo queremos en logs — sin esto, un 500 acá
		// no dice nada sobre si falló MarcarOrdenPagada, por qué tipo de
		// orden, etc.
		log.Printf("[stripe webhook] error procesando evento: %v", err)
		http.Error(w, "error interno procesando el webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
