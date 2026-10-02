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
	"github.com/google/uuid"
)

type mockProvider struct {
	byItem map[string][]ports.ExternalTransaction
	errs   map[string]error
	from   time.Time
	block  chan struct{}

	positions    []ports.ExternalInvestment
	positionsErr error
	institution  *ports.ExternalInstitution
}

func (m *mockProvider) FetchItem(_ context.Context, itemID string, from time.Time) (ports.ItemData, error) {
	m.from = from
	if m.block != nil {
		<-m.block
	}
	return ports.ItemData{
		Institution:  m.institution,
		Accounts:     []ports.ExternalAccount{{ID: "acc", ItemID: itemID, Type: "BANK", Name: "Conta"}},
		Transactions: m.byItem[itemID],
	}, m.errs[itemID]
}

func (m *mockProvider) FetchInvestments(context.Context, string) ([]ports.ExternalInvestment, error) {
	return m.positions, m.positionsErr
}

func extTx(id string, kind domain.PurchaseKind) ports.ExternalTransaction {
	return ports.ExternalTransaction{
		ID: id, AccountID: "acc", Date: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), Description: "Mercado",
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
		refreshExternalFn: func(context.Context, ports.ExternalTransaction, uuid.UUID) (bool, error) { return true, nil },
		saveExternalFn:    func(context.Context, *domain.Purchase, *domain.Payment) error { saved++; return nil },
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
		reconcileExternalFn: func(context.Context, ports.ExternalTransaction, uuid.UUID) (bool, error) { return true, nil },
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

func TestSync_LinksPaymentToAccount(t *testing.T) {
	accountID := uuid.New()
	var upserted ports.ExternalAccount
	var saved *domain.Payment
	var linkedTo, reconciledTo uuid.UUID
	repo := &mockPurchaseRepo{
		upsertAccountFn: func(_ context.Context, a ports.ExternalAccount) (uuid.UUID, error) {
			upserted = a
			return accountID, nil
		},
		refreshExternalFn: func(_ context.Context, tx ports.ExternalTransaction, acc uuid.UUID) (bool, error) {
			linkedTo = acc
			return tx.ID == "old", nil
		},
		reconcileExternalFn: func(_ context.Context, tx ports.ExternalTransaction, acc uuid.UUID) (bool, error) {
			reconciledTo = acc
			return tx.ID == "manual", nil
		},
		saveExternalFn: func(_ context.Context, _ *domain.Purchase, pa *domain.Payment) error { saved = pa; return nil },
	}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {
		extTx("old", domain.KindExpense), extTx("manual", domain.KindExpense), extTx("new", domain.KindExpense),
	}}}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if err != nil || res.Existing != 1 || res.Reconciled != 1 || res.Inserted != 1 {
		t.Fatalf("resultado inesperado: %+v, %v", res, err)
	}
	if upserted.ID != "acc" || upserted.ItemID != "item" {
		t.Errorf("conta não salva: %+v", upserted)
	}
	if linkedTo != accountID || reconciledTo != accountID || saved.AccountID == nil || *saved.AccountID != accountID {
		t.Error("as três vias deveriam vincular o pagamento à conta")
	}
}

func TestSync_AccountFailureSkipsItsTransactions(t *testing.T) {
	saved := 0
	repo := &mockPurchaseRepo{
		upsertAccountFn: func(context.Context, ports.ExternalAccount) (uuid.UUID, error) {
			return uuid.Nil, errors.New("db error")
		},
		saveExternalFn: func(context.Context, *domain.Purchase, *domain.Payment) error { saved++; return nil },
	}
	prov := &mockProvider{byItem: map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}}}

	if _, err := newSync(repo, prov, "item").Sync(context.Background()); err == nil || saved != 0 {
		t.Errorf("sem conta salva a transação não pode ser gravada: saved=%d err=%v", saved, err)
	}
}

func TestSync_SavesInvestmentsIndependentlyOfTransactions(t *testing.T) {
	var gotItem string
	var gotPositions []ports.ExternalInvestment
	repo := &mockPurchaseRepo{saveInvestmentsFn: func(_ context.Context, item string, p []ports.ExternalInvestment, _ time.Time) error {
		gotItem, gotPositions = item, p
		return nil
	}}
	prov := &mockProvider{
		positions: []ports.ExternalInvestment{{ID: "inv1", Balance: 100}, {ID: "inv2", Balance: 50}},
		errs:      map[string]error{"item": errors.New("transações fora do ar")},
	}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if err == nil {
		t.Error("a falha das transações ainda deve ser reportada")
	}
	if gotItem != "item" || len(gotPositions) != 2 || res.Positions != 2 {
		t.Errorf("posições deveriam ser salvas mesmo com as transações falhando: item=%q n=%d res=%+v", gotItem, len(gotPositions), res)
	}
}

