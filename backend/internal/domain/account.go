package domain

import (
	"time"

	"github.com/google/uuid"
)

// AccountType diferencia conta corrente de cartão de crédito nas contas vindas do Open Finance.
type AccountType string

const (
	AccountBank   AccountType = "BANK"   // conta corrente: o saldo é dinheiro disponível
	AccountCredit AccountType = "CREDIT" // cartão de crédito: o saldo é valor devido
)

type Account struct {
	ID                   uuid.UUID
	ItemID               string     // conexão de origem; nunca sai da API
	InstitutionID        *uuid.UUID // nil em contas anteriores às instituições, até a próxima sincronização
	Type                 AccountType
	Name                 string
	Last4                string
	Balance              float64
	CreditLimit          *float64
	AvailableCreditLimit *float64
	Brand                string
	CloseDate            *time.Time
	DueDate              *time.Time
	MinimumPayment       *float64
	AutoInvested         *float64
	UpdatedAt            time.Time
}

// Institution é o banco de uma conexão, sem os bytes do logo.
type Institution struct {
	ID      uuid.UUID
	ItemID  string
	Name    string
	Color   string // hexadecimal de 6 dígitos sem "#"; vazio se desconhecida
	HasLogo bool
	// SourceUpdatedAt é quando o Pluggy atualizou os dados do banco; nil se nunca informado.
	SourceUpdatedAt *time.Time
}
