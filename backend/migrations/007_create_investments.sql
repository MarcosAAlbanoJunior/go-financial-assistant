-- Posições de investimento do Open Finance (saldo real). Guarda só o necessário:
-- tipo, subtipo, nome do produto e saldos; nunca titular, CNPJ do emissor nem número da posição.
CREATE TABLE IF NOT EXISTS investments (
    id          UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT          NOT NULL UNIQUE,
    item_id     TEXT          NOT NULL,
    type        TEXT          NOT NULL,
    subtype     TEXT          NOT NULL DEFAULT '',
    name        TEXT          NOT NULL,
    balance     DECIMAL(14,2) NOT NULL DEFAULT 0, -- saldo líquido atual
    amount      DECIMAL(14,2) NOT NULL DEFAULT 0, -- valor bruto
    active      BOOLEAN       NOT NULL DEFAULT TRUE, -- FALSE quando a posição some do Pluggy (resgatada)
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Um saldo por posição por dia: é o histórico que permite desenhar a progressão do patrimônio.
CREATE TABLE IF NOT EXISTS investment_balances (
    investment_id UUID          NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
    day           DATE          NOT NULL,
    balance       DECIMAL(14,2) NOT NULL,
    PRIMARY KEY (investment_id, day)
);
