-- ID da transação na origem (Open Finance/Pluggy). Garante que sincronizar
-- de novo nunca duplica um lançamento. Lançamentos manuais ficam com NULL.
ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS external_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_external_id
    ON payments(external_id) WHERE external_id IS NOT NULL;
