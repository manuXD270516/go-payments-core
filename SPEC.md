# SPEC — go-payments-core
### Núcleo transaccional de una pasarela de pagos · Spec-Driven Development

> Esta especificación define **qué** debe construirse y **cómo se verifica**. No define el código. El agente implementa contra esta spec; los criterios de aceptación son el contrato.

## 1. Propósito

Servicio en Go que expone el núcleo transaccional de una pasarela de pagos: creación de intentos de pago, autorización, captura, reverso y reembolso, con registro contable de doble partida (ledger) e idempotencia extremo a extremo.

## 2. Alcance

### Entra
- Ciclo de vida de una transacción: `payment_intent` → `authorized` → `captured` → (`refunded` | `voided`).
- Transacciones **con tarjeta** (vía token, nunca PAN en claro) y **sin tarjeta** (transferencia, efectivo, QR).
- **Idempotencia** por `Idempotency-Key` en todos los endpoints de escritura.
- **Ledger de doble partida**: cada movimiento de dinero genera asientos balanceados (débito = crédito).
- **Webhooks** salientes con reintentos y backoff exponencial.
- **Multitenant**: aislamiento por `merchant_id` en datos y consultas.
- Máquina de estados explícita con transiciones válidas y rechazo de transiciones ilegales.

### No entra (fuera de alcance)
- Integración real con procesadores (se mockea vía interfaz `Processor`).
- Almacenamiento de datos de tarjeta (responsabilidad de `go-payment-methods-hub`).
- UI / backoffice.

## 3. Modelo de dominio

- **Money**: entero en la unidad mínima (centavos) + `Currency` ISO 4217. Nunca `float`.
- **PaymentIntent**: `id`, `merchant_id`, `amount (Money)`, `status`, `method_ref`, `metadata`, `created_at`.
- **Transaction**: operación sobre un intent (`authorize`, `capture`, `refund`, `void`) con `provider_ref` y resultado.
- **LedgerEntry**: `account`, `direction (debit|credit)`, `amount`, `transaction_id`. Los asientos de una transacción **siempre suman cero**.
- **IdempotencyRecord**: `key`, `merchant_id`, `request_hash`, `response_snapshot`, `status`.

## 4. Reglas de negocio (invariantes)

1. No se puede capturar un intent que no esté `authorized`.
2. No se puede reembolsar más que el monto capturado (`sum(refunds) <= captured`).
3. Toda transición de estado inválida devuelve `409 Conflict` con código de dominio.
4. Dos requests con la **misma** `Idempotency-Key` y mismo cuerpo devuelven la **misma** respuesta, sin re-ejecutar el efecto.
5. Misma `Idempotency-Key` con cuerpo distinto → `422 Unprocessable Entity`.
6. Todo movimiento monetario produce asientos de ledger balanceados; si el ledger no balancea, la operación **falla completa** (transacción de DB).
7. Ningún dato sensible de tarjeta se persiste ni se loguea.

## 5. API (contratos)

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/v1/payment-intents` | Crea un intent (requiere `Idempotency-Key`) |
| POST | `/v1/payment-intents/{id}/authorize` | Autoriza contra el procesador |
| POST | `/v1/payment-intents/{id}/capture` | Captura total o parcial |
| POST | `/v1/payment-intents/{id}/refund` | Reembolsa total o parcial |
| POST | `/v1/payment-intents/{id}/void` | Anula una autorización |
| GET | `/v1/payment-intents/{id}` | Consulta estado |
| GET | `/v1/payment-intents/{id}/ledger` | Asientos contables |
| GET | `/healthz` `/readyz` | Health y readiness |

Errores en formato **RFC 7807** (`application/problem+json`) con `type`, `title`, `status`, `detail`, `code` de dominio.

## 6. Arquitectura

Hexagonal / Clean, estricta:

```
cmd/api                     → composición y arranque
internal/domain/*           → entidades, value objects, reglas. CERO dependencias externas
internal/application/usecase→ casos de uso; dependen de INTERFACES (puertos)
internal/infrastructure/*   → adaptadores: Postgres, HTTP, eventos, idempotencia
internal/platform/config    → configuración por entorno
```

**Reglas:** el dominio no importa `database/sql`, ni `net/http`, ni el driver. Los casos de uso reciben puertos por inyección. Errores de dominio tipados (`errors.Is/As`), nunca strings.

## 7. Criterios de aceptación (verificables)

- [ ] `make up && make migrate && make run` levanta el servicio contra Postgres.
- [ ] `POST /v1/payment-intents` dos veces con la misma `Idempotency-Key` crea **un solo** intent y devuelve idéntica respuesta.
- [ ] Misma key con cuerpo distinto → `422`.
- [ ] Capturar un intent no autorizado → `409` con código `invalid_state_transition`.
- [ ] Reembolso mayor al capturado → `422` con código `refund_exceeds_captured`.
- [ ] Para toda transacción, `SUM(debit) == SUM(credit)` en `ledger_entries` (test de invariante).
- [ ] Un merchant no puede leer intents de otro merchant (test de aislamiento multitenant).
- [ ] Webhook fallido se reintenta con backoff y termina en DLQ tras N intentos.
- [ ] `go test ./...` en verde; cobertura del paquete `domain` ≥ 80%.
- [ ] `go vet ./...` y `golangci-lint run` sin hallazgos.
- [ ] Ningún log contiene PAN, CVV ni token en claro (test de redacción).
- [ ] Concurrencia: 50 capturas simultáneas del mismo intent producen **una** captura efectiva (test con `-race`).

## 8. Stack

Go 1.22+ · `net/http` con router ligero (chi) · PostgreSQL con `pgx` · migraciones versionadas (goose/migrate) · `testcontainers-go` para tests de integración · logging estructurado (`slog`) · OpenAPI 3 generado · Docker + docker-compose · Makefile.
