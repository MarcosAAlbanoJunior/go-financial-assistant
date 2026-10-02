package ports

import (
	"context"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/google/uuid"
)

// TransferCleaner acha e cancela transferências entre contas da própria pessoa já gravadas.
type TransferCleaner interface {
	OwnTransferCandidates(ctx context.Context) ([]domain.TransferCandidate, error)
	CancelPayments(ctx context.Context, ids []uuid.UUID) (int64, error)
}
