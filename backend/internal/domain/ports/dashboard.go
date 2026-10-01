package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MonthTotals soma, em um mês, entradas, despesas e movimentação de investimentos
// (aplicado = dinheiro que saiu para investir, resgatado = voltou).
type MonthTotals struct {
	Month    time.Time
	Income   float64
	Expense  float64
	Applied  float64
	Redeemed float64
}

// InvestmentMonth acumula o líquido aplicado (aplicado - resgatado) desde o primeiro
// lançamento de investimento, não apenas dentro da janela consultada.
type InvestmentMonth struct {
	Month      time.Time
	Applied    float64
	Redeemed   float64
	Cumulative float64
}

type BreakdownDimension string

const (
	BreakdownByCategory      BreakdownDimension = "category"
	BreakdownByPaymentMethod BreakdownDimension = "payment_method"
	BreakdownByAccount       BreakdownDimension = "account"
)

// BreakdownItem é um grupo de despesas. Key é o valor do enum (categoria, forma de
// pagamento) ou o ID da conta; Name só vem preenchido para contas.
type BreakdownItem struct {
	Key   string
	Name  string
	Total float64
}

type TransactionFilter struct {
	Month         *time.Time // primeiro dia do mês; nil = todos os meses
	Kind          string
	Category      string
	PaymentMethod string
	AccountID     *uuid.UUID
	Search        string // trecho da descrição
	Limit         int
	Offset        int
}

type Transaction struct {
	ID                uuid.UUID
	Date              time.Time
	Description       string
	Category          string
	PaymentMethod     string
	Kind              string
	TransferDirection string
	Type              string
	Status            string
	Amount            float64
	InstallmentNumber *int
	AccountID         *uuid.UUID
	AccountName       string
	FromOpenFinance   bool
}

type Account struct {
	ID                   uuid.UUID
	Type                 string
	Name                 string
	Last4                string
	Balance              float64
	CreditLimit          *float64
	AvailableCreditLimit *float64
	UpdatedAt            time.Time
}

// DashboardReader reúne as consultas de leitura do dashboard. As agregações são feitas
// no banco; um mês é identificado pelo seu primeiro dia.
type DashboardReader interface {
	// MonthlyTotals devolve uma linha por mês de from a to (inclusive), com zeros nos meses vazios.
	MonthlyTotals(ctx context.Context, from, to time.Time) ([]MonthTotals, error)
	InvestmentSeries(ctx context.Context, from, to time.Time) ([]InvestmentMonth, error)
	ExpenseBreakdown(ctx context.Context, month time.Time, by BreakdownDimension) ([]BreakdownItem, error)
	// Transactions devolve a página pedida e o total de linhas que casam com o filtro.
	Transactions(ctx context.Context, f TransactionFilter) ([]Transaction, int, error)
	Accounts(ctx context.Context) ([]Account, error)
}
