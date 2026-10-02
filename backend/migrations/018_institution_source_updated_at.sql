-- Quando o Pluggy atualizou os dados do banco (lastUpdatedAt do item), diferente de quando o app sincronizou.
-- O Meu Pluggy atualiza sozinho (cerca de 1x/dia) ou quando a pessoa pede no app dele; a API não permite forçar.
ALTER TABLE institutions ADD COLUMN IF NOT EXISTS source_updated_at TIMESTAMPTZ;
