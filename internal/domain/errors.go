package domain

import "errors"

var (
	ErrInvalidAmount               = errors.New("valor deve ser maior que zero")
	ErrEmptyDescription            = errors.New("descrição não pode ser vazia")
	ErrInvalidPaymentMethod        = errors.New("forma de pagamento inválida")
	ErrExpenseNotFound             = errors.New("despesa não encontrada")
	ErrInvalidInstallments         = errors.New("número de parcelas deve ser maior que zero")
	ErrRecurringExpenseNotFound    = errors.New("despesa recorrente não encontrada")
	ErrInstallmentPurchaseNotFound = errors.New("compra parcelada não encontrada")
)