func TestSync_InvestmentFailureIsNotAnError(t *testing.T) {
	saved := false
	repo := &mockPurchaseRepo{saveInvestmentsFn: func(context.Context, string, []ports.ExternalInvestment, time.Time) error {
		saved = true
		return nil
	}}
	prov := &mockProvider{
		byItem:       map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}},
		positionsErr: errors.New("item sem produto de investimentos"),
	}

	res, err := newSync(repo, prov, "item").Sync(context.Background())
	if err != nil || res.Inserted != 1 || saved || res.Positions != 0 {
		t.Errorf("sem investimentos não é falha e não deve gravar nada: %+v saved=%v err=%v", res, saved, err)
	}
}

type mockLogos struct {
	data  []byte
	mime  string
	err   error
	calls int
}

func (m *mockLogos) Fetch(context.Context, string) ([]byte, string, error) {
	m.calls++
	return m.data, m.mime, m.err
}

func TestSync_SavesInstitutionAndLogo(t *testing.T) {
	instID := uuid.New()
	var linked *uuid.UUID
	var savedLogo []byte
	repo := &mockPurchaseRepo{
		upsertInstitutionFn: func(context.Context, string, ports.ExternalInstitution) (uuid.UUID, bool, error) {
			return instID, true, nil
		},
		upsertAccountFn: func(_ context.Context, a ports.ExternalAccount) (uuid.UUID, error) {
			linked = a.InstitutionID
			return uuid.New(), nil
		},
		saveLogoFn: func(_ context.Context, _ uuid.UUID, data []byte, _ string) error { savedLogo = data; return nil },
	}
	prov := &mockProvider{institution: &ports.ExternalInstitution{Name: "Itaú", Color: "ec7000", ImageURL: "https://x/logo.png"}}
	s := newSync(repo, prov, "item")
	s.SetLogoFetcher(&mockLogos{data: []byte("png"), mime: "image/png"})

	if _, err := s.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if linked == nil || *linked != instID {
		t.Errorf("a conta deveria apontar para a instituição, got %v", linked)
	}
	if string(savedLogo) != "png" {
		t.Errorf("logo deveria ser gravado, got %q", savedLogo)
	}
}

func TestSync_LogoFailureDoesNotBreakSync(t *testing.T) {
	var saved bool
	var savedData []byte
	repo := &mockPurchaseRepo{
		upsertInstitutionFn: func(context.Context, string, ports.ExternalInstitution) (uuid.UUID, bool, error) {
			return uuid.New(), true, nil
		},
		saveLogoFn: func(_ context.Context, _ uuid.UUID, d []byte, _ string) error { saved, savedData = true, d; return nil },
	}
	prov := &mockProvider{
		institution: &ports.ExternalInstitution{Name: "Banco", ImageURL: "https://10.0.0.1/x.png"},
		byItem:      map[string][]ports.ExternalTransaction{"item": {extTx("t1", domain.KindExpense)}},
	}
	s := newSync(repo, prov, "item")
	s.SetLogoFetcher(&mockLogos{err: errors.New("bloqueado")})

	res, err := s.Sync(context.Background())
	if err != nil || res.Inserted != 1 {
		t.Fatalf("a falha do logo não pode derrubar o sync: %+v, %v", res, err)
	}
	if !saved || savedData != nil {
		t.Error("deveria registrar a tentativa (dados nulos) para não repetir a cada sync")
	}
}

func TestSync_SkipsLogoWhenNotNeeded(t *testing.T) {
	repo := &mockPurchaseRepo{} // needsLogo = false
	logos := &mockLogos{}
	prov := &mockProvider{institution: &ports.ExternalInstitution{Name: "Banco", ImageURL: "https://x/y.png"}}
	s := newSync(repo, prov, "item")
	s.SetLogoFetcher(logos)
	if _, err := s.Sync(context.Background()); err != nil || logos.calls != 0 {
		t.Fatalf("não deveria buscar o logo: calls=%d err=%v", logos.calls, err)
	}
}

func TestSync_InstitutionErrorIsNotFatal(t *testing.T) {
	var got *uuid.UUID
	repo := &mockPurchaseRepo{
		upsertInstitutionFn: func(context.Context, string, ports.ExternalInstitution) (uuid.UUID, bool, error) {
			return uuid.Nil, false, errors.New("db")
		},
		upsertAccountFn: func(_ context.Context, a ports.ExternalAccount) (uuid.UUID, error) {
			got = a.InstitutionID
			return uuid.New(), nil
		},
	}
	prov := &mockProvider{institution: &ports.ExternalInstitution{Name: "Banco"}}
	if _, err := newSync(repo, prov, "item").Sync(context.Background()); err != nil || got != nil {
		t.Fatalf("err=%v institution=%v", err, got)
	}
}
