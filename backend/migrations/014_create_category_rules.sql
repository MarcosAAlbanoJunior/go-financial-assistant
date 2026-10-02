-- Regras de categoria por conta (a mesma descrição normalizada de expense_rules). Ao classificar uma conta
-- (ex.: um Pix recorrente como Moradia), o app reclassifica os lançamentos que estavam em "Outros" e aplica
-- a regra nas próximas sincronizações. Regra com category OTHER significa "manter em Outros": a conta não
-- volta a ser perguntada.
CREATE TABLE IF NOT EXISTS category_rules (
    key        TEXT        PRIMARY KEY,
    category   TEXT        NOT NULL CHECK (category IN
        ('FOOD', 'MARKET', 'TRANSPORT', 'HEALTH', 'ENTERTAINMENT', 'SHOPPING', 'HOUSING', 'BILLS', 'EDUCATION', 'PEOPLE', 'OTHER')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
