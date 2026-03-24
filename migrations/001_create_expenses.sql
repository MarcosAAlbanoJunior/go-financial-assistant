-- migrations/001_create_expenses.sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS expenses (
    id          UUID PRIMARY KEY,
    amount      NUMERIC(10, 2)  NOT NULL,
    description TEXT            NOT NULL,
    category    VARCHAR(50)     NOT NULL,
    payment     VARCHAR(50)     NOT NULL,
    receipt_url TEXT,
    raw_input   TEXT            NOT NULL,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);