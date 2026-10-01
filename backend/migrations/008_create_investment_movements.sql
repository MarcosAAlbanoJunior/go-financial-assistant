-- Movimentações (aplicações, resgates, taxas) de cada posição de investimento. Com elas é
-- possível estimar o saldo de meses anteriores à conexão, já que o Pluggy não informa saldos passados.
CREATE TABLE IF NOT EXISTS investment_movements (
    investment_id UUID          NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
    external_id   TEXT          NOT NULL,
    day           DATE          NOT NULL,
    amount        DECIMAL(14,2) NOT NULL, -- com sinal: positivo entra na posição (aplicação), negativo sai (resgate, taxa)
    PRIMARY KEY (investment_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_investment_movements_day ON investment_movements(investment_id, day);
