# kajve · payment-service

Documentación técnica del microservicio de pagos de **kajve**, escrito en Go con **arquitectura hexagonal** (puertos y adaptadores). Este servicio gestiona el catálogo de productos (camas de café y planes de suscripción), el flujo de checkout con **Stripe**, las órdenes de compra y la publicación de eventos de dominio hacia el resto del sistema kajve vía **RabbitMQ**.

---

## 1. Idea general de la arquitectura

El servicio sigue el patrón **hexagonal (Ports & Adapters)**:

```
                     ┌────────────────────────────┐
   HTTP Request  ──▶ │   controllers (adaptador    │
                     │   de entrada)                │
                     └──────────────┬──────────────┘
                                    │ llama a
                     ┌──────────────▼──────────────┐
                     │   usecases (aplicación)      │──▶ NO conoce Postgres
                     │   orquesta reglas de negocio │    ni Stripe directamente
                     └───┬───────────────────┬──────┘
                         │ depende de puertos │
              ┌──────────▼──────┐   ┌─────────▼─────────┐
              │ OrderRepository │   │  PaymentGateway    │  ← interfaces (ports/)
              │  (interface)    │   │   (interface)       │
              └──────────┬──────┘   └─────────┬───────────┘
                         │ implementado por    │ implementado por
              ┌──────────▼──────┐   ┌─────────▼───────────┐
              │ PostgresOrder   │   │  StripeGateway        │
              │ Repository      │   │  (SDK de Stripe)      │
              └─────────────────┘   └───────────────────────┘
```

**Regla de oro:** el dominio y los casos de uso solo conocen **interfaces** (`ports.OrderRepository`, `ports.PaymentGateway`, `ports.EventPublisher`). Las implementaciones concretas (Postgres, Stripe, RabbitMQ) viven en `infrastructure/` y se conectan únicamente en `main.go` (inyección de dependencias manual).

Esto permite, por ejemplo, cambiar Stripe por otra pasarela de pago reescribiendo solo `stripegateway/`, sin tocar la lógica de negocio.

---

## 2. Estructura de capas y para qué sirve cada archivo

### 2.1 `domain/entities/` — El corazón del negocio

| Archivo | Qué contiene | Para qué sirve |
|---|---|---|
| `producto.go` | Struct `Producto`, enum `TipoProducto` (`cama_cafe` \| `suscripcion`), método `EsVendible()` | Representa un renglón del catálogo. La regla de negocio "¿se puede comprar ahora?" vive aquí: para `cama_cafe` exige `Stock > 0`, para `suscripcion` basta con `Activo == true`. |
| `orden.go` | Structs `Comprador`, `Orden`, `OrdenConComprador`, enum `EstadoOrden` | Representa la compra de un producto. `Comprador` es **guest checkout** (no requiere cuenta kajve). `Orden.IDUsuario` solo se llena si la compra es de tipo suscripción, porque ahí sí hace falta saber a qué cuenta activarle el plan. |
| `lote.go` | `EventoOsilVendido`, todos los `errors.New(...)` del dominio | Define el evento de dominio que se publica a RabbitMQ cuando se vende un lote físico, y centraliza los errores tipados (`ErrProductoNoEncontrado`, `ErrUsuarioRequerido`, etc.) que los casos de uso y controllers usan con `errors.Is()`. |

### 2.2 `domain/ports/` — Los "enchufes" del hexágono

| Archivo | Interface | Métodos clave | Quién la implementa |
|---|---|---|---|
| `repository.go` | `OrderRepository` | CRUD de catálogo, `CrearOrden`, `MarcarOrdenPagada`, idempotencia de webhooks, gestión de órdenes admin | `PostgresOrderRepository` |
| `payment_gateway.go` | `PaymentGateway` | `CrearCliente`, `CrearSesionPago`, `VerificarYParsearWebhook` | `StripeGateway` |
| `event_publisher.go` | `EventPublisher` | `PublicarOsilVendido` | `RabbitMQPublisher` |

Estas interfaces son el contrato que la capa de aplicación usa; nunca importan paquetes de infraestructura.

### 2.3 `application/usecases/` — Orquestación de reglas de negocio

