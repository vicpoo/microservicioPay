//internal/infrastructure/repository/postgres_order_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kajve/payment-service/internal/domain/entities"
)

// PostgresOrderRepository implementa ports.OrderRepository. El resto
// del sistema (dominio, casos de uso) nunca importa este paquete
// directamente; solo lo hace main.go al momento de inyectarlo.
type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOrderRepository(ctx context.Context, databaseURL string) (*PostgresOrderRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping a la base de datos falló: %w", err)
	}
	return &PostgresOrderRepository{pool: pool}, nil
}

func (r *PostgresOrderRepository) Close() {
	r.pool.Close()
}

func (r *PostgresOrderRepository) LoteVendible(ctx context.Context, idLote int) (entities.LoteVendible, error) {
	var l entities.LoteVendible
	l.IDLote = idLote
	row := r.pool.QueryRow(ctx, `
		SELECT nombre_lote, precio_venta, disponible_para_venta
		FROM lotes_cafe
		WHERE id_lote = $1 AND estado = 'finalizado'
	`, idLote)
	if err := row.Scan(&l.NombreLote, &l.Precio, &l.Disponible); err != nil {
		return entities.LoteVendible{}, err
	}
	return l, nil
}

func (r *PostgresOrderRepository) CrearComprador(ctx context.Context, c *entities.Comprador) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO compradores (nombre, email, telefono, pais, stripe_customer_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id_comprador
	`, c.Nombre, c.Email, c.Telefono, c.Pais, c.StripeCustomerID).Scan(&id)
	return id, err
}

func (r *PostgresOrderRepository) CrearOrden(ctx context.Context, o *entities.Orden) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ordenes (id_lote, id_comprador, precio_total, moneda, estado_orden)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id_orden
	`, o.IDLote, o.IDComprador, o.PrecioTotal, o.Moneda, entities.EstadoPendiente).Scan(&id)
	return id, err
}

func (r *PostgresOrderRepository) ActualizarCheckoutSession(ctx context.Context, idOrden int, sessionID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ordenes SET stripe_checkout_session_id = $1 WHERE id_orden = $2
	`, sessionID, idOrden)
	return err
}

func (r *PostgresOrderRepository) MarcarOrdenPagada(ctx context.Context, checkoutSessionID, paymentIntentID string) (idOrden int, idLote int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE ordenes
		SET estado_orden = 'pagada', stripe_payment_intent_id = $1, fecha_pago = NOW()
		WHERE stripe_checkout_session_id = $2 AND estado_orden = 'pendiente'
		RETURNING id_orden, id_lote
	`, paymentIntentID, checkoutSessionID).Scan(&idOrden, &idLote)
	if err != nil {
		return 0, 0, err
	}

	if _, err = tx.Exec(ctx, `
		UPDATE lotes_cafe SET disponible_para_venta = FALSE WHERE id_lote = $1
	`, idLote); err != nil {
		return 0, 0, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO historial_eventos (id_lote, id_usuario, tipo_evento, descripcion)
		VALUES ($1, NULL, 'osil_vendido', 'Orden pagada vía Stripe')
	`, idLote); err != nil {
		return 0, 0, err
	}

	return idOrden, idLote, tx.Commit(ctx)
}

func (r *PostgresOrderRepository) EventoYaProcesado(ctx context.Context, stripeEventID string) (bool, error) {
	var procesado bool
	err := r.pool.QueryRow(ctx, `
		SELECT procesado FROM stripe_webhook_events WHERE id_evento_stripe = $1
	`, stripeEventID).Scan(&procesado)
	if err != nil {
		return false, nil // no existe todavía
	}
	return procesado, nil
}

func (r *PostgresOrderRepository) RegistrarEventoWebhook(ctx context.Context, stripeEventID, tipoEvento string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO stripe_webhook_events (id_evento_stripe, tipo_evento, payload)
		VALUES ($1, $2, $3)
		ON CONFLICT (id_evento_stripe) DO NOTHING
	`, stripeEventID, tipoEvento, payload)
	return err
}

func (r *PostgresOrderRepository) MarcarEventoProcesado(ctx context.Context, stripeEventID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE stripe_webhook_events SET procesado = TRUE, fecha_procesado = NOW()
		WHERE id_evento_stripe = $1
	`, stripeEventID)
	return err
}
