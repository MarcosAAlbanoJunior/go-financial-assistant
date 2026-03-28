package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

type PaymentSummary struct {
	Category string
	Total    float64
}

type PaymentDetail struct {
	Description       *string
	Category          string
	PaymentMethod     string
	Amount            float64
	Status            string
	PurchaseType      string
	InstallmentNumber *int
	DueDate           *time.Time
	ReferenceMonth    *time.Time
	CreatedAt         time.Time
}

type PurchaseRepository interface {
	Save(ctx context.Context, purchase *domain.Purchase, payments []domain.Payment) error
	FindActiveRecurring(ctx context.Context) ([]domain.Purchase, error)
	FindByDescription(ctx context.Context, description string) ([]domain.Purchase, error)
	Update(ctx context.Context, purchase *domain.Purchase) error
	SavePayment(ctx context.Context, payment *domain.Payment) error
	HasPaymentForMonth(ctx context.Context, purchaseID uuid.UUID, month time.Time) (bool, error)
	FindPaymentsByMonth(ctx context.Context, month time.Time) ([]PaymentSummary, error)
	FindPaymentDetailsByMonth(ctx context.Context, month time.Time) ([]PaymentDetail, error)
}
