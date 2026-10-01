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
	PurchaseKind      string // "EXPENSE", "INCOME" ou "TRANSFER"
	TransferDirection string // "IN", "OUT" ou "" para não-TRANSFER
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
	FindIncomeTotalByMonth(ctx context.Context, month time.Time) (float64, error)
	FindTransferNetByMonth(ctx context.Context, month time.Time) (applied float64, redeemed float64, err error)
	ExistsPaymentByDateAndAmount(ctx context.Context, date time.Time, amount float64) (bool, error)

	// Open Finance
	// UpsertAccount cria ou atualiza a conta e devolve seu ID interno.
	UpsertAccount(ctx context.Context, account ExternalAccount) (uuid.UUID, error)
	// RefreshExternal informa se a transação já foi sincronizada. Se sim, vincula a conta de
	// origem (quando ainda não tinha, pois foi sincronizada antes da tabela de contas) e promove
	// a categoria de um lançamento que estava em OTHER (regras novas de classificação).
	RefreshExternal(ctx context.Context, tx ExternalTransaction, accountID uuid.UUID) (bool, error)
	// SaveInvestments grava as posições do item e o saldo de hoje; posições do item que não
	// vieram na lista (resgatadas) ficam inativas com saldo zero a partir de hoje.
	SaveInvestments(ctx context.Context, itemID string, positions []ExternalInvestment, day time.Time) error
	// ReconcileExternal liga a transação a um lançamento manual equivalente ainda não conciliado.
	ReconcileExternal(ctx context.Context, tx ExternalTransaction, accountID uuid.UUID) (bool, error)
	SaveExternal(ctx context.Context, purchase *domain.Purchase, payment *domain.Payment) error
}
