package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// CategoryStore classifica as despesas que ficaram em "Outros".
type CategoryStore interface {
	// UncategorizedExpenses lista as contas com despesa em Outros que ainda não têm regra, da maior para a menor.
	UncategorizedExpenses(ctx context.Context, limit int) ([]domain.UncategorizedGroup, error)
	// SetCategoryRule grava a regra e reclassifica as despesas da conta que estavam em Outros (ou na categoria
	// da regra anterior). Devolve quantos lançamentos mudaram. A regra também vale para as próximas sincronizações.
	SetCategoryRule(ctx context.Context, key, category string) (int64, error)
}
