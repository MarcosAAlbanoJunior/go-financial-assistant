package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/jackc/pgx/v5"
)

// paymentMonth é o mês a que um pagamento pertence, igual ao usado nas demais consultas.
const paymentMonth = `DATE_TRUNC('month', COALESCE(pay.due_date, pay.reference_month, pay.created_at))::date`

// monthlyCTE agrega por mês as entradas, despesas e transferências (pagamentos cancelados ficam de fora).
const monthlyCTE = `
	monthly AS (
		SELECT ` + paymentMonth + ` AS month,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'INCOME'), 0)  AS income,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'EXPENSE'), 0) AS expense,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'OUT'), 0) AS applied,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER' AND p.transfer_direction = 'IN'), 0)  AS redeemed
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE pay.status != 'CANCELLED'
		GROUP BY 1
	)`

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

func (r *PostgresPurchaseRepository) Transactions(ctx context.Context, f ports.TransactionFilter) ([]ports.Transaction, int, error) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	where = append(where, "pay.status != 'CANCELLED'")
	if f.Month != nil {
		add(paymentMonth+" = ?::date", *f.Month)
	}
	if f.Kind != "" {
		add("p.kind = ?", f.Kind)
	}
	if f.Category != "" {
		add("p.category = ?", f.Category)
	}
	if f.PaymentMethod != "" {
		add("p.payment_method = ?", f.PaymentMethod)
	}
	if f.AccountID != nil {
		add("pay.account_id = ?", *f.AccountID)
	}
	if f.Search != "" {
		add(`p.description ILIKE ? ESCAPE '\'`, "%"+escapeLike(f.Search)+"%")
	}
	args = append(args, f.Limit, f.Offset)

	// COUNT(*) OVER () traz o total sem uma segunda consulta; sem linhas, o total é 0.
	query := fmt.Sprintf(`
		SELECT pay.id, COALESCE(pay.due_date, pay.reference_month, pay.created_at::date) AS date,
		       COALESCE(p.description, ''), p.category, p.payment_method, p.kind,
		       COALESCE(p.transfer_direction, ''), p.type, pay.status, pay.amount,
		       pay.installment_number, a.id, COALESCE(a.name || ' ' || a.last4, ''),
		       pay.external_id IS NOT NULL, COUNT(*) OVER ()
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		LEFT JOIN accounts a ON a.id = pay.account_id
		WHERE %s
		ORDER BY date DESC, pay.created_at DESC, pay.id
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao listar transações: %w", err)
	}
	defer rows.Close()

	var (
		result []ports.Transaction
		total  int
	)
	for rows.Next() {
		var t ports.Transaction
		if err := rows.Scan(&t.ID, &t.Date, &t.Description, &t.Category, &t.PaymentMethod, &t.Kind,
			&t.TransferDirection, &t.Type, &t.Status, &t.Amount, &t.InstallmentNumber,
			&t.AccountID, &t.AccountName, &t.FromOpenFinance, &total); err != nil {
			return nil, 0, fmt.Errorf("erro ao escanear transação: %w", err)
		}
		result = append(result, t)
	}
	return result, total, rows.Err()
}

// escapeLike neutraliza os curingas do LIKE para o texto buscado valer literalmente.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *PostgresPurchaseRepository) Accounts(ctx context.Context) ([]ports.Account, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, type, name, last4, balance, credit_limit, available_credit_limit, updated_at
		FROM accounts
		ORDER BY type, name
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar contas: %w", err)
	}
	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ports.Account, error) {
		var a ports.Account
		err := row.Scan(&a.ID, &a.Type, &a.Name, &a.Last4, &a.Balance, &a.CreditLimit, &a.AvailableCreditLimit, &a.UpdatedAt)
		return a, err
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear contas: %w", err)
	}
	return result, nil
}

func NewDashboardReader(db *DB) ports.DashboardReader {
	return &PostgresPurchaseRepository{db: db}
}

func (r *PostgresPurchaseRepository) Positions(ctx context.Context) ([]ports.Position, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, type, subtype, name, balance, amount, updated_at
		FROM investments
		WHERE active
		ORDER BY balance DESC, name
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar posições: %w", err)
	}
	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ports.Position, error) {
		var p ports.Position
		err := row.Scan(&p.ID, &p.Type, &p.Subtype, &p.Name, &p.Balance, &p.Amount, &p.UpdatedAt)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao escanear posições: %w", err)
	}
	return result, nil
}

func (r *PostgresPurchaseRepository) PortfolioHistory(ctx context.Context, from, to time.Time) ([]ports.PortfolioMonth, error) {
	// Para cada mês, soma o último saldo de cada posição até o fim do mês. Sem nenhum
	// registro até lá, a soma é NULL (histórico ainda não existia).
	query := `
		SELECT s.month::date, SUM(b.balance)
		FROM generate_series($1::date, $2::date, '1 month') AS s(month)
		LEFT JOIN LATERAL (
			SELECT DISTINCT ON (ib.investment_id) ib.balance
			FROM investment_balances ib
			WHERE ib.day < s.month + INTERVAL '1 month'
			ORDER BY ib.investment_id, ib.day DESC
		) b ON TRUE
		GROUP BY s.month
		ORDER BY s.month
	`
	rows, err := r.db.Pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar histórico do patrimônio: %w", err)
	}
	defer rows.Close()

	var result []ports.PortfolioMonth
	for rows.Next() {
		var m ports.PortfolioMonth
		if err := rows.Scan(&m.Month, &m.Balance); err != nil {
			return nil, fmt.Errorf("erro ao escanear histórico do patrimônio: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