| Archivo | Casos de uso | Qué hace |
|---|---|---|
| `catalogo_usecases.go` | `CrearProductoUseCase`, `ListarProductosUseCase`, `ObtenerProductoUseCase`, `ActualizarProductoUseCase`, `EliminarProductoUseCase` | CRUD completo del catálogo (camas de café y planes). Valida que los campos requeridos existan según el `TipoProducto` (ej. `PesoKg` para camas, `PlanSuscripcion`+`DuracionDias` para suscripciones). |
| `crear_orden.go` | `CrearOrdenUseCase` | **Flujo de checkout completo:** valida que el producto sea vendible → crea cliente en Stripe → registra comprador (guest) → crea la orden en estado `pendiente` → crea la sesión de pago en Stripe → guarda el `session_id`. Funciona igual para cama de café o suscripción. |
| `gestionar_ordenes.go` | `ListarOrdenesUseCase`, `ObtenerOrdenUseCase`, `ActualizarEstadoOrdenUseCase` | Vista y administración de órdenes para el panel admin. `ActualizarEstadoOrdenUseCase` valida que el nuevo estado esté dentro del set permitido (`pendiente`, `pagada`, `cancelada`, `reembolsada`). |
| `procesar_webhook.go` | `ProcesarWebhookUseCase` | Recibe el webhook de Stripe ya parseado, aplica **idempotencia** por `event.ID` (si ya se procesó, no hace nada), marca la orden como pagada, y si la compra involucró un lote físico, publica el evento `osil.vendido` a RabbitMQ. |

### 2.4 `infrastructure/` — Adaptadores concretos

| Archivo | Rol |
|---|---|
| `config/config.go` | Carga variables de entorno (`DATABASE_URL`, claves de Stripe, URL de RabbitMQ) y valida que las obligatorias existan al arrancar. |
| `controllers/orders_controller.go` | Adaptador HTTP público: recibe `POST /orders`, valida el payload y llama a `CrearOrdenUseCase`. |
| `controllers/webhooks_controller.go` | Recibe `POST /webhooks/stripe`, lee el body crudo (necesario para verificar firma), lo pasa a `ProcesarWebhookUseCase`. |
| `controllers/orders_admin_controller.go` | Endpoints admin de solo-gestión de órdenes (listar, detalle, cambiar estado). |
| `controllers/catalog_admin_controller.go` | CRUD HTTP del catálogo, expone también las rutas públicas de solo lectura (`/catalogo`). |
| `repository/postgres_order_repository.go` | Implementación real de `OrderRepository` con `pgx`. Contiene la lógica transaccional más delicada del servicio (ver sección 4). |
| `stripegateway/stripe_gateway.go` | Único archivo que importa el SDK de Stripe. Traduce eventos de Stripe a `CheckoutSessionEvent` (tipo agnóstico) para que el resto del sistema no dependa de tipos de Stripe. |
| `messaging/rabbitmq_publisher.go` | Publica `EventoOsilVendido` al exchange `kajve.events` con routing key `osil.vendido`. |
| `routes/routes.go` | Arma el árbol de rutas con `chi`, aplica CORS y monta los grupos `/admin`. |
| `main.go` | Composition root: crea las implementaciones concretas, las inyecta en los casos de uso, y arranca el servidor HTTP en el puerto configurado (default `8000`). |

---

## 3. Endpoints HTTP

Base del router: `chi`, con middlewares `Logger`, `Recoverer` y CORS abierto (`AllowedOrigins: *`).

### Públicos (sin autenticación)

| Método | Ruta | Controller → Caso de uso | Descripción |
|---|---|---|---|
| `GET` | `/health` | — | Health check simple, responde `"ok"`. |
| `POST` | `/orders` | `OrdersController.CrearOrden` → `CrearOrdenUseCase` | Crea una orden de compra (guest checkout) para un producto del catálogo y devuelve la URL de Stripe Checkout. |
| `POST` | `/webhooks/stripe` | `WebhooksController.StripeWebhook` → `ProcesarWebhookUseCase` | Recibe eventos de Stripe (firmados), confirma pagos y dispara efectos (stock / suscripción / evento de dominio). |
| `GET` | `/catalogo` | `CatalogAdminController.ListarProductos` | Lista pública del catálogo (para la tienda). Soporta query params `tipo_producto`, `solo_activos`, `limit`, `offset`. |
| `GET` | `/catalogo/{id}` | `CatalogAdminController.ObtenerProducto` | Detalle público de un producto. |

