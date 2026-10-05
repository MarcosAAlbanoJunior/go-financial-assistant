package domain

import (
	"time"

	"github.com/google/uuid"
)

type TransactionFilter struct {
	Month         *time.Time // primeiro dia do mês; nil = todos os meses
	Kind          string
	Category      string
	PaymentMethod string
	AccountID     *uuid.UUID
	Day           *time.Time // um dia específico (sobrepõe Month)
	Search        string     // trecho da descrição
	Limit         int
	Offset        int
}

type GroupBy string

const (
	GroupByCategory GroupBy = "category"
	GroupByDay      GroupBy = "day"
)

// TransactionGroup soma as transações de um grupo (categoria ou dia) sob os mesmos filtros da lista.
// Key é o valor do enum (categoria) ou a data AAAA-MM-DD.
type TransactionGroup struct {
	Key      string
	Count    int
	Expense  float64
	Income   float64
	Transfer float64
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

// TransferCandidate é um lançamento do banco que parece Pix, TED ou DOC.
type TransferCandidate struct {
	PaymentID   uuid.UUID
	Description string
	Kind        string // EXPENSE ou INCOME
	Amount      float64
}
