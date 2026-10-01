CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE purchases (
    id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    description         TEXT,
    category            TEXT          NOT NULL DEFAULT 'OTHER',
    payment_method      TEXT          NOT NULL DEFAULT 'OTHER',
    type                TEXT          NOT NULL CHECK (type IN ('SINGLE', 'INSTALLMENT', 'RECURRING')),
    total_amount        DECIMAL(12,2) NOT NULL CHECK (total_amount > 0),
    installment_count   INT           CHECK (installment_count > 0),
    installment_amount  DECIMAL(12,2) CHECK (installment_amount > 0),
    day_of_month        INT           CHECK (day_of_month BETWEEN 1 AND 31),
    is_active           BOOLEAN       NOT NULL DEFAULT TRUE,
    cancelled_at        TIMESTAMPTZ,
    cancellation_reason TEXT,
    raw_input           TEXT          NOT NULL,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE payments (
    id                 UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_id        UUID          NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    amount             DECIMAL(12,2) NOT NULL CHECK (amount > 0),
    status             TEXT          NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID', 'CANCELLED')),
    installment_number INT,
    due_date           DATE,
    reference_month    DATE,
    paid_at            TIMESTAMPTZ,
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_purchase_id ON payments(purchase_id);
CREATE INDEX idx_payments_status      ON payments(status);
CREATE INDEX idx_purchases_type       ON purchases(type);
CREATE INDEX idx_purchases_is_active  ON purchases(is_active) WHERE type = 'RECURRING';