> ⚠️ Nota: las rutas de catálogo público **reusan los mismos casos de uso** que el CRUD administrativo (solo los de lectura), no hay duplicación de lógica.

### Administrativos — `/admin/*`

> **Pendiente crítico (TODO explícito en el código):** ninguna ruta bajo `/admin` tiene middleware de autenticación todavía. Cualquiera que llegue a estos endpoints puede leer PII de compradores (`email`, `teléfono`) y modificar el catálogo/órdenes. Esto está marcado como tarea abierta tanto en `routes.go` como en los comentarios de los controllers.

| Método | Ruta | Controller → Caso de uso | Descripción |
|---|---|---|---|
| `GET` | `/admin/orders` | `OrdersAdminController.ListarOrdenes` | Lista órdenes con filtros `estado`, `id_lote`, `limit`, `offset`. Incluye datos del comprador (PII). |
| `GET` | `/admin/orders/{id}` | `OrdersAdminController.ObtenerOrden` | Detalle completo de una orden. |
| `PATCH` | `/admin/orders/{id}/estado` | `OrdersAdminController.ActualizarEstado` | Cambia el estado manualmente. Body: `{"estado": "cancelada"}`. Si se cancela una orden de `cama_cafe`, libera stock y vuelve a marcar el lote como disponible. |
| `POST` | `/admin/catalogo` | `CatalogAdminController.CrearProducto` | Alta de un producto (cama de café o plan de suscripción). |
| `GET` | `/admin/catalogo` | `CatalogAdminController.ListarProductos` | Listado admin (mismos filtros que el público). |
| `GET` | `/admin/catalogo/{id}` | `CatalogAdminController.ObtenerProducto` | Detalle admin. |
| `PUT` | `/admin/catalogo/{id}` | `CatalogAdminController.ActualizarProducto` | Edición completa del producto. |
| `DELETE` | `/admin/catalogo/{id}` | `CatalogAdminController.EliminarProducto` | Borrado físico. Si el producto ya tiene órdenes asociadas, la FK lo bloquea (mejor desactivar con `activo=false` en ese caso). |

---

## 4. Flujos completos del sistema

### 4.1 Flujo de compra (checkout)

```
Cliente/App          OrdersController      CrearOrdenUseCase        Postgres          Stripe
    │  POST /orders        │                       │                    │                │
    ├──────────────────────▶                       │                    │                │
    │                      ├───────────────────────▶                    │                │
    │                      │            1) ProductoVendible(idProducto) │                │
    │                      │                       ├────────────────────▶                │
    │                      │                       │◀────producto───────┤                │
    │                      │            2) valida EsVendible() y        │                │
    │                      │               (si es suscripción) IDUsuario│                │
    │                      │            3) CrearCliente(nombre,email)   │                │
    │                      │                       ├─────────────────────────────────────▶
    │                      │                       │◀──────customerID───────────────────┤
    │                      │            4) CrearComprador(...)          │                │
    │                      │                       ├────────────────────▶                │
    │                      │            5) CrearOrden(estado=pendiente) │                │
    │                      │                       ├────────────────────▶                │
    │                      │            6) CrearSesionPago(...)         │                │
    │                      │                       ├─────────────────────────────────────▶
    │                      │                       │◀────sessionID, checkoutURL──────────┤
    │                      │            7) ActualizarCheckoutSession()  │                │
    │                      │                       ├────────────────────▶                │
    │                      │◀───{id_orden, checkout_url}──┤                    │                │
    │◀─────────────────────┤                       │                    │                │
```

Puntos clave:
- Si el producto no existe → `404`. Si no es vendible (sin stock / inactivo) → `409`. Si es suscripción sin `id_usuario` → `400`.
- El comprador **no necesita cuenta en kajve** (guest checkout); solo las suscripciones enlazan con un `id_usuario` real.

### 4.2 Flujo de confirmación de pago (webhook de Stripe)

