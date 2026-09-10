# go-payments-core

Núcleo transaccional de una pasarela de pagos, escrito en Go con arquitectura hexagonal.

Implementa el ciclo de vida de una transacción (autorización, captura, reembolso, anulación) con **idempotencia extremo a extremo**, **ledger de doble partida** y aislamiento **multitenant** por comercio. Soporta transacciones con tarjeta (vía token) y sin tarjeta (transferencia, efectivo, QR).

## Por qué existe

Un cobro duplicado o un saldo que no cuadra no son bugs cualquiera: son dinero real. Este servicio trata la corrección contable y la resistencia a reintentos como requisitos de primera clase, no como detalles de implementación.

## Características

- Máquina de estados explícita con transiciones validadas en el dominio
- Idempotencia por `Idempotency-Key` en todos los endpoints de escritura
- Ledger de doble partida: los asientos de cada transacción siempre balancean
- Dinero como entero en unidad mínima + moneda ISO 4217 (nunca punto flotante)
- Webhooks con reintentos, backoff exponencial y DLQ
- Errores en formato RFC 7807 con códigos de dominio
- Sin datos sensibles de tarjeta persistidos ni logueados

## Stack

Go 1.22+ · PostgreSQL (pgx) · chi · migraciones versionadas · testcontainers-go · slog · Docker

## Arranque rápido

```bash
make up        # levanta Postgres
make migrate   # aplica migraciones
make run       # corre la API
make test      # tests con -race
```

## Documentación

- [`SPEC.md`](SPEC.md) — especificación funcional y criterios de aceptación
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — capas, puertos y decisiones de arquitectura
- [`ARCHITECT-PROMPT.md`](ARCHITECT-PROMPT.md) — prompt de arquitecto para desarrollo asistido

## Estado

En construcción, por etapas, siguiendo `SPEC.md`.
