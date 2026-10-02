package db

import (
	"context"
	"fmt"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func (r *PostgresPurchaseRepository) ExpenseKeyMonths(ctx context.Context, from, to time.Time) ([]ports.ExpenseKeyMonth, error) {
	query := `
		SELECT ` + expenseKey + ` AS key,
		       (ARRAY_AGG(` + cleanDescription + ` ORDER BY pay.created_at DESC))[1] AS label,
		       (ARRAY_AGG(p.category ORDER BY pay.created_at DESC))[1] AS category,
		       ` + paymentMonth + ` AS month,
		       SUM(pay.amount), COUNT(*),
		       EXTRACT(DAY FROM MAX(COALESCE(pay.due_date, pay.reference_month, pay.created_at::date)))::int,
		       BOOL_AND(pay.status = 'PAID'),
		       BOOL_OR(p.type = 'INSTALLMENT' OR COALESCE(p.description, '') ~* '(\(\s*\d+\s*/\s*\d+\s*\)|parc[a-z.]*\s*\d+\s*/\s*\d+)'),
		       BOOL_OR(p.type = 'RECURRING')
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'EXPENSE' AND pay.status != 'CANCELLED' AND p.description IS NOT NULL
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		GROUP BY 1, 4
		HAVING ` + expenseKey + ` <> ''
		ORDER BY 4, 5 DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar despesas por conta: %w", err)
	}
	defer rows.Close()

	var result []ports.ExpenseKeyMonth
	for rows.Next() {
		var k ports.ExpenseKeyMonth
		if err := rows.Scan(&k.Key, &k.Label, &k.Category, &k.Month, &k.Total, &k.Count, &k.Day, &k.AllPaid, &k.Installment, &k.Recurring); err != nil {
			return nil, fmt.Errorf("erro ao escanear despesas por conta: %w", err)
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) ExpenseRules(ctx context.Context) (map[string]ports.ExpenseClass, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT key, class FROM expense_rules`)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler regras de despesas: %w", err)
	}
	defer rows.Close()

	rules := map[string]ports.ExpenseClass{}
	for rows.Next() {
		var key, class string
		if err := rows.Scan(&key, &class); err != nil {
			return nil, fmt.Errorf("erro ao escanear regra de despesa: %w", err)
		}
		rules[key] = ports.ExpenseClass(class)
	}
	return rules, rows.Err()
}

func (r *PostgresPurchaseRepository) SetExpenseRule(ctx context.Context, key string, class ports.ExpenseClass) error {
	var err error
	if class == "" {
		_, err = r.db.Pool.Exec(ctx, `DELETE FROM expense_rules WHERE key = $1`, key)
	} else {
		_, err = r.db.Pool.Exec(ctx, `
			INSERT INTO expense_rules (key, class, updated_at) VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET class = EXCLUDED.class, updated_at = NOW()
		`, key, string(class))
	}
	if err != nil {
		return fmt.Errorf("erro ao gravar regra de despesa: %w", err)
	}
	return nil
}

func (r *PostgresPurchaseRepository) KnownInstallments(ctx context.Context, from, to time.Time) (map[time.Time]float64, error) {
	query := `
		SELECT ` + paymentMonth + ` AS month, SUM(pay.amount)
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'EXPENSE' AND p.type = 'INSTALLMENT' AND pay.status != 'CANCELLED'
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		GROUP BY 1
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar parcelas futuras: %w", err)
	}
	defer rows.Close()

	result := map[time.Time]float64{}
	for rows.Next() {
		var m time.Time
		var total float64
		if err := rows.Scan(&m, &total); err != nil {
			return nil, fmt.Errorf("erro ao escanear parcelas futuras: %w", err)
		}
		result[m.UTC()] = total
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) IncomePayments(ctx context.Context, from, to time.Time) ([]ports.IncomePayment, error) {
	query := `
		SELECT CASE WHEN p.category = 'SALARY' THEN 'salario' ELSE ` + expenseKey + ` END AS key,
		       CASE WHEN p.category = 'SALARY' THEN 'Salário' ELSE p.description END, ` + paymentMonth + ` AS month, pay.amount
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE p.kind = 'INCOME' AND pay.status != 'CANCELLED' AND p.description IS NOT NULL
		  AND ` + paymentMonth + ` BETWEEN $1::date AND $2::date
		  AND ` + expenseKey + ` <> ''
		ORDER BY 3, 4 DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar entradas de renda: %w", err)
	}
	defer rows.Close()

	var result []ports.IncomePayment
	for rows.Next() {
		var i ports.IncomePayment
		if err := rows.Scan(&i.Key, &i.Label, &i.Month, &i.Amount); err != nil {
			return nil, fmt.Errorf("erro ao escanear entrada de renda: %w", err)
		}
		result = append(result, i)
	}
	return result, rows.Err()
}
