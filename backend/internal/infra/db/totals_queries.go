package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (r *PostgresPurchaseRepository) MonthlyTotals(ctx context.Context, from, to time.Time) ([]ports.MonthTotals, error) {
	query := `
		WITH ` + monthlyCTE + `
		SELECT s.month::date, COALESCE(m.income, 0), COALESCE(m.expense, 0), COALESCE(m.applied, 0), COALESCE(m.redeemed, 0)
		FROM generate_series($1::date, $2::date, '1 month') AS s(month)
		LEFT JOIN monthly m ON m.month = s.month::date
		ORDER BY s.month
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar totais mensais: %w", err)
	}
	defer rows.Close()

	var result []ports.MonthTotals
	for rows.Next() {
		var m ports.MonthTotals
		if err := rows.Scan(&m.Month, &m.Income, &m.Expense, &m.Applied, &m.Redeemed); err != nil {
			return nil, fmt.Errorf("erro ao escanear totais mensais: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) InvestmentSeries(ctx context.Context, from, to time.Time) ([]ports.InvestmentMonth, error) {
	// O acumulado é calculado desde o primeiro lançamento de investimento e só depois
	// recortado na janela pedida.
	query := `
		WITH ` + monthlyCTE + `,
		first AS (SELECT MIN(month) AS month FROM monthly WHERE applied > 0 OR redeemed > 0)
		SELECT month, applied, redeemed, cumulative FROM (
			SELECT s.month::date AS month, COALESCE(m.applied, 0) AS applied, COALESCE(m.redeemed, 0) AS redeemed,
			       SUM(COALESCE(m.applied, 0) - COALESCE(m.redeemed, 0)) OVER (ORDER BY s.month) AS cumulative
			FROM generate_series((SELECT month FROM first), $2::date, '1 month') AS s(month)
			LEFT JOIN monthly m ON m.month = s.month::date
		) t
		WHERE month >= $1::date
		ORDER BY month
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar investimentos: %w", err)
	}
	defer rows.Close()

	var result []ports.InvestmentMonth
	for rows.Next() {
		var m ports.InvestmentMonth
		if err := rows.Scan(&m.Month, &m.Applied, &m.Redeemed, &m.Cumulative); err != nil {
			return nil, fmt.Errorf("erro ao escanear investimentos: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) ExpenseBreakdown(ctx context.Context, month time.Time, by ports.BreakdownDimension) ([]ports.BreakdownItem, error) {
	// key/name vêm de fragmentos fixos: nada do que o cliente envia entra no SQL.
	var key, name string
	switch by {
	case ports.BreakdownByCategory:
		key, name = "p.category", "p.category"
	case ports.BreakdownByPaymentMethod:
		key, name = "p.payment_method", "p.payment_method"
	case ports.BreakdownByAccount:
		key, name = "COALESCE(a.id::text, '')", "COALESCE(NULLIF(a.name || ' ' || a.last4, ''), '')"
	default:
		return nil, fmt.Errorf("agrupamento inválido: %q", by)
	}

	query := `
		SELECT ` + key + ` AS key, ` + name + ` AS name, SUM(pay.amount) AS total
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		WHERE ` + paymentMonth + ` = $1::date
		  AND pay.status != 'CANCELLED'
		  AND p.kind = 'EXPENSE'
		GROUP BY 1, 2
		ORDER BY total DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, month)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas agrupadas: %w", err)
	}
	defer rows.Close()

	var result []ports.BreakdownItem
	for rows.Next() {
		var it ports.BreakdownItem
		if err := rows.Scan(&it.Key, &it.Name, &it.Total); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesas agrupadas: %w", err)
		}
		result = append(result, it)
	}
	return result, rows.Err()
}
