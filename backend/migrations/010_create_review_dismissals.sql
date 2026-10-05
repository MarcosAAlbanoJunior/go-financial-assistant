-- Sugestões da tela de revisão que a pessoa dispensou. A chave é a descrição normalizada da conta
-- (a mesma de expense_rules) ou, nos aumentos, a categoria. Sem linha, a sugestão volta a aparecer.
CREATE TABLE IF NOT EXISTS review_dismissals (
    kind         TEXT        NOT NULL CHECK (kind IN ('INCREASE', 'FIXED', 'ANT', 'DUPLICATE', 'NEW')),
    key          TEXT        NOT NULL,
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (kind, key)
);
