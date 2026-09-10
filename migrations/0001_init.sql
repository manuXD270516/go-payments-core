-- Esquema inicial del núcleo de pagos.
-- Las migraciones son versionadas y se aplican con "migrate up"; nunca se
-- sincroniza el esquema automáticamente en producción.

CREATE TABLE IF NOT EXISTS payment_intents (
    id              TEXT        PRIMARY KEY,
    merchant_id     TEXT        NOT NULL,
    amount          BIGINT      NOT NULL CHECK (amount >= 0),
    currency        CHAR(3)     NOT NULL,
    captured        BIGINT      NOT NULL DEFAULT 0 CHECK (captured >= 0),
    refunded        BIGINT      NOT NULL DEFAULT 0 CHECK (refunded >= 0),
    status          TEXT        NOT NULL,
    method_type     TEXT        NOT NULL,
    method_token    TEXT,
    provider_ref    TEXT,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    version         INTEGER     NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Invariantes de negocio garantizadas también por la base de datos.
    CONSTRAINT captured_no_excede_autorizado CHECK (captured <= amount),
    CONSTRAINT reembolso_no_excede_captura   CHECK (refunded <= captured)
);

-- El aislamiento multitenant se apoya en este índice: toda consulta filtra por comercio.
CREATE INDEX IF NOT EXISTS idx_intents_merchant ON payment_intents (merchant_id, id);
CREATE INDEX IF NOT EXISTS idx_intents_status   ON payment_intents (merchant_id, status);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id              TEXT        PRIMARY KEY,
    transaction_id  TEXT        NOT NULL,
    intent_id       TEXT        NOT NULL REFERENCES payment_intents (id),
    merchant_id     TEXT        NOT NULL,
    account         TEXT        NOT NULL,
    direction       TEXT        NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    currency        CHAR(3)     NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ledger_intent      ON ledger_entries (merchant_id, intent_id);
CREATE INDEX IF NOT EXISTS idx_ledger_transaction ON ledger_entries (transaction_id);

CREATE TABLE IF NOT EXISTS idempotency_records (
    merchant_id     TEXT        NOT NULL,
    key             TEXT        NOT NULL,
    request_hash    TEXT        NOT NULL,
    status_code     INTEGER,
    response        JSONB,
    completed       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (merchant_id, key)
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              TEXT        PRIMARY KEY,
    event_id        TEXT        NOT NULL,
    merchant_id     TEXT        NOT NULL,
    endpoint        TEXT        NOT NULL,
    attempts        INTEGER     NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
