package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// BudgetReader lê e grava o que a tela de orçamento precisa.
type BudgetReader interface {
	// ExpenseKeyMonths devolve as despesas agrupadas por conta e mês, de from a to (primeiros dias dos meses).
	ExpenseKeyMonths(ctx context.Context, from, to time.Time) ([]domain.ExpenseKeyMonth, error)
	// ExpenseRules devolve as correções manuais (chave -> FIXED ou VARIABLE).
	ExpenseRules(ctx context.Context) (map[string]domain.ExpenseClass, error)
	// IncomePayments devolve cada entrada de renda de from a to (primeiros dias dos meses).
	IncomePayments(ctx context.Context, from, to time.Time) ([]domain.IncomePayment, error)
	// KnownInstallments soma, por mês (de from a to), as parcelas já cadastradas de compras parceladas.
	KnownInstallments(ctx context.Context, from, to time.Time) (map[time.Time]float64, error)
	// SetExpenseRule grava a correção manual; class vazio apaga e volta à detecção automática.
	SetExpenseRule(ctx context.Context, key string, class domain.ExpenseClass) error
}
