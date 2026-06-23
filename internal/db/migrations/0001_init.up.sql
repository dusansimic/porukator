CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Client devices (Android phones) that send SMS.
CREATE TABLE clients (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name              TEXT NOT NULL,
    access_token_hash TEXT NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at      TIMESTAMPTZ
);

-- Credentials for upstream producer services.
CREATE TABLE api_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

-- Single-row global pacing configuration.
CREATE TABLE settings (
    id        INT PRIMARY KEY DEFAULT 1,
    delay_ms  INT NOT NULL DEFAULT 2000,
    jitter_ms INT NOT NULL DEFAULT 1000,
    CONSTRAINT settings_singleton CHECK (id = 1)
);
INSERT INTO settings (id, delay_ms, jitter_ms) VALUES (1, 2000, 1000);

CREATE TYPE message_status AS ENUM (
    'pending',
    'dispatched',
    'sent',
    'failed'
);

-- One row per SMS tracked by the service.
CREATE TABLE messages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id      UUID NOT NULL,
    phone_number  TEXT NOT NULL,
    content       TEXT NOT NULL,
    client_id     UUID REFERENCES clients(id) ON DELETE SET NULL,
    status        message_status NOT NULL DEFAULT 'pending',
    error         TEXT NOT NULL DEFAULT '',
    received_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispatched_at TIMESTAMPTZ,
    sent_at       TIMESTAMPTZ
);

-- Fast lookup of a client's still-pending jobs when it (re)connects.
CREATE INDEX messages_client_pending_idx
    ON messages (client_id, received_at)
    WHERE status = 'pending';

-- Message log ordering.
CREATE INDEX messages_received_at_idx ON messages (received_at DESC);
