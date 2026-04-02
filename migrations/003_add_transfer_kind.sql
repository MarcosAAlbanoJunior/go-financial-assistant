-- Remove a constraint antiga que só permite EXPENSE e INCOME
ALTER TABLE purchases DROP CONSTRAINT IF EXISTS purchases_kind_check;

-- Adiciona a nova constraint incluindo TRANSFER
ALTER TABLE purchases
    ADD CONSTRAINT purchases_kind_check
        CHECK (kind IN ('EXPENSE', 'INCOME', 'TRANSFER'));

-- TRANSFER não pode ser parcelado
ALTER TABLE purchases
    ADD CONSTRAINT chk_transfer_no_installment
        CHECK (kind != 'TRANSFER' OR type != 'INSTALLMENT');