```
Stripe            WebhooksController    ProcesarWebhookUseCase       Postgres         RabbitMQ
  │ POST /webhooks/stripe  │                      │                     │                │
  ├────────────────────────▶                      │                     │                │
  │                        ├──────────────────────▶                     │                │
  │                        │        1) VerificarYParsearWebhook(firma)  │                │
  │                        │           (valida firma con webhookSecret) │                │
  │                        │        2) EventoYaProcesado(event.ID)?     │                │
  │                        │                     ├───────────────────────▶               │
  │                        │           si ya procesado → return nil (idempotencia)       │
  │                        │        3) RegistrarEventoWebhook(...)      │                │
  │                        │                     ├───────────────────────▶               │
  │                        │        4) MarcarOrdenPagada(session,intent)│                │
  │                        │                     ├───────────────────────▶               │
  │                        │                     │  (transacción: cama_cafe→stock-1      │
  │                        │                     │   o suscripcion→activa plan)          │
  │                        │                     │◀────idOrden, idLote──┤                │
  │                        │        5) si idLote != 0:                 │                │
  │                        │           PublicarOsilVendido(evento)     │                │
  │                        │                     ├────────────────────────────────────────▶
  │                        │        6) MarcarEventoProcesado(...)      │                │
  │                        │                     ├───────────────────────▶               │
  │                        │◀───── 200 OK ────────┤                     │                │
  │◀───────────────────────┤                      │                    │                │
```

**Lógica dentro de `MarcarOrdenPagada` (transacción única en Postgres):**

- Si `tipo_orden == cama_cafe`:
  1. Bloquea la fila del producto (`FOR UPDATE`).
  2. Descuenta 1 al stock; si llega a 0, marca el producto como `activo=false`.
  3. Si el producto tenía un lote asociado, marca `lotes_cafe.disponible_para_venta = FALSE` y registra un evento en `historial_eventos`.
- Si `tipo_orden == suscripcion`:
  1. Cancela cualquier suscripción previa activa del usuario (evita traslapes).
  2. Inserta la nueva suscripción con `fecha_fin = NOW() + duracion_dias`.
  3. No genera evento `osil.vendido` (idLote queda en 0).

**Idempotencia:** cada evento de Stripe llega con un `event.ID` único. Si ya se procesó, el caso de uso corta la ejecución antes de tocar nada más — esto es importante porque Stripe puede reintentar el mismo webhook varias veces.

**Nota de confiabilidad ya documentada en el código:** si la publicación a RabbitMQ falla después de marcar la orden como pagada, el pago **ya quedó registrado** en la base de datos (fuente de verdad), pero el evento se pierde. El propio código señala que falta un mecanismo de reintento/outbox para este caso.

### 4.3 Flujo de cancelación de orden (liberar disponibilidad)

Cuando un admin cambia el estado de una orden a `cancelada` vía `PATCH /admin/orders/{id}/estado`, y la orden es de tipo `cama_cafe`:

1. Se regresa `+1` al stock del producto y se reactiva (`activo = true`).
2. Si el producto tenía lote asociado, se vuelve a marcar `lotes_cafe.disponible_para_venta = TRUE`.

Esto solo ocurre si el estado anterior **no era ya** `cancelada` (evita liberar stock dos veces por error).

---

## 5. Modelo de datos (tablas involucradas, inferidas del código)

| Tabla | Uso |
|---|---|
| `catalogo_productos` | Catálogo unificado de camas de café y planes de suscripción (columnas compartidas + específicas nullable por tipo). |
| `compradores` | Guest checkout: nombre, email, teléfono, país, `stripe_customer_id`. |
| `ordenes` | Una orden por producto comprado; referencia `id_producto`, `id_comprador`, `id_usuario` (nullable), `id_lote` (heredado, solo trazabilidad). |
| `lotes_cafe` | Lotes físicos de café; se actualiza `disponible_para_venta` al vender/cancelar. |
| `suscripciones` | Historial de planes por usuario (`estado`: activa/cancelada, `fecha_inicio`, `fecha_fin`). |
| `stripe_webhook_events` | Tabla de idempotencia: registra cada `event.ID` de Stripe recibido y si ya fue procesado. |
| `historial_eventos` | Bitácora de eventos sobre un lote (ej. `osil_vendido`). |

Esto refleja el refactor reciente que separó `catalogo_productos` de `lotes_cafe`, desacoplando la tienda del manejo operativo de lotes.

