package domain

import (
	"time"

	"github.com/google/uuid"
)

type PurchaseType string

const (
	PurchaseTypeSingle      PurchaseType = "SINGLE"
	PurchaseTypeInstallment PurchaseType = "INSTALLMENT"
	PurchaseTypeRecurring   PurchaseType = "RECURRING"
)

type PurchaseKind string

const (
	KindExpense  PurchaseKind = "EXPENSE"
	KindIncome   PurchaseKind = "INCOME"
	KindTransfer PurchaseKind = "TRANSFER"
)

type TransferDirection string

const (
	TransferDirectionOut TransferDirection = "OUT" // aplicação: dinheiro saindo para investimento
	TransferDirectionIn  TransferDirection = "IN"  // resgate: dinheiro voltando do investimento
)

type PaymentMethod string

const (
	PaymentMethodCash       PaymentMethod = "CASH"
	PaymentMethodCreditCard PaymentMethod = "CREDIT_CARD"
	PaymentMethodDebitCard  PaymentMethod = "DEBIT_CARD"
	PaymentMethodPix        PaymentMethod = "PIX"
	PaymentMethodOther      PaymentMethod = "OTHER"
)

type Category string

const (
	CategoryFood          Category = "FOOD"
	CategoryTransport     Category = "TRANSPORT"
	CategoryHealth        Category = "HEALTH"
	CategoryEntertainment Category = "ENTERTAINMENT"
	CategoryShopping      Category = "SHOPPING"
	CategoryMarket        Category = "MARKET"
	CategoryInvestment    Category = "INVESTMENT"
	CategorySalary        Category = "SALARY"
	CategoryOther         Category = "OTHER"
)

type Purchase struct {
	ID                 uuid.UUID
	Description        *string
	Category           Category
	PaymentMethod      PaymentMethod
	Kind               PurchaseKind
	TransferDirection  *TransferDirection
	Type               PurchaseType
	TotalAmount        float64
	InstallmentCount   *int
	InstallmentAmount  *float64
	DayOfMonth         *int
	IsActive           bool
	CancelledAt        *time.Time
	CancellationReason *string
	RawInput           string
	CreatedAt          time.Time
}

func NewPurchase(
	amount float64,
	description *string,
	category Category,
	paymentMethod PaymentMethod,
	purchaseType PurchaseType,
	rawInput string,
) (*Purchase, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	return &Purchase{
		ID:            uuid.New(),
		Description:   description,
		Category:      category,
		PaymentMethod: paymentMethod,
		Kind:          KindExpense,
		Type:          purchaseType,
		TotalAmount:   amount,
		IsActive:      true,
		RawInput:      rawInput,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func NewIncome(
	amount float64,
	description *string,
	category Category,
	paymentMethod PaymentMethod,
	purchaseType PurchaseType,
	rawInput string,
) (*Purchase, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if purchaseType == PurchaseTypeInstallment {
		return nil, ErrInvalidAmount
	}
	return &Purchase{
		ID:            uuid.New(),
		Description:   description,
		Category:      category,
		PaymentMethod: paymentMethod,
		Kind:          KindIncome,
		Type:          purchaseType,
		TotalAmount:   amount,
		IsActive:      true,
		RawInput:      rawInput,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func NewTransfer(
	amount float64,
	description *string,
	paymentMethod PaymentMethod,
	purchaseType PurchaseType,
	rawInput string,
	direction TransferDirection,
) (*Purchase, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if purchaseType == PurchaseTypeInstallment {
		return nil, ErrInvalidAmount
	}
	return &Purchase{
		ID:                uuid.New(),
		Description:       description,
		Category:          CategoryOther,
		PaymentMethod:     paymentMethod,
		Kind:              KindTransfer,
		TransferDirection: &direction,
		Type:              purchaseType,
		TotalAmount:       amount,
		IsActive:          true,
		RawInput:          rawInput,
		CreatedAt:         time.Now().UTC(),
	}, nil
}

func ParseTransferDirection(s string) TransferDirection {
	if s == string(TransferDirectionIn) {
		return TransferDirectionIn
	}
	return TransferDirectionOut
}

func (p *Purchase) Cancel(reason string) {
	now := time.Now().UTC()
	p.IsActive = false
	p.CancelledAt = &now
	p.CancellationReason = &reason
}
