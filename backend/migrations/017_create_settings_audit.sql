-- Histórico de alterações das configurações do dashboard. Nunca guarda valores (podem ser segredos), só o que mudou e de onde.
CREATE TABLE IF NOT EXISTS settings_audit (
    id        BIGSERIAL   PRIMARY KEY,
    at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action    TEXT        NOT NULL CHECK (action IN ('set', 'reset')),
    key       TEXT        NOT NULL CHECK (key ~ '^[A-Z][A-Z0-9_]{1,63}$'),
    sensitive BOOLEAN     NOT NULL DEFAULT FALSE,
    ip        TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_settings_audit_at ON settings_audit (at DESC);
