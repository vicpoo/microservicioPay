//internal/infrastructure/config/config.go
package config

import "fmt"
import "os"

type Config struct {
	Port                string
	DatabaseURL         string
	StripeSecretKey     string
	StripeWebhookSecret string
	StripeSuccessURL    string
	StripeCancelURL     string
	RabbitMQURL         string
	RabbitMQExchange    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:                getEnv("PORT", "8000"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripeSuccessURL:    getEnv("STRIPE_SUCCESS_URL", "https://kajve.com/orden/exito"),
		StripeCancelURL:     getEnv("STRIPE_CANCEL_URL", "https://kajve.com/orden/cancelada"),
		RabbitMQURL:         getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		RabbitMQExchange:    getEnv("RABBITMQ_EXCHANGE", "kajve.events"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL es requerido")
	}
	if cfg.StripeSecretKey == "" {
		return nil, fmt.Errorf("STRIPE_SECRET_KEY es requerido")
	}
	if cfg.StripeWebhookSecret == "" {
		return nil, fmt.Errorf("STRIPE_WEBHOOK_SECRET es requerido")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
