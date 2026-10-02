-- Histórico do Coach: cada análise validada (o JSON que a tela mostra, com os números calculados pelo app)
-- e as respostas que a pessoa deu às perguntas da IA. As últimas entram no contexto da próxima análise.
CREATE TABLE IF NOT EXISTS coach_analyses (
    id         UUID        PRIMARY KEY,
    month      DATE        NOT NULL,
    advice     JSONB       NOT NULL,
    answers    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS coach_analyses_month_idx ON coach_analyses (month, created_at DESC);