---

## 6. Manejo de errores (mapeo a HTTP)

| Error de dominio | HTTP status | Dónde se usa |
|---|---|---|
| `ErrProductoNoEncontrado` | `404` | Crear orden, obtener/actualizar producto, obtener orden |
| `ErrProductoNoDisponible` | `409` | Crear orden (sin stock / inactivo) |
| `ErrProductoTipoInvalido` / `ErrDatosProductoInvalidos` | `400` | Crear producto |
| `ErrUsuarioRequerido` | `400` | Crear orden de suscripción sin `id_usuario` |
| `ErrOrdenNoEncontrada` | `404` | Obtener/actualizar orden |
| `ErrEstadoInvalido` | `400` | Actualizar estado de orden con valor fuera del set permitido |
| `ErrFirmaWebhookInvalida` | `400` | Webhook con firma inválida |
| cualquier otro error | `500` | Fallback genérico ("error interno...") |

---

## 7. Configuración (variables de entorno)

| Variable | Obligatoria | Default | Uso |
|---|---|---|---|
| `PORT` | No | `8000` | Puerto HTTP del servicio |
| `DATABASE_URL` | **Sí** | — | Conexión a Postgres (Neon) |
| `STRIPE_SECRET_KEY` | **Sí** | — | Clave secreta de Stripe |
| `STRIPE_WEBHOOK_SECRET` | **Sí** | — | Para verificar firma de webhooks |
| `STRIPE_SUCCESS_URL` | No | `https://kajve.com/orden/exito` | Redirección tras pago exitoso |
| `STRIPE_CANCEL_URL` | No | `https://kajve.com/orden/cancelada` | Redirección si cancela el checkout |
| `RABBITMQ_URL` | No | `amqp://guest:guest@rabbitmq:5672/` | Broker de eventos |
| `RABBITMQ_EXCHANGE` | No | `kajve.events` | Exchange donde se publica `osil.vendido` |

Si falta `DATABASE_URL`, `STRIPE_SECRET_KEY` o `STRIPE_WEBHOOK_SECRET`, el servicio no arranca (`config.Load()` regresa error y `main.go` hace `log.Fatalf`). Si RabbitMQ no está disponible, el servicio **sí arranca** en modo degradado (sin publicar eventos), solo lo reporta con un `log.Printf`.

---

## 8. Pendientes detectados en el código (TODOs explícitos)

1. **Autenticación de admin** — todas las rutas `/admin/*` (órdenes y catálogo) están sin proteger. Es el pendiente marcado como más urgente en `routes.go`.
2. **Outbox/reintento para eventos perdidos** — si `PublicarOsilVendido` falla tras un pago exitoso, el evento se pierde silenciosamente (el pago sí queda guardado).
3. **Reembolsos reales con Stripe** — actualmente `ActualizarEstadoOrden` solo cambia el estado en base de datos; no hay integración con la API de reembolsos de Stripe cuando el estado pasa a `reembolsada`.
4. **Webhooks locales** — para desarrollo local hace falta correr el Stripe CLI (`stripe listen --forward-to localhost:8000/webhooks/stripe`) ya que Stripe no puede alcanzar `localhost` directamente.

---

## 9. Resumen de responsabilidades por capa (una línea cada una)

- **`domain/entities`**: qué es un producto, una orden, un comprador — y sus reglas de negocio intrínsecas.
- **`domain/ports`**: los contratos que la aplicación necesita del mundo exterior (BD, pasarela de pago, bus de eventos).
- **`application/usecases`**: la orquestación paso a paso de cada operación de negocio, sin saber cómo se implementa nada externo.
- **`infrastructure/controllers`**: traducir HTTP ↔ casos de uso (parseo de JSON, códigos de estado).
- **`infrastructure/repository`**: implementación real con Postgres/pgx, incluida toda la lógica transaccional sensible (stock, suscripciones, idempotencia).
- **`infrastructure/stripegateway`**: único punto de contacto con el SDK de Stripe.
- **`infrastructure/messaging`**: publicación de eventos de dominio a RabbitMQ.
- **`infrastructure/routes`** y **`main.go`**: cablean todo junto (composition root) y levantan el servidor HTTP.