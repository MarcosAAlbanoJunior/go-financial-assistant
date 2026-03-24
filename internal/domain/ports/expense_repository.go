package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

type ExpenseRepository interface {
	Save(ctx context.Context, expense *domain.Expense) error

	FindByID(ctx context.Context, id uuid.UUID) (*domain.Expense, error)

	FindAll(ctx context.Context, filter ExpenseFilter) ([]domain.Expense, error)

	Delete(ctx context.Context, id uuid.UUID) error
}

type ExpenseFilter struct {
	Category *domain.Category
	Payment  *domain.PaymentMethod
	FromDate *time.Time
	ToDate   *time.Time
}
