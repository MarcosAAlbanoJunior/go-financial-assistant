package domain

import "errors"

var (
	ErrInvalidAmount        = errors.New("valor deve ser maior que zero")
	ErrInvalidPaymentMethod = errors.New("forma de pagamento inválida")
	ErrPurchaseNotFound     = errors.New("compra não encontrada")
)
