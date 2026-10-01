-- Correções manuais da classificação de despesas (fixa ou variável). A chave é a descrição
-- normalizada (minúsculas, sem números nem pontuação), a mesma que o dashboard usa para
-- reconhecer a mesma conta em meses diferentes. Sem regra, vale a detecção automática.
CREATE TABLE IF NOT EXISTS expense_rules (
    key        TEXT        PRIMARY KEY,
    class      TEXT        NOT NULL CHECK (class IN ('FIXED', 'VARIABLE')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
