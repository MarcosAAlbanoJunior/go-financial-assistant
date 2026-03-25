package domain

import (
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusPaid      PaymentStatus = "PAID"
	PaymentStatusCancelled PaymentStatus = "CANCELLED"
)

type Payment struct {
	ID                uuid.UUID
	PurchaseID        uuid.UUID
	Amount            float64
	Status            PaymentStatus
	InstallmentNumber *int
	DueDate           *time.Time
	ReferenceMonth    *time.Time
	PaidAt            *time.Time
	CreatedAt         time.Time
}

func NewPayment(purchaseID uuid.UUID, amount float64, status PaymentStatus) *Payment {
	now := time.Now().UTC()
	p := &Payment{
		ID:         uuid.New(),
		PurchaseID: purchaseID,
		Amount:     amount,
		Status:     status,
		CreatedAt:  now,
	}
	if status == PaymentStatusPaid {
		p.PaidAt = &now
	}
	return p
}
