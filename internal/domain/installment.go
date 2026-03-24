package domain

import (
	"time"

	"github.com/google/uuid"
)

type InstallmentStatus string

const (
	InstallmentStatusPending   InstallmentStatus = "PENDING"
	InstallmentStatusPaid      InstallmentStatus = "PAID"
	InstallmentStatusCancelled InstallmentStatus = "CANCELLED"
)

type InstallmentPurchase struct {
	ID                uuid.UUID
	Description       string
	TotalAmount       float64
	InstallmentAmount float64
	TotalInstallments int
	Category          Category
	Payment           PaymentMethod
	PurchaseDate      time.Time
	RawInput          string
	CreatedAt         time.Time
}

type Installment struct {
	ID                uuid.UUID
	PurchaseID        uuid.UUID
	InstallmentNumber int
	TotalInstallments int
	Amount            float64
	DueDate           time.Time
	PaidAt            *time.Time
	Status            InstallmentStatus
	CreatedAt         time.Time
}

func NewInstallmentPurchase(
	description string,
	totalAmount float64,
	installmentAmount float64,
	totalInstallments int,
	category Category,
	payment PaymentMethod,
	rawInput string,
) (*InstallmentPurchase, []Installment, error) {
	if totalAmount <= 0 {
		return nil, nil, ErrInvalidAmount
	}
	if description == "" {
		return nil, nil, ErrEmptyDescription
	}
	if totalInstallments <= 0 {
		return nil, nil, ErrInvalidInstallments
	}
	if payment == "" {
		return nil, nil, ErrInvalidPaymentMethod
	}

	now := time.Now().UTC()
	purchase := &InstallmentPurchase{
		ID:                uuid.New(),
		Description:       description,
		TotalAmount:       totalAmount,
		InstallmentAmount: installmentAmount,
		TotalInstallments: totalInstallments,
		Category:          category,
		Payment:           payment,
		PurchaseDate:      now,
		RawInput:          rawInput,
		CreatedAt:         now,
	}

	installments := make([]Installment, totalInstallments)
	for i := 0; i < totalInstallments; i++ {
		dueDate := time.Date(now.Year(), now.Month()+time.Month(i), now.Day(), 0, 0, 0, 0, time.UTC)
		installments[i] = Installment{
			ID:                uuid.New(),
			PurchaseID:        purchase.ID,
			InstallmentNumber: i + 1,
			TotalInstallments: totalInstallments,
			Amount:            installmentAmount,
			DueDate:           dueDate,
			Status:            InstallmentStatusPending,
			CreatedAt:         now,
		}
	}

	return purchase, installments, nil
}
