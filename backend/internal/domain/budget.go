package domain

import "time"

// ExpenseKeyMonth soma, em um mês, as despesas de uma mesma conta (descrição normalizada em Key).
type ExpenseKeyMonth struct {
	Key         string
	Label       string // descrição de exemplo, como veio do banco
	Category    string
	Month       time.Time
	Total       float64
	Count       int
	Day         int  // dia do mês do último lançamento
	AllPaid     bool // nenhum lançamento pendente
	Installment bool // parcelada (tipo INSTALLMENT ou "n/m" na descrição)
	Recurring   bool // cadastrada como recorrente
}

// IncomePayment é um recebimento (uma entrada de renda), com a descrição normalizada em Key.
type IncomePayment struct {
	Key    string
	Label  string // descrição de exemplo, como veio do banco
	Month  time.Time
	Amount float64
}

// ExpenseClass é a classificação de uma despesa para o orçamento.
type ExpenseClass string

const (
	ClassFixed       ExpenseClass = "FIXED"
	ClassInstallment ExpenseClass = "INSTALLMENT"
	ClassVariable    ExpenseClass = "VARIABLE"
)

// CategoryMonth soma as despesas de uma categoria em um mês.
type CategoryMonth struct {
	Category string
	Month    time.Time
	Total    float64
}

// ExpensePayment é uma despesa individual, com a descrição normalizada em Key.
type ExpensePayment struct {
	Key           string
	Label         string // descrição de exemplo, como veio do banco
	Category      string
	PaymentMethod string
	Date          time.Time
	Amount        float64
}
