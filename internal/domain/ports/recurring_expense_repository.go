package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

type RecurringExpenseRepository interface {
	Save(ctx context.Context, expense *domain.RecurringExpense) error

	FindByID(ctx context.Context, id uuid.UUID) (*domain.RecurringExpense, error)

	FindActive(ctx context.Context) ([]domain.RecurringExpense, error)

	Update(ctx context.Context, expense *domain.RecurringExpense) error

	FindByDescription(ctx context.Context, description string) ([]domain.RecurringExpense, error)
}
