ALTER TABLE purchases
    ADD COLUMN IF NOT EXISTS transfer_direction TEXT
        CHECK (transfer_direction IN ('IN', 'OUT'));
