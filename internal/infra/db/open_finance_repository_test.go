package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// Teste de integração: roda só com TEST_DATABASE_URL apontando para um Postgres com as
// migrations aplicadas. Ex.: TEST_DATABASE_URL=$DATABASE_URL go test ./internal/infra/db
func newTestRepo(t *testing.T) (*PostgresPurchaseRepository, *DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	pg, err := NewPostgres(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	return &PostgresPurchaseRepository{db: pg}, pg
}

func cleanup(t *testing.T, pg *DB, purchases ...*domain.Purchase) {
	t.Helper()
	t.Cleanup(func() {
		for _, p := range purchases {
			pg.Pool.Exec(context.Background(), `DELETE FROM purchases WHERE id = $1`, p.ID)
		}
	})
}

func manualExpense(t *testing.T, repo *PostgresPurchaseRepository, pg *DB, amount float64, typ domain.PurchaseType) *domain.Purchase {
	t.Helper()
	desc := "teste-open-finance"
	p, err := domain.NewPurchase(amount, &desc, domain.CategoryFood, domain.PaymentMethodPix, typ, "teste")
	if err != nil {
		t.Fatal(err)
	}
	pay := domain.NewPayment(p.ID, amount, domain.PaymentStatusPaid)
	if typ == domain.PurchaseTypeRecurring {
		first := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.UTC)
		pay.ReferenceMonth = &first
	}
	if err := repo.Save(context.Background(), p, []domain.Payment{*pay}); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
	return p
}

func externalTx(id string, amount float64, date time.Time) ports.ExternalTransaction {
	return ports.ExternalTransaction{ID: id, Date: date, Amount: amount, Kind: domain.KindExpense}
}

func externalIDOf(t *testing.T, pg *DB, purchaseID any) *string {
	t.Helper()
	var id *string
	if err := pg.Pool.QueryRow(context.Background(), `SELECT external_id FROM payments WHERE purchase_id = $1`, purchaseID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestReconcileExternal_LinksManualSingle(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	manual := manualExpense(t, repo, pg, 987.65, domain.PurchaseTypeSingle)

	// Fora da janela de 3 dias, de outro valor ou de outro tipo: não concilia.
	for name, tx := range map[string]ports.ExternalTransaction{
		"data distante":   externalTx("of-far", 987.65, time.Now().AddDate(0, 0, -10)),
		"valor diferente": externalTx("of-amount", 987.66, time.Now()),
		"outro tipo":      {ID: "of-kind", Date: time.Now(), Amount: 987.65, Kind: domain.KindIncome},
	} {
		if ok, err := repo.ReconcileExternal(ctx, tx); err != nil || ok {
			t.Fatalf("%s: não deveria conciliar (ok=%v, err=%v)", name, ok, err)
		}
	}

	ok, err := repo.ReconcileExternal(ctx, externalTx("of-1", 987.65, time.Now().AddDate(0, 0, -2)))
	if err != nil || !ok {
		t.Fatalf("deveria conciliar: ok=%v err=%v", ok, err)
	}
	if id := externalIDOf(t, pg, manual.ID); id == nil || *id != "of-1" {
		t.Errorf("external_id não gravado: %v", id)
	}

	// Cada lançamento manual só pode ser conciliado uma vez.
	if ok, _ := repo.ReconcileExternal(ctx, externalTx("of-2", 987.65, time.Now())); ok {
		t.Error("um lançamento já conciliado não pode casar de novo")
	}
}

func TestReconcileExternal_LinksManualRecurringInSameMonth(t *testing.T) {
	repo, pg := newTestRepo(t)
	manual := manualExpense(t, repo, pg, 543.21, domain.PurchaseTypeRecurring)

	now := time.Now()
	ok, err := repo.ReconcileExternal(context.Background(), externalTx("of-rec", 543.21, time.Date(now.Year(), now.Month(), 20, 0, 0, 0, 0, time.UTC)))
	if err != nil || !ok {
		t.Fatalf("recorrência do mês deveria conciliar: ok=%v err=%v", ok, err)
	}
	if id := externalIDOf(t, pg, manual.ID); id == nil || *id != "of-rec" {
		t.Errorf("external_id não gravado: %v", id)
	}
}

func TestSaveExternal_AndExists(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()

	save := func() error {
		desc := "teste-open-finance"
		p, _ := domain.NewPurchase(12.34, &desc, domain.CategoryMarket, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		cleanup(t, pg, p)
		id := "of-save-1"
		pay := domain.NewPayment(p.ID, 12.34, domain.PaymentStatusPaid)
		pay.ExternalID = &id
		return repo.SaveExternal(ctx, p, pay)
	}

	if exists, _ := repo.ExistsExternalID(ctx, "of-save-1"); exists {
		t.Fatal("banco de teste sujo: of-save-1 já existe")
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	if exists, err := repo.ExistsExternalID(ctx, "of-save-1"); err != nil || !exists {
		t.Errorf("deveria existir: %v %v", exists, err)
	}
	// O índice único é a última barreira contra duplicidade, e a compra não pode ficar órfã.
	if err := save(); err == nil {
		t.Error("segunda gravação do mesmo external_id deveria falhar")
	}
	var n int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM purchases WHERE description = 'teste-open-finance' AND total_amount = 12.34`).Scan(&n)
	if n != 1 {
		t.Errorf("a transação falha deveria ter sido revertida por inteiro, compras=%d", n)
	}
}
