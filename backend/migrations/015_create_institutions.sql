-- Instituições (bancos) das conexões do Open Finance: nome oficial, cor de marca e logo em cache,
-- servido pelo próprio app. O logo vem do conector do Pluggy e não é versionado no repositório.
CREATE TABLE IF NOT EXISTS institutions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id         TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL,
    color           TEXT        CHECK (color ~ '^[0-9a-fA-F]{6}$'),
    logo            BYTEA,
    logo_mime       TEXT        CHECK (logo_mime IN ('image/png', 'image/jpeg', 'image/webp', 'image/svg+xml')),
    logo_checked_at TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((logo IS NULL) = (logo_mime IS NULL))
);

-- Dados extras de contas e cartões. Todos opcionais: dependem do que o banco informa.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS institution_id        UUID REFERENCES institutions(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS brand                 TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS close_date            DATE,
    ADD COLUMN IF NOT EXISTS due_date              DATE,
    ADD COLUMN IF NOT EXISTS minimum_payment       DECIMAL(14,2),
    ADD COLUMN IF NOT EXISTS auto_invested_balance DECIMAL(14,2);
