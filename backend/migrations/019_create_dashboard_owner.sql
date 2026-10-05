-- Setup pelo navegador (docs/specs/setup-inicial.md). Uma linha só em cada tabela (o app é de uma pessoa).

-- Dono do dashboard: o hash argon2id da senha (nunca o texto) e quando o setup terminou. A senha só vale para entrar
-- depois de concluído; sem hash aqui, vale a DASHBOARD_PASSWORD do ambiente.
CREATE TABLE IF NOT EXISTS dashboard_owner (
    id                 SMALLINT    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    password_hash      TEXT        CHECK (password_hash IS NULL OR password_hash LIKE '$argon2id$%'),
    setup_completed_at TIMESTAMPTZ,
    -- SETUP_REOPEN já usado: enquanto a variável continuar ligada, o setup não reabre de novo.
    reopen_done        BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Rascunho do setup: o que foi digitado nos passos ainda não confirmados. Nada daqui vale como configuração; ao
-- concluir, vira configuração ativa (settings) e o rascunho é apagado.
CREATE TABLE IF NOT EXISTS setup_draft (
    id                 SMALLINT    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    password_hash      TEXT        CHECK (password_hash IS NULL OR password_hash LIKE '$argon2id$%'),
    telegram_token     TEXT,                 -- cifrado com a APP_SECRET_KEY, como os segredos das configurações
    telegram_bot       TEXT,                 -- @ do bot, para a tela
    telegram_offset    BIGINT      NOT NULL DEFAULT 0, -- próximo update do Telegram a ler
    candidate_id       BIGINT,               -- quem mandou /start (ainda não confirmado)
    candidate_name     TEXT,
    candidate_username TEXT,
    candidate_accepted BOOLEAN     NOT NULL DEFAULT FALSE, -- "Sim, sou eu": o código foi pedido para este chat
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Os passos do setup entram no histórico de alterações (sem valores), com a ação "setup".
ALTER TABLE settings_audit DROP CONSTRAINT IF EXISTS settings_audit_action_check;
ALTER TABLE settings_audit ADD CONSTRAINT settings_audit_action_check CHECK (action IN ('set', 'reset', 'setup'));
