package domain

// AccountType diferencia conta corrente de cartão de crédito nas contas vindas do Open Finance.
type AccountType string

const (
	AccountBank   AccountType = "BANK"   // conta corrente: o saldo é dinheiro disponível
	AccountCredit AccountType = "CREDIT" // cartão de crédito: o saldo é valor devido
)
