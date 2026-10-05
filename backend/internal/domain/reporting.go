package domain

import "time"

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
