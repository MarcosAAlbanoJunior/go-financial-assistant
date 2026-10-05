-- Metas financeiras. Cada tipo usa só os seus campos (garantido pelos CHECKs):
--   SAVE    juntar target_amount até target_date (primeiro dia do mês);
--   CUT     gastar cut_percent% menos em category do que baseline (média dos meses anteriores à criação);
--   RESERVE manter reserve_months meses de despesas fixas.
-- O progresso não é gravado: é calculado na hora a partir de saldos, investimentos e gastos.
CREATE TABLE IF NOT EXISTS goals (
    id             UUID          PRIMARY KEY,
    kind           TEXT          NOT NULL CHECK (kind IN ('SAVE', 'CUT', 'RESERVE')),
    name           TEXT          NOT NULL CHECK (CHAR_LENGTH(name) BETWEEN 1 AND 60),
    target_amount  NUMERIC(14,2) CHECK (target_amount > 0),
    target_date    DATE,
    category       TEXT,
    cut_percent    INT           CHECK (cut_percent BETWEEN 1 AND 90),
    baseline       NUMERIC(14,2) CHECK (baseline > 0),
    reserve_months INT           CHECK (reserve_months BETWEEN 1 AND 36),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CHECK (kind <> 'SAVE'    OR (target_amount IS NOT NULL AND target_date IS NOT NULL)),
    CHECK (kind <> 'CUT'     OR (category IS NOT NULL AND cut_percent IS NOT NULL AND baseline IS NOT NULL)),
    CHECK (kind <> 'RESERVE' OR reserve_months IS NOT NULL)
);
