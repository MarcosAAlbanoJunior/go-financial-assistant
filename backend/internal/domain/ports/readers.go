package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// Leituras do dashboard por contexto. Um mês é identificado pelo seu primeiro dia; as agregações são feitas no banco.

// TotalsReader lê os totais por mês e a divisão dos gastos.
type TotalsReader interface {
	// MonthlyTotals devolve uma linha por mês de from a to (inclusive), com zeros nos meses vazios.
	MonthlyTotals(ctx context.Context, from, to time.Time) ([]domain.MonthTotals, error)
	InvestmentSeries(ctx context.Context, from, to time.Time) ([]domain.InvestmentMonth, error)
	ExpenseBreakdown(ctx context.Context, month time.Time, by domain.BreakdownDimension) ([]domain.BreakdownItem, error)
}

// TransactionReader lê as transações com filtro, paginação e agrupamento.
type TransactionReader interface {
	// Transactions devolve a página pedida e o total de linhas que casam com o filtro.
	Transactions(ctx context.Context, f domain.TransactionFilter) ([]domain.Transaction, int, error)
	// TransactionGroups agrupa as transações que casam com o filtro (Limit e Offset são ignorados).
	TransactionGroups(ctx context.Context, f domain.TransactionFilter, by domain.GroupBy) ([]domain.TransactionGroup, error)
}

// AccountReader lê contas, cartões e as instituições (bancos) a que pertencem.
type AccountReader interface {
	Accounts(ctx context.Context) ([]domain.Account, error)
	Institutions(ctx context.Context) ([]domain.Institution, error)
	// InstitutionLogo devolve o logo em cache; found falso se a instituição ou o logo não existem.
	InstitutionLogo(ctx context.Context, id uuid.UUID) (data []byte, mime string, found bool, err error)
}

// PortfolioReader lê as posições de investimento.
type PortfolioReader interface {
	// Positions devolve as posições de investimento ativas, da maior para a menor.
	Positions(ctx context.Context) ([]domain.Position, error)
	// PortfolioHistory devolve o saldo total ao fim de cada mês de from a to. Só existe a
	// partir da primeira sincronização de investimentos.
	PortfolioHistory(ctx context.Context, from, to time.Time) ([]domain.PortfolioMonth, error)
}
