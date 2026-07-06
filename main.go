// main.go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/kajve/payment-service/internal/application/usecases"
	"github.com/kajve/payment-service/internal/infrastructure/config"
	"github.com/kajve/payment-service/internal/infrastructure/controllers"
	"github.com/kajve/payment-service/internal/infrastructure/messaging"
	"github.com/kajve/payment-service/internal/infrastructure/repository"
	"github.com/kajve/payment-service/internal/infrastructure/routes"
	"github.com/kajve/payment-service/internal/infrastructure/stripegateway"
)


func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuración inválida: %v", err)
	}

	// --- Adaptadores de salida (implementan los puertos del dominio) ---
	orderRepo, err := repository.NewPostgresOrderRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("no se pudo conectar a PostgreSQL: %v", err)
	}
	defer orderRepo.Close()

	paymentGateway := stripegateway.New(cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripeSuccessURL, cfg.StripeCancelURL)

	var eventPublisher *messaging.RabbitMQPublisher
	publisher, amqpConn, err := messaging.New(cfg.RabbitMQURL, cfg.RabbitMQExchange)
	if err != nil {
		log.Printf("aviso: sin conexión a RabbitMQ (%v); el servicio seguirá sin publicar eventos", err)
	} else {
		defer amqpConn.Close()
		eventPublisher = publisher
	}

	// --- Casos de uso (dependen solo de los puertos, no de las implementaciones) ---
	crearOrdenUC := usecases.NewCrearOrdenUseCase(orderRepo, paymentGateway)

	var procesarWebhookUC *usecases.ProcesarWebhookUseCase
	if eventPublisher != nil {
		procesarWebhookUC = usecases.NewProcesarWebhookUseCase(orderRepo, paymentGateway, eventPublisher)
	} else {
		procesarWebhookUC = usecases.NewProcesarWebhookUseCase(orderRepo, paymentGateway, nil)
	}

	// --- Adaptadores de entrada (HTTP) ---
	ordersController := controllers.NewOrdersController(crearOrdenUC)
	webhooksController := controllers.NewWebhooksController(procesarWebhookUC)

	router := routes.NewRouter(ordersController, webhooksController)

	log.Printf("payment-service (arquitectura hexagonal) escuchando en :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
