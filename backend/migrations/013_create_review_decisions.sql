-- "Cancelei": decisão da pessoa sobre uma sugestão recorrente da Revisão (fixa, gasto formiga ou conta nova).
-- O app confere nos meses seguintes se a cobrança sumiu e soma a economia realizada. monthly é quanto a conta
-- custava por mês quando a decisão foi tomada; label e category ficam guardados porque a conta deixa de aparecer.
CREATE TABLE IF NOT EXISTS review_decisions (
    kind          TEXT          NOT NULL CHECK (kind IN ('FIXED', 'ANT', 'NEW')),
    key           TEXT          NOT NULL,
    label         TEXT          NOT NULL,
    category      TEXT          NOT NULL,
    decided_month DATE          NOT NULL,
    monthly       NUMERIC(14,2) NOT NULL CHECK (monthly > 0),
    decided_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    PRIMARY KEY (kind, key)
);
