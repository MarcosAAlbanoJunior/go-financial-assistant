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
	CategoryOther         Category = "OTHER"
)

type Purchase struct {
	ID                 uuid.UUID
	Description        *string
	Category           Category
	PaymentMethod      PaymentMethod
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
		Type:          purchaseType,
		TotalAmount:   amount,
		IsActive:      true,
		RawInput:      rawInput,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (p *Purchase) Cancel(reason string) {
	now := time.Now().UTC()
	p.IsActive = false
	p.CancelledAt = &now
	p.CancellationReason = &reason
}
