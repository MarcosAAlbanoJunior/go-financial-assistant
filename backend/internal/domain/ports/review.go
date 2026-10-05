package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// ReviewReader lê e grava o que a tela de revisão precisa.
type ReviewReader interface {
	// CategoryMonths soma as despesas por categoria e mês, de from a to (primeiros dias dos meses).
	CategoryMonths(ctx context.Context, from, to time.Time) ([]domain.CategoryMonth, error)
	// ExpensePayments devolve cada despesa dos meses de from a to (primeiros dias dos meses), da mais antiga para a mais nova.
	ExpensePayments(ctx context.Context, from, to time.Time) ([]domain.ExpensePayment, error)
	Dismissals(ctx context.Context) ([]domain.Dismissal, error)
	// SetDismissal dispensa a sugestão; dismissed falso a traz de volta.
	SetDismissal(ctx context.Context, d domain.Dismissal, dismissed bool) error
	Decisions(ctx context.Context) ([]domain.Decision, error)
	// SetDecision grava a decisão e dispensa a sugestão (ela passa a aparecer só na economia realizada).
	SetDecision(ctx context.Context, d domain.Decision) error
	// DeleteDecision desfaz a decisão e traz a sugestão de volta.
	DeleteDecision(ctx context.Context, kind, key string) error
}
