package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

type InstallmentRepository interface {
	SavePurchase(ctx context.Context, purchase *domain.InstallmentPurchase, installments []domain.Installment) error

	FindPurchaseByID(ctx context.Context, id uuid.UUID) (*domain.InstallmentPurchase, error)

	FindInstallmentsByPurchaseID(ctx context.Context, purchaseID uuid.UUID) ([]domain.Installment, error)

	FindPendingInstallments(ctx context.Context) ([]domain.Installment, error)
}
