# kajve — Payment Service (BC-07: Comercialización y Pagos)

Microservicio en Go que permite a **compradores externos, sin cuenta en kajve**
(guest checkout), comprar un osil/lote de café **completo** vía **Stripe
Checkout**. Es un Bounded Context nuevo, separado del flujo existente de
`suscripciones`/`pagos` (que sigue siendo MercadoPago, para el pago de la
suscripción SaaS de los productores).

## Arquitectura hexagonal (ports & adapters)

```
main.go                                  ← composition root (arma todo)
internal/
  domain/                                ← núcleo, cero dependencias externas
    entities/    Orden, Comprador, LoteVendible, EventoOsilVendido
    ports/       interfaces: OrderRepository, PaymentGateway, EventPublisher
  application/
    usecases/    CrearOrdenUseCase, ProcesarWebhookUseCase
                 (orquestan los puertos; no conocen HTTP/Postgres/Stripe)
  infrastructure/
    controllers/ adaptadores de ENTRADA: HTTP <-> casos de uso
    routes/      chi.Router, conecta URLs a controllers
    repository/  adaptador de SALIDA: implementa OrderRepository con Postgres
    stripegateway/ adaptador de SALIDA: implementa PaymentGateway con Stripe
    messaging/   adaptador de SALIDA: implementa EventPublisher con RabbitMQ
    config/      lectura de variables de entorno
```

**Regla de dependencia:** las flechas de import siempre apuntan hacia
adentro. `domain` no importa nada del proyecto. `application` solo
importa `domain` (los puertos). `infrastructure` importa `domain` y
`application` para implementar/inyectar. Solo `main.go` conoce las
cuatro carpetas a la vez. Esto permite, por ejemplo, cambiar Postgres
por otra base de datos, o Stripe por otra pasarela, sin tocar
`usecases` ni `entities` — solo se escribe un nuevo adaptador que
implemente el mismo puerto.

## Cómo encaja en la arquitectura existente (SAD)

- Sigue el mismo estilo Pub/Sub + SOA descrito en el SAD: al confirmar un
  pago, publica un evento `osil.vendido` al mismo broker RabbitMQ (ADR-001),
  que el BC de **Gestión de Osiles** puede consumir para actualizar su propio
  estado (Published Language, igual que la relación Ingesta IoT → ML descrita
  en el Documento de Dominio, Sección 5.1).
- Se despliega como un contenedor Docker independiente más, junto a Ingesta,
  WebSocket Gateway, API REST y ML/NLP (Sección 5 del SAD).
- Usa la misma base de datos PostgreSQL (nuevas tablas del BC de Ventas, ver
  `migration_payment_service.sql`), pero **no** aplica Row-Level Security por
  `user_id` porque las órdenes no pertenecen a un `usuario` del sistema.

## Endpoints

| Método | Ruta               | Descripción                                            |
|--------|---------------------|--------------------------------------------------------|
| POST   | `/orders`            | Crea la orden + Stripe Checkout Session, regresa `checkout_url` |
| POST   | `/webhooks/stripe`   | Recibe confirmaciones de pago de Stripe (idempotente)   |
| GET    | `/health`            | Healthcheck                                             |

## Flujo completo

1. El frontend público llama `POST /orders` con `id_lote`, `nombre_comprador`, `email_comprador`.
2. El servicio valida que el lote esté `finalizado` y `disponible_para_venta`.
3. Crea el `Comprador` (guest) + `Customer` en Stripe.
4. Crea la `Orden` en estado `pendiente`.
5. Crea la Checkout Session de Stripe y regresa `checkout_url` al frontend, que redirige al comprador.
6. Stripe notifica el pago vía webhook `checkout.session.completed`.
7. El servicio verifica la firma, aplica idempotencia por `event.id`, marca la orden `pagada`,
   bloquea el lote (`disponible_para_venta = false`) y publica `osil.vendido` al bus de eventos.

## Pendientes antes de producción (marcados en el código)

- **`orders.go`**: falta el `UPDATE ordenes SET stripe_checkout_session_id = ...`
  después de crear la Checkout Session — quedó señalado con un comentario;
  lo ideal es mover la creación de la orden y el `UPDATE` a una sola
  transacción junto con `CrearOrden`.
- Agregar rate limiting al endpoint público `/orders` (es la única ruta sin
  JWT de todo el sistema, por diseño, así que es el punto más expuesto).
- Endpoint `GET /orders/{id}` para que el frontend haga polling del estado
  de la orden mientras el webhook llega (Stripe no es instantáneo).
- Definir política de expiración de órdenes `pendiente` (ej. job que las
  pasa a `cancelada` tras 24h) para liberar `disponible_para_venta`.
- `go.sum` no está incluido — correr `go mod tidy` con acceso a internet.

## Variables de entorno

Ver `.env.example`.
