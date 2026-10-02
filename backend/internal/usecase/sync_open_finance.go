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
	"github.com/google/uuid"
)

var (
	ErrSyncInProgress = errors.New("já existe uma sincronização em andamento")
	ErrNotConfigured  = errors.New("o Open Finance não está configurado")
)

type SyncResult struct {
	Inserted   int // lançamentos novos
	Reconciled int // casados com um lançamento manual equivalente
	Existing   int // já sincronizados antes
	Positions  int // posições de investimento atualizadas
}

// SyncOpenFinance copia as transações do Open Finance para o banco. É idempotente: o ID da
// origem é único, então reexecutar (ou sobrepor a janela de datas) nunca duplica.
type SyncOpenFinance struct {
	repo         ports.PurchaseRepository
	provider     ports.OpenFinanceProvider
	itemIDs      []string
	lookbackDays int
	logger       *slog.Logger
	logos        ports.LogoFetcher // opcional: sem ele, o painel usa o monograma
	mu           sync.Mutex
	cfgMu        sync.RWMutex
}

// SetConfig troca os bancos e a janela de datas com o app rodando (página de configurações).
func (s *SyncOpenFinance) SetConfig(itemIDs []string, lookbackDays int) {
	s.cfgMu.Lock()
	s.itemIDs, s.lookbackDays = itemIDs, lookbackDays
	s.cfgMu.Unlock()
}

// SetLogoFetcher liga o cache de logos dos bancos.
func (s *SyncOpenFinance) SetLogoFetcher(f ports.LogoFetcher) { s.logos = f }

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

	s.cfgMu.RLock()
	itemIDs, lookbackDays := s.itemIDs, s.lookbackDays
	s.cfgMu.RUnlock()
	if len(itemIDs) == 0 {
		return SyncResult{}, ErrNotConfigured
	}
	from := time.Now().UTC().AddDate(0, 0, -lookbackDays)

	var result SyncResult
	var errs []error
	for _, itemID := range itemIDs {
		s.syncInvestments(ctx, itemID, &result)

		data, err := s.provider.FetchItem(ctx, itemID, from)
		if err != nil {
			errs = append(errs, fmt.Errorf("item %s: %w", itemID, err))
			continue
		}

		institutionID := s.syncInstitution(ctx, itemID, data.Institution)

		accountIDs := make(map[string]uuid.UUID, len(data.Accounts))
		for _, acc := range data.Accounts {
			acc.InstitutionID = institutionID
			id, err := s.repo.UpsertAccount(ctx, acc)
			if err != nil {
				s.logger.Error("erro ao salvar conta", "account_id", acc.ID, "error", err)
				errs = append(errs, err)
				continue
			}
			accountIDs[acc.ID] = id
		}

		for _, tx := range data.Transactions {
			accountID, ok := accountIDs[tx.AccountID]
			if !ok {
				errs = append(errs, fmt.Errorf("transação %s sem conta salva", tx.ID))
				continue
			}
			if err := s.syncOne(ctx, tx, accountID, &result); err != nil {
				s.logger.Error("erro ao sincronizar transação", "external_id", tx.ID, "error", err)
				errs = append(errs, err)
			}
		}
	}
	return result, errors.Join(errs...)
}

// syncInstitution grava o banco do item e, quando preciso, busca o logo. É só enfeite: qualquer
// falha aqui (inclusive de um imageUrl malicioso) é registrada e a sincronização segue sem logo.
func (s *SyncOpenFinance) syncInstitution(ctx context.Context, itemID string, inst *ports.ExternalInstitution) *uuid.UUID {
	if inst == nil {
		return nil
	}
	id, needsLogo, err := s.repo.UpsertInstitution(ctx, itemID, *inst)
	if err != nil {
		s.logger.Error("erro ao salvar instituição", "error", err)
		return nil
	}
	if needsLogo && s.logos != nil && inst.ImageURL != "" {
		data, mime, err := s.logos.Fetch(ctx, inst.ImageURL)
		if err != nil {
			// Só o erro: a URL é de terceiros e não deve ir para o log.
			s.logger.Warn("logo da instituição não baixado", "institution", inst.Name, "error", err)
			data, mime = nil, ""
		}
		if err := s.repo.SaveInstitutionLogo(ctx, id, data, mime); err != nil {
			s.logger.Error("erro ao salvar logo", "error", err)
		}
	}
	return &id
}

// syncInvestments atualiza as posições do item. É independente das transações: um item pode
// não ter investimentos (ou o Pluggy não os oferecer), e isso não deve contar como falha.
func (s *SyncOpenFinance) syncInvestments(ctx context.Context, itemID string, result *SyncResult) {
	positions, err := s.provider.FetchInvestments(ctx, itemID)
	if err != nil {
		s.logger.Warn("investimentos não sincronizados", "error", err)
		return
	}
	if err := s.repo.SaveInvestments(ctx, itemID, positions, time.Now().UTC()); err != nil {
		s.logger.Error("erro ao salvar investimentos", "error", err)
		return
	}
	result.Positions += len(positions)

	// Mostra se o banco entrega as movimentações, de que depende a estimativa do histórico.
	var withMovements, failed, total int
	for _, p := range positions {
		total += len(p.Movements)
		if len(p.Movements) > 0 {
			withMovements++
		}
		if p.MovementsFailed {
			failed++
		}
	}
	s.logger.Info("investimentos sincronizados", "positions", len(positions), "with_movements", withMovements, "movements", total, "movements_failed", failed)
}

func (s *SyncOpenFinance) syncOne(ctx context.Context, tx ports.ExternalTransaction, accountID uuid.UUID, result *SyncResult) error {
	exists, err := s.repo.RefreshExternal(ctx, tx, accountID)
	if err != nil {
		return err
	}
	if exists {
		result.Existing++
		return nil
	}

	reconciled, err := s.repo.ReconcileExternal(ctx, tx, accountID)
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
	payment.DueDate, payment.PaidAt, payment.ExternalID, payment.AccountID = &tx.Date, &tx.Date, &tx.ID, &accountID
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
