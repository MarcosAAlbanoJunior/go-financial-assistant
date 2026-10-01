package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// ExternalTransaction é uma transação vinda do Open Finance já normalizada para o
// vocabulário do app. O adapter descarta o que não deve ser contado (pagamento de
// fatura, transferência entre contas próprias), então tudo aqui vira lançamento.
type ExternalTransaction struct {
	ID            string
	Date          time.Time
	Description   string
	RawInput      string
	Amount        float64 // sempre positivo
	Kind          domain.PurchaseKind
	Direction     domain.TransferDirection // só para KindTransfer
	Category      domain.Category
	PaymentMethod domain.PaymentMethod
	Pending       bool // compra ainda na fatura aberta do cartão
}

type OpenFinanceProvider interface {
	// FetchTransactions retorna as transações de todas as contas e cartões do item desde "from".
	FetchTransactions(ctx context.Context, itemID string, from time.Time) ([]ExternalTransaction, error)
}
