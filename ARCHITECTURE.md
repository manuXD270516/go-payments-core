# ARCHITECTURE — go-payments-core

## Capas y regla de dependencia

Las dependencias apuntan **hacia adentro**. El dominio no conoce a nadie.

```
        cmd/api  (composición: wiring, arranque, shutdown)
             │
   internal/infrastructure  (adaptadores: pgx, chi, eventos, idempotencia)
             │  implementa los puertos
   internal/application/usecase  (orquesta el caso de uso; depende de PUERTOS)
             │  usa
   internal/domain  (entidades, value objects, invariantes, errores) ← sin deps externas
```

| Capa | Puede importar | NO puede importar |
|---|---|---|
| `domain` | stdlib básica | `net/http`, `database/sql`, pgx, chi, cualquier driver |
| `application` | `domain`, sus puertos | adaptadores concretos |
| `infrastructure` | `domain`, `application`, libs | — |
| `cmd` | todo (solo para componer) | — |

## Puertos (interfaces) — definidos por el consumidor

- `PaymentRepository` — persistencia de intents y transacciones.
- `LedgerRepository` — asientos contables.
- `Processor` — abstracción del procesador de pagos (mockeable).
- `IdempotencyStore` — registro y recuperación de respuestas idempotentes.
- `EventPublisher` — publicación de eventos de dominio (webhooks).

## Decisiones de arquitectura (ADR resumidas)

**ADR-001 — Dinero como entero.** `Money{Amount int64, Currency string}` en unidad mínima. Evita errores de redondeo de punto flotante, inaceptables en contabilidad.

**ADR-002 — Ledger de doble partida.** Cada movimiento produce asientos débito/crédito balanceados. Da auditabilidad y permite reconstruir saldos desde los asientos.

**ADR-003 — Idempotencia como capa transversal.** Middleware + tabla `idempotency_records` con hash del request. Los reintentos de red no duplican cobros.

**ADR-004 — Máquina de estados en el dominio.** Las transiciones válidas se declaran como datos y se validan en el dominio, no en el handler HTTP.

**ADR-005 — Concurrencia con bloqueo.** Operaciones sobre el mismo intent se serializan (`SELECT ... FOR UPDATE` o versión optimista) para evitar doble captura.

**ADR-006 — Errores RFC 7807.** Respuestas de error uniformes con código de dominio, para consumidores predecibles.

## Modelo de datos (resumen)

- `payment_intents` — id, merchant_id, amount, currency, status, method_ref, metadata, version, timestamps.
- `transactions` — id, intent_id, type, amount, provider_ref, result, timestamps.
- `ledger_entries` — id, transaction_id, account, direction, amount, currency, created_at.
- `idempotency_records` — key, merchant_id, request_hash, response_snapshot, status, created_at.
- `webhook_deliveries` — id, event_id, url, attempts, next_retry_at, status.

Índices: `(merchant_id, id)` en intents para aislamiento multitenant; único en `(merchant_id, idempotency_key)`.

## Observabilidad

`slog` estructurado con `trace_id`, redacción activa de campos sensibles, métricas de latencia y tasa de error por endpoint, health y readiness separados.
