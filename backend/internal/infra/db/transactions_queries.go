package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

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
