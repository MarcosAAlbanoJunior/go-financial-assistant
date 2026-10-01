package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

var ErrSyncInProgress = errors.New("já existe uma sincronização em andamento")

type SyncResult struct {
	Inserted   int // lançamentos novos
	Reconciled int // casados com um lançamento manual equivalente
	Existing   int // já sincronizados antes
}

// SyncOpenFinance copia as transações do Open Finance para o banco. É idempotente: o ID da
// origem é único, então reexecutar (ou sobrepor a janela de datas) nunca duplica.
type SyncOpenFinance struct {
	repo         ports.PurchaseRepository
	provider     ports.OpenFinanceProvider
	itemIDs      []string
	lookbackDays int
	logger       *slog.Logger
	mu           sync.Mutex
}

func NewSyncOpenFinance(repo ports.PurchaseRepository, provider ports.OpenFinanceProvider, itemIDs []string, lookbackDays int, logger *slog.Logger) *SyncOpenFinance {
	return &SyncOpenFinance{repo: repo, provider: provider, itemIDs: itemIDs, lookbackDays: lookbackDays, logger: logger}
}

// Sync processa todos os itens. A falha de um item (ou de uma transação) não impede os
// demais; os erros são reunidos no retorno junto com o que foi possível sincronizar.
func (s *SyncOpenFinance) Sync(ctx context.Context) (SyncResult, error) {
	if !s.mu.TryLock() {
		return SyncResult{}, ErrSyncInProgress
	}
	defer s.mu.Unlock()

	from := time.Now().UTC().AddDate(0, 0, -s.lookbackDays)

	var result SyncResult
	var errs []error
	for _, itemID := range s.itemIDs {
		txs, err := s.provider.FetchTransactions(ctx, itemID, from)
		if err != nil {
			errs = append(errs, fmt.Errorf("item %s: %w", itemID, err))
			continue
		}
		for _, tx := range txs {
			if err := s.syncOne(ctx, tx, &result); err != nil {
				s.logger.Error("erro ao sincronizar transação", "external_id", tx.ID, "error", err)
				errs = append(errs, err)
			}
		}
	}
	return result, errors.Join(errs...)
}

func (s *SyncOpenFinance) syncOne(ctx context.Context, tx ports.ExternalTransaction, result *SyncResult) error {
	exists, err := s.repo.ExistsExternalID(ctx, tx.ID)
	if err != nil {
		return err
	}
	if exists {
		result.Existing++
		return nil
	}

	reconciled, err := s.repo.ReconcileExternal(ctx, tx)
	if err != nil {
		return err
	}
	if reconciled {
		result.Reconciled++
		return nil
	}

	purchase, err := newPurchaseFromExternal(tx)
	if err != nil {
		return fmt.Errorf("transação externa inválida: %w", err)
	}

	payment := domain.NewPayment(purchase.ID, tx.Amount, domain.PaymentStatusPaid)
	payment.DueDate, payment.PaidAt, payment.ExternalID = &tx.Date, &tx.Date, &tx.ID
	if tx.Pending {
		payment.Status, payment.PaidAt = domain.PaymentStatusPending, nil
	}

	if err := s.repo.SaveExternal(ctx, purchase, payment); err != nil {
		return err
	}
	result.Inserted++
	return nil
}

func newPurchaseFromExternal(tx ports.ExternalTransaction) (*domain.Purchase, error) {
	switch tx.Kind {
	case domain.KindIncome:
		return domain.NewIncome(tx.Amount, &tx.Description, tx.Category, tx.PaymentMethod, domain.PurchaseTypeSingle, tx.RawInput)
	case domain.KindTransfer:
		return domain.NewTransfer(tx.Amount, &tx.Description, tx.PaymentMethod, domain.PurchaseTypeSingle, tx.RawInput, tx.Direction)
	default:
		return domain.NewPurchase(tx.Amount, &tx.Description, tx.Category, tx.PaymentMethod, domain.PurchaseTypeSingle, tx.RawInput)
	}
}
