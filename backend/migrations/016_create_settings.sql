-- Configurações editadas pela página do dashboard. O valor salvo aqui vale mais que a variável de ambiente.
-- Segredos (tokens, chaves) ficam cifrados (AES-256-GCM, chave mestra APP_SECRET_KEY, que só existe no ambiente).
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT        PRIMARY KEY CHECK (key ~ '^[A-Z][A-Z0-9_]{1,63}$'),
    value      TEXT        NOT NULL,
    secret     BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
