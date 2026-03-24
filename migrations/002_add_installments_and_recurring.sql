-- Add recurring_expense_id to existing expenses table
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS recurring_expense_id UUID;

-- Installment purchases (compras parceladas)
CREATE TABLE IF NOT EXISTS installment_purchases (
    id                  UUID            PRIMARY KEY,
    description         TEXT            NOT NULL,
    total_amount        NUMERIC(10, 2)  NOT NULL,
    installment_amount  NUMERIC(10, 2)  NOT NULL,
    total_installments  INT             NOT NULL,
    category            VARCHAR(50)     NOT NULL,
    payment             VARCHAR(50)     NOT NULL DEFAULT 'CREDIT_CARD',
    purchase_date       TIMESTAMPTZ     NOT NULL,
    raw_input           TEXT            NOT NULL,
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- Individual installments (parcelas)
CREATE TABLE IF NOT EXISTS installments (
    id                  UUID            PRIMARY KEY,
    purchase_id         UUID            NOT NULL REFERENCES installment_purchases(id) ON DELETE CASCADE,
    installment_number  INT             NOT NULL,
    total_installments  INT             NOT NULL,
    amount              NUMERIC(10, 2)  NOT NULL,
    due_date            TIMESTAMPTZ     NOT NULL,
    paid_at             TIMESTAMPTZ,
    status              VARCHAR(20)     NOT NULL DEFAULT 'PENDING',
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_installments_purchase_id ON installments(purchase_id);
CREATE INDEX IF NOT EXISTS idx_installments_status ON installments(status);
CREATE INDEX IF NOT EXISTS idx_installments_due_date ON installments(due_date);

-- Recurring expenses (despesas recorrentes mensais)
CREATE TABLE IF NOT EXISTS recurring_expenses (
    id                   UUID            PRIMARY KEY,
    description          TEXT            NOT NULL,
    amount               NUMERIC(10, 2)  NOT NULL,
    category             VARCHAR(50)     NOT NULL,
    payment              VARCHAR(50)     NOT NULL,
    day_of_month         INT             NOT NULL DEFAULT 1,
    start_date           TIMESTAMPTZ     NOT NULL,
    end_date             TIMESTAMPTZ,
    is_active            BOOLEAN         NOT NULL DEFAULT TRUE,
    last_generated_date  TIMESTAMPTZ,
    cancelled_at         TIMESTAMPTZ,
    cancellation_reason  TEXT,
    raw_input            TEXT            NOT NULL,
    created_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recurring_expenses_is_active ON recurring_expenses(is_active);
