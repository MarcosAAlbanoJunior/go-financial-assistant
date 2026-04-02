ALTER TABLE purchases
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'EXPENSE'
        CHECK (kind IN ('EXPENSE', 'INCOME'));

ALTER TABLE purchases
    ADD CONSTRAINT chk_income_no_installment
        CHECK (kind != 'INCOME' OR type != 'INSTALLMENT');

CREATE INDEX idx_purchases_kind ON purchases(kind);
