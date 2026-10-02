package db

import (
	"context"
	"fmt"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresPurchaseRepository) UncategorizedExpenses(ctx context.Context, limit int) ([]domain.UncategorizedGroup, error) {
	query := `
		WITH g AS (
			SELECT ` + expenseKey + ` AS key,
			       (ARRAY_AGG(` + cleanDescription + ` ORDER BY pay.created_at DESC))[1] AS label,
			       COUNT(*) AS n, SUM(pay.amount) AS total, MAX(` + txDate + `) AS last
			FROM payments pay
			JOIN purchases p ON p.id = pay.purchase_id
			WHERE p.kind = 'EXPENSE' AND p.category = 'OTHER' AND pay.status != 'CANCELLED' AND p.description IS NOT NULL
			GROUP BY 1
		)
		SELECT key, label, n, total, last FROM g
		WHERE key <> '' AND key NOT IN (SELECT key FROM category_rules)
		ORDER BY total DESC, key
		LIMIT $1
	`
	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar despesas em Outros: %w", err)
	}
	defer rows.Close()

	var result []domain.UncategorizedGroup
	for rows.Next() {
		var g domain.UncategorizedGroup
		if err := rows.Scan(&g.Key, &g.Label, &g.Count, &g.Total, &g.Last); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesa em Outros: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) SetCategoryRule(ctx context.Context, key, category string) (int64, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("erro ao gravar regra de categoria: %w", err)
	}
	defer tx.Rollback(ctx)

	previous := "OTHER" // sem regra anterior, só o que está em Outros muda
	if err := tx.QueryRow(ctx, `SELECT category FROM category_rules WHERE key = $1`, key).Scan(&previous); err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("erro ao ler regra de categoria: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO category_rules (key, category) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET category = EXCLUDED.category, updated_at = NOW()
	`, key, category); err != nil {
		return 0, fmt.Errorf("erro ao gravar regra de categoria: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE purchases p SET category = $2
		WHERE p.kind = 'EXPENSE' AND `+expenseKey+` = $1 AND p.category IN ('OTHER', $3) AND p.category <> $2
	`, key, category, previous)
	if err != nil {
		return 0, fmt.Errorf("erro ao reclassificar despesas: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("erro ao gravar regra de categoria: %w", err)
	}
	return tag.RowsAffected(), nil
}
