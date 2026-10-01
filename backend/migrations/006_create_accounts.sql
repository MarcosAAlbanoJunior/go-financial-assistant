-- Contas e cartões vindos do Open Finance. Guarda só o necessário para o dashboard:
-- nunca CPF, nome do titular nem o número completo da conta (apenas os 4 últimos dígitos).
CREATE TABLE IF NOT EXISTS accounts (
    id                     UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id            TEXT          NOT NULL UNIQUE,
    item_id                TEXT          NOT NULL,
    type                   TEXT          NOT NULL CHECK (type IN ('BANK', 'CREDIT')),
    name                   TEXT          NOT NULL,
    last4                  TEXT          NOT NULL DEFAULT '',
    balance                DECIMAL(14,2) NOT NULL DEFAULT 0,
    credit_limit           DECIMAL(14,2),
    available_credit_limit DECIMAL(14,2),
    updated_at             TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Conta de origem do pagamento; NULL em lançamentos manuais.
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_payments_account_id ON payments(account_id) WHERE account_id IS NOT NULL;
