package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RecurringExpense struct {
	ID                 uuid.UUID
	Description        string
	Amount             float64
	Category           Category
	Payment            PaymentMethod
	DayOfMonth         int
	StartDate          time.Time
	EndDate            *time.Time
	IsActive           bool
	LastGeneratedDate  *time.Time
	CancelledAt        *time.Time
	CancellationReason *string
	RawInput           string
	CreatedAt          time.Time
}

func NewRecurringExpense(
	description string,
	amount float64,
	category Category,
	payment PaymentMethod,
	dayOfMonth int,
	rawInput string,
) (*RecurringExpense, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if description == "" {
		return nil, ErrEmptyDescription
	}
	if payment == "" {
		return nil, ErrInvalidPaymentMethod
	}
	if dayOfMonth < 1 || dayOfMonth > 28 {
		dayOfMonth = 1
	}

	now := time.Now().UTC()
	return &RecurringExpense{
		ID:          uuid.New(),
		Description: description,
		Amount:      amount,
		Category:    category,
		Payment:     payment,
		DayOfMonth:  dayOfMonth,
		StartDate:   now,
		IsActive:    true,
		RawInput:    rawInput,
		CreatedAt:   now,
	}, nil
}

func (r *RecurringExpense) Cancel(reason string) {
	now := time.Now().UTC()
	r.IsActive = false
	r.CancelledAt = &now
	r.CancellationReason = &reason
	r.EndDate = &now
}

func (r *RecurringExpense) GenerateExpense() (*Expense, error) {
	expense, err := NewExpense(
		r.Amount,
		r.Description,
		r.Category,
		r.Payment,
		fmt.Sprintf("[recorrente] %s", r.RawInput),
		nil,
	)
	if err != nil {
		return nil, err
	}
	expense.RecurringExpenseID = &r.ID
	return expense, nil
}

func (r *RecurringExpense) ShouldGenerateForMonth(year int, month time.Month) bool {
	if !r.IsActive {
		return false
	}
	if r.LastGeneratedDate == nil {
		return true
	}
	lastYear, lastMonth, _ := r.LastGeneratedDate.Date()
	return lastYear < year || (lastYear == year && lastMonth < month)
}
