//internal/infrastructure/repository/postgres_order_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kajve/payment-service/internal/domain/entities"
	"github.com/kajve/payment-service/internal/domain/ports"
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

// ============================================================
// Catálogo
// ============================================================

func scanProducto(row interface {
	Scan(dest ...any) error
}) (entities.Producto, error) {
	var p entities.Producto
	var idLote *int
	var variedad, planSuscripcion *string
	var pesoKg *float64
	var duracionDias, lotesMax *int

	err := row.Scan(
		&p.ID, &p.TipoProducto, &p.Nombre, &p.Descripcion, &p.Precio, &p.Moneda,
		&p.ImagenURL, &p.Activo,
		&idLote, &variedad, &pesoKg, &p.Stock,
		&planSuscripcion, &duracionDias, &lotesMax,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return entities.Producto{}, err
	}
	p.IDLote = idLote
	if variedad != nil {
		p.Variedad = *variedad
	}
	if pesoKg != nil {
		p.PesoKg = *pesoKg
	}
	if planSuscripcion != nil {
		p.PlanSuscripcion = *planSuscripcion
	}
	if duracionDias != nil {
		p.DuracionDias = *duracionDias
	}
	if lotesMax != nil {
		p.LotesMax = *lotesMax
	}
	return p, nil
}

const selectProductoCols = `
	id_producto, tipo_producto, nombre, descripcion, precio, moneda,
	COALESCE(imagen_url, ''), activo,
	id_lote, variedad, peso_kg, stock,
	plan_suscripcion, duracion_dias, lotes_max,
	created_at, updated_at
`

func (r *PostgresOrderRepository) ProductoVendible(ctx context.Context, idProducto int) (entities.Producto, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+selectProductoCols+`
		FROM catalogo_productos
		WHERE id_producto = $1
	`, idProducto)
	return scanProducto(row)
}

func (r *PostgresOrderRepository) CrearProducto(ctx context.Context, p *entities.Producto) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO catalogo_productos
			(tipo_producto, nombre, descripcion, precio, moneda, imagen_url, activo,
			 id_lote, variedad, peso_kg, stock,
			 plan_suscripcion, duracion_dias, lotes_max)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id_producto
	`, p.TipoProducto, p.Nombre, p.Descripcion, p.Precio, p.Moneda, nullIfEmpty(p.ImagenURL), p.Activo,
		p.IDLote, nullIfEmpty(p.Variedad), nullIfZero(p.PesoKg), p.Stock,
		nullIfEmpty(p.PlanSuscripcion), nullIfZeroInt(p.DuracionDias), nullIfZeroInt(p.LotesMax),
	).Scan(&id)
	return id, err
}

func (r *PostgresOrderRepository) ObtenerProducto(ctx context.Context, idProducto int) (entities.Producto, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+selectProductoCols+`
		FROM catalogo_productos
		WHERE id_producto = $1
	`, idProducto)
	p, err := scanProducto(row)
	if err != nil {
		return entities.Producto{}, entities.ErrProductoNoEncontrado
	}
	return p, nil
}

func (r *PostgresOrderRepository) ListarProductos(ctx context.Context, filtro ports.FiltroCatalogo) ([]entities.Producto, error) {
	limit := filtro.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filtro.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+selectProductoCols+`
		FROM catalogo_productos
		WHERE ($1 = '' OR tipo_producto::text = $1)
		  AND (NOT $2 OR activo = TRUE)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, filtro.TipoProducto, filtro.SoloActivos, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entities.Producto
	for rows.Next() {
		p, err := scanProducto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresOrderRepository) ActualizarProducto(ctx context.Context, p *entities.Producto) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE catalogo_productos SET
			nombre = $1, descripcion = $2, precio = $3, moneda = $4,
			imagen_url = $5, activo = $6,
			variedad = $7, peso_kg = $8, stock = $9,
			plan_suscripcion = $10, duracion_dias = $11, lotes_max = $12,
			updated_at = NOW()
		WHERE id_producto = $13
	`, p.Nombre, p.Descripcion, p.Precio, p.Moneda,
		nullIfEmpty(p.ImagenURL), p.Activo,
		nullIfEmpty(p.Variedad), nullIfZero(p.PesoKg), p.Stock,
		nullIfEmpty(p.PlanSuscripcion), nullIfZeroInt(p.DuracionDias), nullIfZeroInt(p.LotesMax),
		p.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return entities.ErrProductoNoEncontrado
	}
	return nil
}

func (r *PostgresOrderRepository) EliminarProducto(ctx context.Context, idProducto int) error {
	// Eliminación física. Si el producto ya tiene órdenes asociadas, la
	// FK en `ordenes.id_producto` bloqueará el DELETE — en ese caso lo
	// correcto es desactivarlo (activo=false) en vez de borrarlo.
	tag, err := r.pool.Exec(ctx, `DELETE FROM catalogo_productos WHERE id_producto = $1`, idProducto)
	if err != nil {
		return fmt.Errorf("no se pudo eliminar (¿tiene órdenes asociadas? intenta desactivarlo en vez de borrarlo): %w", err)
	}
	if tag.RowsAffected() == 0 {
		return entities.ErrProductoNoEncontrado
	}
	return nil
}

// ============================================================
// Compradores
// ============================================================

func (r *PostgresOrderRepository) CrearComprador(ctx context.Context, c *entities.Comprador) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO compradores (nombre, email, telefono, pais, stripe_customer_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id_comprador
	`, c.Nombre, c.Email, c.Telefono, c.Pais, c.StripeCustomerID).Scan(&id)
	return id, err
}

