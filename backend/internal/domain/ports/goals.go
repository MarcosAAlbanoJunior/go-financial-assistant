package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// GoalStore guarda as metas.
type GoalStore interface {
	Goals(ctx context.Context) ([]domain.Goal, error)
	CreateGoal(ctx context.Context, g domain.Goal) error
	// DeleteGoal apaga a meta; false se ela não existe.
	DeleteGoal(ctx context.Context, id uuid.UUID) (bool, error)
}
