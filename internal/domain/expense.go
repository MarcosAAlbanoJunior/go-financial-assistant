package domain

import (
	"time"

	"github.com/google/uuid"
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
	CategoryOther         Category = "OTHER"
)

type Expense struct {
	ID                 uuid.UUID
	Amount             float64
	Description        string
	Category           Category
	Payment            PaymentMethod
	ReceiptURL         *string
	RawInput           string
	RecurringExpenseID *uuid.UUID
	CreatedAt          time.Time
}

func NewExpense(
	amount float64,
	description string,
	category Category,
	payment PaymentMethod,
	rawInput string,
	receiptURL *string,
) (*Expense, error) {
	if err := validate(amount, description, payment); err != nil {
		return nil, err
	}

	return &Expense{
		ID:          uuid.New(),
		Amount:      amount,
		Description: description,
		Category:    category,
		Payment:     payment,
		RawInput:    rawInput,
		ReceiptURL:  receiptURL,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func validate(amount float64, description string, payment PaymentMethod) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if description == "" {
		return ErrEmptyDescription
	}
	if payment == "" {
		return ErrInvalidPaymentMethod
	}
	return nil
}
