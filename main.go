// main.go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

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

	// --- Casos de uso administrativos: órdenes ---
	listarOrdenesUC := usecases.NewListarOrdenesUseCase(orderRepo)
	obtenerOrdenUC := usecases.NewObtenerOrdenUseCase(orderRepo)
	actualizarEstadoUC := usecases.NewActualizarEstadoOrdenUseCase(orderRepo)

	// --- Casos de uso administrativos: catálogo (camas de café + suscripciones) ---
	crearProductoUC := usecases.NewCrearProductoUseCase(orderRepo)
	listarProductosUC := usecases.NewListarProductosUseCase(orderRepo)
	obtenerProductoUC := usecases.NewObtenerProductoUseCase(orderRepo)
	actualizarProductoUC := usecases.NewActualizarProductoUseCase(orderRepo)
	eliminarProductoUC := usecases.NewEliminarProductoUseCase(orderRepo)

	// --- Casos de uso: premium ---
	expirarPremiumsUC := usecases.NewExpirarPremiumsUseCase(orderRepo)
	verificarPremiumUC := usecases.NewVerificarPremiumUseCase(orderRepo)

	// Scheduler en memoria: no dependemos de pg_cron porque en Neon los
	// jobs no corren si el compute está en scale-to-zero. Como este
	// servicio ya vive corriendo 24/7, aquí es donde debe vivir el tick
	// que apaga a los usuarios cuyo mes de premium ya venció.
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			n, err := expirarPremiumsUC.Execute(context.Background())
			if err != nil {
				log.Printf("aviso: error expirando premiums vencidos: %v", err)
			} else if n > 0 {
				log.Printf("premium: %d usuario(s) desactivados por vencimiento", n)
			}
			<-ticker.C
		}
	}()

	// --- Adaptadores de entrada (HTTP) ---
	ordersController := controllers.NewOrdersController(crearOrdenUC)
	webhooksController := controllers.NewWebhooksController(procesarWebhookUC)
	ordersAdminController := controllers.NewOrdersAdminController(listarOrdenesUC, obtenerOrdenUC, actualizarEstadoUC)
	catalogAdminController := controllers.NewCatalogAdminController(
		crearProductoUC, listarProductosUC, obtenerProductoUC, actualizarProductoUC, eliminarProductoUC,
	)
	premiumController := controllers.NewPremiumController(verificarPremiumUC)

	router := routes.NewRouter(
		ordersController,
		webhooksController,
		ordersAdminController,
		catalogAdminController,
		premiumController,
	)

	log.Printf("payment-service (arquitectura hexagonal) escuchando en :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}