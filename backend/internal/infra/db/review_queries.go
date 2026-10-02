package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func (r *PostgresPurchaseRepository) CategoryMonths(ctx context.Context, from, to time.Time) ([]domain.CategoryMonth, error) {
	query := `
		SELECT p.category, ` + paymentMonth + ` AS month, SUM(pay.amount)
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'EXPENSE' AND pay.status != 'CANCELLED'
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		GROUP BY 1, 2
		ORDER BY 2, 1
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas por categoria e mês: %w", err)
	}
	defer rows.Close()

	var result []domain.CategoryMonth
	for rows.Next() {
		var c domain.CategoryMonth
		if err := rows.Scan(&c.Category, &c.Month, &c.Total); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesa por categoria e mês: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) ExpensePayments(ctx context.Context, from, to time.Time) ([]domain.ExpensePayment, error) {
	query := `
		SELECT ` + expenseKey + ` AS key, ` + cleanDescription + `, p.category, p.payment_method, ` + txDate + ` AS day, pay.amount
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'EXPENSE' AND pay.status != 'CANCELLED' AND p.description IS NOT NULL
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		  AND ` + expenseKey + ` <> ''
		ORDER BY day, pay.created_at
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas: %w", err)
	}
	defer rows.Close()

	var result []domain.ExpensePayment
	for rows.Next() {
		var e domain.ExpensePayment
		if err := rows.Scan(&e.Key, &e.Label, &e.Category, &e.PaymentMethod, &e.Date, &e.Amount); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesa: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) Dismissals(ctx context.Context) ([]domain.Dismissal, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT kind, key FROM review_dismissals`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler sugestões dispensadas: %w", err)
	}
	defer rows.Close()

	var result []domain.Dismissal
	for rows.Next() {
		var d domain.Dismissal
		if err := rows.Scan(&d.Kind, &d.Key); err != nil {
			return nil, fmt.Errorf("erro ao escanear sugestão dispensada: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) SetDismissal(ctx context.Context, d domain.Dismissal, dismissed bool) error {
	var err error
	if dismissed {
		_, err = r.db.Pool.Exec(ctx, `INSERT INTO review_dismissals (kind, key) VALUES ($1, $2) ON CONFLICT DO NOTHING`, d.Kind, d.Key)
	} else {
		_, err = r.db.Pool.Exec(ctx, `DELETE FROM review_dismissals WHERE kind = $1 AND key = $2`, d.Kind, d.Key)
	}
	if err != nil {
		return fmt.Errorf("erro ao gravar sugestão dispensada: %w", err)
	}
	return nil
}

func (r *PostgresPurchaseRepository) Decisions(ctx context.Context) ([]domain.Decision, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT kind, key, label, category, decided_month, monthly FROM review_decisions ORDER BY decided_month, key`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler decisões: %w", err)
	}
	defer rows.Close()

	var result []domain.Decision
	for rows.Next() {
		var d domain.Decision
		if err := rows.Scan(&d.Kind, &d.Key, &d.Label, &d.Category, &d.Month, &d.Monthly); err != nil {
			return nil, fmt.Errorf("erro ao escanear decisão: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) SetDecision(ctx context.Context, d domain.Decision) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao gravar decisão: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO review_decisions (kind, key, label, category, decided_month, monthly) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (kind, key) DO UPDATE SET label = EXCLUDED.label, category = EXCLUDED.category,
		    decided_month = EXCLUDED.decided_month, monthly = EXCLUDED.monthly, decided_at = NOW()
	`, d.Kind, d.Key, d.Label, d.Category, d.Month, d.Monthly); err != nil {
		return fmt.Errorf("erro ao gravar decisão: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO review_dismissals (kind, key) VALUES ($1, $2) ON CONFLICT DO NOTHING`, d.Kind, d.Key); err != nil {
		return fmt.Errorf("erro ao dispensar sugestão decidida: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *PostgresPurchaseRepository) DeleteDecision(ctx context.Context, kind, key string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erro ao desfazer decisão: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{`DELETE FROM review_decisions WHERE kind = $1 AND key = $2`, `DELETE FROM review_dismissals WHERE kind = $1 AND key = $2`} {
		if _, err := tx.Exec(ctx, q, kind, key); err != nil {
			return fmt.Errorf("erro ao desfazer decisão: %w", err)
		}
	}
	return tx.Commit(ctx)
}
