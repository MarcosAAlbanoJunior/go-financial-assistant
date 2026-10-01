package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

type mockProvider struct {
	byItem map[string][]ports.ExternalTransaction
	errs   map[string]error
	from   time.Time
	block  chan struct{}
}

func (m *mockProvider) FetchTransactions(_ context.Context, itemID string, from time.Time) ([]ports.ExternalTransaction, error) {
	m.from = from
	if m.block != nil {
		<-m.block
	}
	return m.byItem[itemID], m.errs[itemID]
}

func extTx(id string, kind domain.PurchaseKind) ports.ExternalTransaction {
	return ports.ExternalTransaction{
		ID: id, Date: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), Description: "Mercado",
		Amount: 42.5, Kind: kind, Category: domain.CategoryMarket, PaymentMethod: domain.PaymentMethodPix,
		RawInput: "[open finance]",
	}
}

func newSync(repo *mockPurchaseRepo, p *mockProvider, items ...string) *SyncOpenFinance {
	return NewSyncOpenFinance(repo, p, items, 60, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestSync_InsertsNewTransactions(t *testing.T) {
	var purchases []*domain.Purchase
	var payments []*domain.Payment
	repo := &mockPurchaseRepo{saveExternalFn: func(_ context.Context, pu *domain.Purchase, pa *domain.Payment) error {
		purchases, payments = append(purchases, pu), append(payments, pa)
		return nil
	}}
	income := extTx("t2", domain.KindIncome)
	transfer := extTx("t3", domain.KindTransfer)
	transfer.Direction = domain.TransferDirectionOut
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{
		"item": {extTx("t1", domain.KindExpense), income, transfer},
	}}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if err != nil || res.Inserted != 3 {
		t.Fatalf("esperava 3 inseridos, got %+v, %v", res, err)
	}
	if purchases[0].Kind != domain.KindExpense || purchases[1].Kind != domain.KindIncome {
		t.Errorf("tipos incorretos: %s %s", purchases[0].Kind, purchases[1].Kind)
	}
	if purchases[2].Kind != domain.KindTransfer || *purchases[2].TransferDirection != domain.TransferDirectionOut {
		t.Error("transferência deveria manter a direção")
	}
	p := payments[0]
	if *p.ExternalID != "t1" || p.Status != domain.PaymentStatusPaid || p.DueDate == nil || p.PaidAt == nil || p.Amount != 42.5 {
		t.Errorf("pagamento incorreto: %+v", p)
	}
	if !prov.from.Before(time.Now().AddDate(0, 0, -59)) {
		t.Errorf("janela deveria ser de 60 dias, from=%v", prov.from)
	}
}

func TestSync_PendingCardTransactionStaysPending(t *testing.T) {
	var got *domain.Payment
	repo := &mockPurchaseRepo{saveExternalFn: func(_ context.Context, _ *domain.Purchase, pa *domain.Payment) error { got = pa; return nil }}
	tx := extTx("t1", domain.KindExpense)
	tx.Pending = true
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {tx}}}

	newSync(repo, prov, "item").Sync(context.Background())
	if got.Status != domain.PaymentStatusPending || got.PaidAt != nil {
		t.Errorf("fatura aberta deveria ficar pendente: %+v", got)
	}
}

func TestSync_IsIdempotent(t *testing.T) {
	saved := 0
	repo := &mockPurchaseRepo{
		existsExternalIDFn: func(context.Context, string) (bool, error) { return true, nil },
		saveExternalFn:     func(context.Context, *domain.Purchase, *domain.Payment) error { saved++; return nil },
	}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if err != nil || res.Existing != 1 || res.Inserted != 0 || saved != 0 {
		t.Errorf("transação existente não pode ser regravada: %+v saved=%d err=%v", res, saved, err)
	}
}

func TestSync_ReconcilesWithManualEntry(t *testing.T) {
	saved := 0
	repo := &mockPurchaseRepo{
		reconcileExternalFn: func(context.Context, ports.ExternalTransaction) (bool, error) { return true, nil },
		saveExternalFn:      func(context.Context, *domain.Purchase, *domain.Payment) error { saved++; return nil },
	}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}

	res, _ := newSync(repo, prov, "item").Sync(context.Background())
	if res.Reconciled != 1 || saved != 0 {
		t.Errorf("deveria conciliar sem criar novo lançamento: %+v saved=%d", res, saved)
	}
}

func TestSync_FailingItemDoesNotStopOthers(t *testing.T) {
	repo := &mockPurchaseRepo{}
	prov := &mockProvider{
		byItem: map[string][]ports.ExternalTransaction{"ok": {extTx("t1", domain.KindExpense)}},
		errs:   map[string]error{"bad": errors.New("pluggy fora do ar")},
	}

	res, err := newSync(repo, prov, "bad", "ok").Sync(context.Background())
	if err == nil {
		t.Error("esperava erro do item que falhou")
	}
	if res.Inserted != 1 {
		t.Errorf("o item saudável deveria ser sincronizado: %+v", res)
	}
}

func TestSync_FailingTransactionDoesNotStopOthers(t *testing.T) {
	calls := 0
	repo := &mockPurchaseRepo{saveExternalFn: func(context.Context, *domain.Purchase, *domain.Payment) error {
		calls++
		if calls == 1 {
			return errors.New("db error")
		}
		return nil
	}}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense), extTx("t2", domain.KindExpense)}}}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if err == nil || res.Inserted != 1 {
		t.Errorf("esperava 1 inserido e erro agregado: %+v, %v", res, err)
	}
}

func TestSync_RejectsConcurrentRuns(t *testing.T) {
	prov := &mockProvider{block: make(chan struct{})}
	s := newSync(&mockPurchaseRepo{}, prov, "item")

	done := make(chan struct{})
	go func() { s.Sync(context.Background()); close(done) }()
	time.Sleep(50 * time.Millisecond)

	if _, err := s.Sync(context.Background()); !errors.Is(err, ErrSyncInProgress) {
		t.Errorf("esperava ErrSyncInProgress, got %v", err)
	}
	close(prov.block)
	<-done
}
