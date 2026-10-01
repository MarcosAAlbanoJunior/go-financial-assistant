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
// txDate é o dia da transação (vencimento, mês de referência ou criação).
const txDate = `COALESCE(pay.due_date, pay.reference_month, pay.created_at::date)`

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

// transactionWhere monta a cláusula WHERE (e os argumentos) do filtro, compartilhada pela lista e
// pelos grupos. Só texto fixo entra no SQL; os valores vão sempre como parâmetros.
func transactionWhere(f ports.TransactionFilter) (string, []any) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	where = append(where, "pay.status != 'CANCELLED'")
	switch {
	case f.Day != nil:
		add(txDate+" = ?::date", *f.Day)
	case f.Month != nil:
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
	return strings.Join(where, " AND "), args
}

func (r *PostgresPurchaseRepository) Transactions(ctx context.Context, f ports.TransactionFilter) ([]ports.Transaction, int, error) {
	where, args := transactionWhere(f)
	args = append(args, f.Limit, f.Offset)

	// COUNT(*) OVER () traz o total sem uma segunda consulta; sem linhas, o total é 0.
	query := fmt.Sprintf(`
		SELECT pay.id, `+txDate+` AS date,
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
	`, where, len(args)-1, len(args))

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
	// A partir do mês da primeira sincronização o saldo é exato: soma do último saldo gravado de
	// cada posição até o fim do mês (NULL se ainda não havia registro).
	// Antes disso é estimado, por posição: saldo do primeiro registro menos o que foi aplicado
	// depois do mês (e antes desse registro), sem contar rendimentos. Posição sem nenhuma
	// movimentação até o fim do mês ainda não existia e fica de fora; o saldo nunca é negativo.
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

	var result []ports.PortfolioMonth
	for rows.Next() {
		var m ports.PortfolioMonth
		if err := rows.Scan(&m.Month, &m.Balance, &m.Estimated); err != nil {
			return nil, fmt.Errorf("erro ao escanear histórico do patrimônio: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *PostgresPurchaseRepository) TransactionGroups(ctx context.Context, f ports.TransactionFilter, by ports.GroupBy) ([]ports.TransactionGroup, error) {
	// A chave do grupo vem de fragmentos fixos, nunca do cliente.
	var key, order string
	switch by {
	case ports.GroupByCategory:
		key, order = "p.category", "expense DESC, income DESC, transfer DESC, key"
	case ports.GroupByDay:
		key, order = "TO_CHAR("+txDate+", 'YYYY-MM-DD')", "key DESC"
	default:
		return nil, fmt.Errorf("agrupamento inválido: %q", by)
	}
	where, args := transactionWhere(f)

	query := fmt.Sprintf(`
		SELECT %s AS key, COUNT(*),
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'EXPENSE'), 0)  AS expense,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'INCOME'), 0)   AS income,
		       COALESCE(SUM(pay.amount) FILTER (WHERE p.kind = 'TRANSFER'), 0) AS transfer
		FROM payments pay
		JOIN purchases p ON p.id = pay.purchase_id
		WHERE %s
		GROUP BY 1
		ORDER BY %s
	`, key, where, order)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao agrupar transações: %w", err)
	}
	defer rows.Close()

	var result []ports.TransactionGroup
	for rows.Next() {
		var g ports.TransactionGroup
		if err := rows.Scan(&g.Key, &g.Count, &g.Expense, &g.Income, &g.Transfer); err != nil {
			return nil, fmt.Errorf("erro ao escanear grupo: %w", err)
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

// expenseKey normaliza a descrição para reconhecer a mesma conta em meses diferentes: minúsculas,
// só letras (some número, data, parcela e pontuação) e espaços compactados.
const expenseKey = `BTRIM(REGEXP_REPLACE(REGEXP_REPLACE(LOWER(p.description), '[^a-zà-ÿ ]+', ' ', 'g'), '\s+', ' ', 'g'))`

func (r *PostgresPurchaseRepository) ExpenseKeyMonths(ctx context.Context, from, to time.Time) ([]ports.ExpenseKeyMonth, error) {
	query := `
		SELECT ` + expenseKey + ` AS key,
		       (ARRAY_AGG(p.description ORDER BY pay.created_at DESC))[1] AS label,
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