// ============================================================
// Órdenes
// ============================================================

func (r *PostgresOrderRepository) CrearOrden(ctx context.Context, o *entities.Orden) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ordenes (id_lote, id_producto, id_comprador, id_usuario, precio_total, moneda, estado_orden, tipo_orden)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id_orden
	`, o.IDLote, o.IDProducto, o.IDComprador, o.IDUsuario, o.PrecioTotal, o.Moneda, entities.EstadoPendiente, o.TipoOrden).Scan(&id)
	return id, err
}

func (r *PostgresOrderRepository) ActualizarCheckoutSession(ctx context.Context, idOrden int, sessionID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ordenes SET stripe_checkout_session_id = $1 WHERE id_orden = $2
	`, sessionID, idOrden)
	return err
}

// MarcarOrdenPagada marca la orden como pagada y, según tipo_orden,
// descuenta stock del catálogo (cama_cafe) o activa el premium del
// usuario (suscripcion). Todo en una sola transacción. Regresa
// idLote = 0 cuando el producto comprado no tenía un lote físico
// asociado (para que el caso de uso sepa que no debe publicar el evento
// de dominio osil.vendido).
func (r *PostgresOrderRepository) MarcarOrdenPagada(ctx context.Context, checkoutSessionID, paymentIntentID string) (idOrden int, idLote int, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	var idProducto int
	var idUsuario *int
	var tipoOrden string

	err = tx.QueryRow(ctx, `
		UPDATE ordenes
		SET estado_orden = 'pagada', stripe_payment_intent_id = $1, fecha_pago = NOW()
		WHERE stripe_checkout_session_id = $2 AND estado_orden = 'pendiente'
		RETURNING id_orden, id_producto, id_usuario, tipo_orden::text
	`, paymentIntentID, checkoutSessionID).Scan(&idOrden, &idProducto, &idUsuario, &tipoOrden)
	if err != nil {
		return 0, 0, err
	}

	switch tipoOrden {
	case string(entities.TipoCamaCafe):
		var idLoteAsociado *int
		var stockActual int
		err = tx.QueryRow(ctx, `
			SELECT id_lote, stock FROM catalogo_productos WHERE id_producto = $1 FOR UPDATE
		`, idProducto).Scan(&idLoteAsociado, &stockActual)
		if err != nil {
			return 0, 0, fmt.Errorf("no se pudo bloquear el producto para descontar stock: %w", err)
		}

		nuevoStock := stockActual - 1
		if nuevoStock < 0 {
			nuevoStock = 0
		}
		if _, err = tx.Exec(ctx, `
			UPDATE catalogo_productos
			SET stock = $1, activo = (($1) > 0), updated_at = NOW()
			WHERE id_producto = $2
		`, nuevoStock, idProducto); err != nil {
			return 0, 0, fmt.Errorf("error descontando stock: %w", err)
		}

		if idLoteAsociado != nil {
			idLote = *idLoteAsociado
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
		}

	case string(entities.TipoSuscripcion):
		// Ya NO se usa la tabla suscripciones. En vez de crear un
		// registro ahí, se activa directamente el boolean es_premium
		// del usuario y se le fija hasta cuándo dura, tomando la
		// duración del plan comprado en el catálogo.
		if idUsuario == nil {
			return 0, 0, entities.ErrUsuarioRequerido
		}
		var duracionDias int
		if err = tx.QueryRow(ctx, `
			SELECT duracion_dias FROM catalogo_productos WHERE id_producto = $1
		`, idProducto).Scan(&duracionDias); err != nil {
			return 0, 0, fmt.Errorf("no se pudo leer la duración del plan: %w", err)
		}

		tag, err := tx.Exec(ctx, `
			UPDATE usuarios
			SET es_premium = TRUE,
			    premium_hasta = NOW() + ($2 || ' days')::interval
			WHERE id_usuario = $1
		`, *idUsuario, duracionDias)
		if err != nil {
			return 0, 0, fmt.Errorf("error activando premium del usuario: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return 0, 0, fmt.Errorf("no se encontró el usuario %d para activar premium", *idUsuario)
		}
		// idLote se queda en 0: no hay evento osil.vendido para suscripciones.
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

const selectOrdenConCompradorCols = `
	o.id_orden, o.id_lote, o.id_producto, o.tipo_orden::text, o.id_comprador, o.id_usuario,
	o.precio_total, o.moneda, o.estado_orden,
	COALESCE(o.stripe_checkout_session_id, ''),
	COALESCE(o.stripe_payment_intent_id, ''),
	o.fecha_orden, o.fecha_pago,
	c.nombre, c.email, c.telefono, c.pais,
	cp.nombre
`

func scanOrdenConComprador(row interface {
	Scan(dest ...any) error
}) (entities.OrdenConComprador, error) {
	var oc entities.OrdenConComprador
	var idLote *int
	var tipoOrden string

	err := row.Scan(
		&oc.ID, &idLote, &oc.IDProducto, &tipoOrden, &oc.IDComprador, &oc.IDUsuario,
		&oc.PrecioTotal, &oc.Moneda, &oc.Estado,
		&oc.StripeCheckoutSessionID, &oc.StripePaymentIntentID,
		&oc.FechaOrden, &oc.FechaPago,
		&oc.NombreComprador, &oc.EmailComprador, &oc.TelefonoComprador, &oc.PaisComprador,
		&oc.NombreProducto,
	)
	if err != nil {
		return entities.OrdenConComprador{}, err
	}
	oc.IDLote = idLote
	oc.TipoOrden = entities.TipoProducto(tipoOrden)
	return oc, nil
}

func (r *PostgresOrderRepository) ListarOrdenes(ctx context.Context, filtro ports.FiltroOrdenes) ([]entities.OrdenConComprador, error) {
	limit := filtro.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filtro.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `
		SELECT `+selectOrdenConCompradorCols+`
		FROM ordenes o
		JOIN compradores c ON c.id_comprador = o.id_comprador
		JOIN catalogo_productos cp ON cp.id_producto = o.id_producto
		WHERE ($1 = '' OR o.estado_orden::text = $1)
		  AND ($2 = 0 OR o.id_lote = $2)
		ORDER BY o.fecha_orden DESC
		LIMIT $3 OFFSET $4
	`, filtro.Estado, filtro.IDLote, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entities.OrdenConComprador
	for rows.Next() {
		oc, err := scanOrdenConComprador(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, oc)
	}
	return out, rows.Err()
}

func (r *PostgresOrderRepository) ObtenerOrdenPorID(ctx context.Context, idOrden int) (entities.OrdenConComprador, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+selectOrdenConCompradorCols+`
		FROM ordenes o
		JOIN compradores c ON c.id_comprador = o.id_comprador
		JOIN catalogo_productos cp ON cp.id_producto = o.id_producto
		WHERE o.id_orden = $1
	`, idOrden)
	oc, err := scanOrdenConComprador(row)
	if err != nil {
		return entities.OrdenConComprador{}, entities.ErrOrdenNoEncontrada
	}
	return oc, nil
}

// ActualizarEstadoOrden cambia el estado de la orden. Si se cancela una
// orden de tipo cama_cafe, se libera la disponibilidad: se regresa 1 al
// stock del producto y, si tenía un lote asociado, se vuelve a marcar
// como disponible_para_venta. (Este era uno de los pendientes del
// servicio: "liberar disponibilidad del lote al cancelar".)
func (r *PostgresOrderRepository) ActualizarEstadoOrden(ctx context.Context, idOrden int, nuevoEstado entities.EstadoOrden) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var idProducto int
	var tipoOrden string
	var estadoAnterior string
	err = tx.QueryRow(ctx, `
		SELECT id_producto, tipo_orden::text, estado_orden::text FROM ordenes WHERE id_orden = $1 FOR UPDATE
	`, idOrden).Scan(&idProducto, &tipoOrden, &estadoAnterior)
	if err != nil {
		return entities.ErrOrdenNoEncontrada
	}

	tag, err := tx.Exec(ctx, `
		UPDATE ordenes SET estado_orden = $1::estado_orden WHERE id_orden = $2
	`, string(nuevoEstado), idOrden)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return entities.ErrOrdenNoEncontrada
	}

	seLiberaDisponibilidad := nuevoEstado == entities.EstadoCancelada && estadoAnterior != string(entities.EstadoCancelada)
	if seLiberaDisponibilidad && tipoOrden == string(entities.TipoCamaCafe) {
		var idLote *int
		if err = tx.QueryRow(ctx, `
			UPDATE catalogo_productos
			SET stock = stock + 1, activo = TRUE, updated_at = NOW()
			WHERE id_producto = $1
			RETURNING id_lote
		`, idProducto).Scan(&idLote); err != nil {
			return fmt.Errorf("error liberando stock del producto: %w", err)
		}
		if idLote != nil {
			if _, err = tx.Exec(ctx, `
				UPDATE lotes_cafe SET disponible_para_venta = TRUE WHERE id_lote = $1
			`, *idLote); err != nil {
				return fmt.Errorf("error liberando disponibilidad del lote: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

// ============================================================
// Premium
// ============================================================

// ActivarPremiumUsuario prende es_premium y fija premium_hasta contando
// desde AHORA (no acumula con un plazo previo vigente: cada activación
// resetea el plazo, igual que hacía antes la lógica de suscripciones).
func (r *PostgresOrderRepository) ActivarPremiumUsuario(ctx context.Context, idUsuario int, duracionDias int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE usuarios
		SET es_premium = TRUE,
		    premium_hasta = NOW() + ($2 || ' days')::interval
		WHERE id_usuario = $1
	`, idUsuario, duracionDias)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return entities.ErrUsuarioNoEncontrado
	}
	return nil
}

// ExpirarPremiumsVencidos apaga es_premium para todos los usuarios cuyo
// premium_hasta ya pasó. Reutiliza la función SQL creada en la
// migración (public.expirar_premiums_vencidos) para no duplicar lógica
// entre Go y SQL.
func (r *PostgresOrderRepository) ExpirarPremiumsVencidos(ctx context.Context) (int, error) {
	var afectados int
	err := r.pool.QueryRow(ctx, `SELECT public.expirar_premiums_vencidos()`).Scan(&afectados)
	if err != nil {
		return 0, err
	}
	return afectados, nil
}

// EsPremium se auto-corrige (si ya venció, apaga el boolean) antes de
// leer el estado, para no depender exclusivamente del scheduler.
func (r *PostgresOrderRepository) EsPremium(ctx context.Context, idUsuario int) (bool, error) {
	if _, err := r.pool.Exec(ctx, `
		UPDATE usuarios
		SET es_premium = FALSE
		WHERE id_usuario = $1
		  AND es_premium = TRUE
		  AND premium_hasta IS NOT NULL
		  AND premium_hasta < NOW()
	`, idUsuario); err != nil {
		return false, err
	}

	var esPremium bool
	err := r.pool.QueryRow(ctx, `
		SELECT es_premium FROM usuarios WHERE id_usuario = $1
	`, idUsuario).Scan(&esPremium)
	if err != nil {
		return false, entities.ErrUsuarioNoEncontrado
	}
	return esPremium, nil
}

// --- helpers para nullable ---

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func nullIfZeroInt(i int) any {
	if i == 0 {
		return nil
	}
	return i
}