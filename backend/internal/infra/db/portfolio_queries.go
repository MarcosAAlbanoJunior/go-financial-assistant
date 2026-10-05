package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PortfolioRepo) Positions(ctx context.Context) ([]domain.Position, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, type, subtype, name, balance, amount, updated_at
		FROM investments
		WHERE active
		ORDER BY balance DESC, name
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar posições: %w", err)
	}
	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Position, error) {
		var p domain.Position
		err := row.Scan(&p.ID, &p.Type, &p.Subtype, &p.Name, &p.Balance, &p.Amount, &p.UpdatedAt)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear posições: %w", err)
	}
	return result, nil
}

func (r *PortfolioRepo) PortfolioHistory(ctx context.Context, from, to time.Time) ([]domain.PortfolioMonth, error) {
	// Do mês da primeira sincronização em diante o saldo é o exato gravado; antes, é estimado pelas movimentações
	// (sem rendimentos). Detalhes em docs/regras-de-calculo.md (Investimentos).
	query := `
		WITH first_snap AS (
			SELECT investment_id, MIN(day) AS day FROM investment_balances GROUP BY investment_id
		), ref AS (
			SELECT f.investment_id, f.day, ib.balance
			FROM first_snap f
			JOIN investment_balances ib ON ib.investment_id = f.investment_id AND ib.day = f.day
		), cutoff AS (
			SELECT DATE_TRUNC('month', MIN(day))::date AS month FROM first_snap
		)
		SELECT s.month::date,
		       CASE WHEN s.month::date < (SELECT month FROM cutoff) THEN est.balance ELSE ex.balance END,
		       COALESCE(s.month::date < (SELECT month FROM cutoff), FALSE)
		FROM generate_series($1::date, $2::date, '1 month') AS s(month)
		LEFT JOIN LATERAL (
			SELECT SUM(x.balance) AS balance FROM (
				SELECT DISTINCT ON (ib.investment_id) ib.balance
				FROM investment_balances ib
				WHERE ib.day < s.month + INTERVAL '1 month'
				ORDER BY ib.investment_id, ib.day DESC
			) x
		) ex ON TRUE
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0, r.balance - COALESCE((
				SELECT SUM(m.amount) FROM investment_movements m
				WHERE m.investment_id = r.investment_id
				  AND m.day >= s.month + INTERVAL '1 month' AND m.day <= r.day
			), 0))) AS balance
			FROM ref r
			WHERE EXISTS (
				SELECT 1 FROM investment_movements m
				WHERE m.investment_id = r.investment_id AND m.day < s.month + INTERVAL '1 month'
			)
		) est ON TRUE
		ORDER BY s.month
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar histórico do patrimônio: %w", err)
	}
	defer rows.Close()

	var result []domain.PortfolioMonth
	for rows.Next() {
		var m domain.PortfolioMonth
		if err := rows.Scan(&m.Month, &m.Balance, &m.Estimated); err != nil {
			return nil, fmt.Errorf("erro ao escanear histórico do patrimônio: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
