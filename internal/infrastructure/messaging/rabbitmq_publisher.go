package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/kajve/payment-service/internal/domain/entities"
)

// RabbitMQPublisher implementa ports.EventPublisher usando el mismo
// broker RabbitMQ que ya describe el ADR-001 del SAD (un exchange de
// eventos de dominio, en vez de los tópicos por-dispositivo de IoT).
type RabbitMQPublisher struct {
	channel  *amqp091.Channel
	exchange string
}

// New regresa (nil, err) si no logra conectar. main.go decide si
// seguir sin publisher (modo degradado) o abortar el arranque.
func New(amqpURL, exchange string) (*RabbitMQPublisher, *amqp091.Connection, error) {
	conn, err := amqp091.Dial(amqpURL)
	if err != nil {
		return nil, nil, fmt.Errorf("no se pudo conectar a RabbitMQ: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("no se pudo abrir canal AMQP: %w", err)
	}
	return &RabbitMQPublisher{channel: ch, exchange: exchange}, conn, nil
}

func (p *RabbitMQPublisher) PublicarOsilVendido(ctx context.Context, evento entities.EventoOsilVendido) error {
	body, err := json.Marshal(evento)
	if err != nil {
		return err
	}
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return p.channel.PublishWithContext(pubCtx, p.exchange, "osil.vendido", false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}
