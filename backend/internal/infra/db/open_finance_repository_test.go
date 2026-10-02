package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/settings"
	"github.com/google/uuid"
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

// testAccount cria (e remove ao final) uma conta de teste e devolve seu ID.
func testAccount(t *testing.T, repo *PostgresPurchaseRepository, pg *DB) uuid.UUID {
	t.Helper()
	id, err := repo.UpsertAccount(context.Background(), ports.ExternalAccount{ID: "of-test-acc", ItemID: "of-test", Type: "BANK", Name: "Conta teste", Last4: "1234"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM accounts WHERE id = $1`, id) })
	return id
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
	acc := testAccount(t, repo, pg)
	ctx := context.Background()
	manual := manualExpense(t, repo, pg, 987.65, domain.PurchaseTypeSingle)

	// Fora da janela de 3 dias, de outro valor ou de outro tipo: não concilia.
	for name, tx := range map[string]ports.ExternalTransaction{
		"data distante":   externalTx("of-far", 987.65, time.Now().AddDate(0, 0, -10)),
		"valor diferente": externalTx("of-amount", 987.66, time.Now()),
		"outro tipo":      {ID: "of-kind", Date: time.Now(), Amount: 987.65, Kind: domain.KindIncome},
	} {
		if ok, err := repo.ReconcileExternal(ctx, tx, acc); err != nil || ok {
			t.Fatalf("%s: não deveria conciliar (ok=%v, err=%v)", name, ok, err)
		}
	}

	ok, err := repo.ReconcileExternal(ctx, externalTx("of-1", 987.65, time.Now().AddDate(0, 0, -2)), acc)
	if err != nil || !ok {
		t.Fatalf("deveria conciliar: ok=%v err=%v", ok, err)
	}
	if id := externalIDOf(t, pg, manual.ID); id == nil || *id != "of-1" {
		t.Errorf("external_id não gravado: %v", id)
	}

	// Cada lançamento manual só pode ser conciliado uma vez.
	if ok, _ := repo.ReconcileExternal(ctx, externalTx("of-2", 987.65, time.Now()), acc); ok {
		t.Error("um lançamento já conciliado não pode casar de novo")
	}
}

func TestReconcileExternal_LinksManualRecurringInSameMonth(t *testing.T) {
	repo, pg := newTestRepo(t)
	acc := testAccount(t, repo, pg)
	manual := manualExpense(t, repo, pg, 543.21, domain.PurchaseTypeRecurring)

	now := time.Now()
	ok, err := repo.ReconcileExternal(context.Background(), externalTx("of-rec", 543.21, time.Date(now.Year(), now.Month(), 20, 0, 0, 0, 0, time.UTC)), acc)
	if err != nil || !ok {
		t.Fatalf("recorrência do mês deveria conciliar: ok=%v err=%v", ok, err)
	}
	if id := externalIDOf(t, pg, manual.ID); id == nil || *id != "of-rec" {
		t.Errorf("external_id não gravado: %v", id)
	}
}

func TestSaveExternal_LinkAndAccount(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	save := func() error {
		desc := "teste-open-finance"
		p, _ := domain.NewPurchase(12.34, &desc, domain.CategoryMarket, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		cleanup(t, pg, p)
		id := "of-save-1"
		pay := domain.NewPayment(p.ID, 12.34, domain.PaymentStatusPaid)
		pay.ExternalID = &id
		return repo.SaveExternal(ctx, p, pay)
	}

	if found, _ := repo.RefreshExternal(ctx, externalTx("of-save-1", 12.34, time.Now()), acc); found {
		t.Fatal("banco de teste sujo: of-save-1 já existe")
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	// Salvo antes da tabela de contas (sem conta): a sincronização seguinte vincula.
	if found, err := repo.RefreshExternal(ctx, externalTx("of-save-1", 12.34, time.Now()), acc); err != nil || !found {
		t.Errorf("deveria existir: %v %v", found, err)
	}
	var linked *uuid.UUID
	pg.Pool.QueryRow(ctx, `SELECT account_id FROM payments WHERE external_id = 'of-save-1'`).Scan(&linked)
	if linked == nil || *linked != acc {
		t.Errorf("conta não vinculada: %v", linked)
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

func TestUpsertAccount_UpdatesInPlace(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	first := testAccount(t, repo, pg)

	limit := 5000.0
	second, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-acc", ItemID: "of-test", Type: "CREDIT", Name: "Cartão", Last4: "8670", Balance: 99.9, CreditLimit: &limit})
	if err != nil || second != first {
		t.Fatalf("deveria atualizar a mesma conta: %v %v %v", first, second, err)
	}
	var name string
	var bal float64
	pg.Pool.QueryRow(ctx, `SELECT name, balance FROM accounts WHERE id = $1`, first).Scan(&name, &bal)
	if name != "Cartão" || bal != 99.9 {
		t.Errorf("conta não atualizada: %s %v", name, bal)
	}
}

func TestRefreshExternal_PromotesOtherExpenseCategory(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	save := func(extID string, cat domain.Category) *domain.Purchase {
		t.Helper()
		desc := "teste-open-finance"
		p, _ := domain.NewPurchase(7.77, &desc, cat, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "[open finance]")
		cleanup(t, pg, p)
		pay := domain.NewPayment(p.ID, 7.77, domain.PaymentStatusPaid)
		pay.ExternalID = &extID
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		return p
	}
	categoryOf := func(p *domain.Purchase) string {
		var c string
		pg.Pool.QueryRow(ctx, `SELECT category FROM purchases WHERE id = $1`, p.ID).Scan(&c)
		return c
	}
	refresh := func(id string, cat domain.Category) bool {
		t.Helper()
		found, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: id, Category: cat}, acc)
		if err != nil {
			t.Fatal(err)
		}
		return found
	}

	other, set := save("of-cat-other", domain.CategoryOther), save("of-cat-food", domain.CategoryFood)

	// Receitas e transferências importadas sem categoria também são promovidas.
	income := save("of-cat-income", domain.CategoryOther)
	pg.Pool.Exec(ctx, `UPDATE purchases SET kind = 'INCOME' WHERE id = $1`, income.ID)
	if !refresh("of-cat-income", domain.CategorySalary) || categoryOf(income) != "SALARY" {
		t.Errorf("receita OTHER deveria virar SALARY: %s", categoryOf(income))
	}

	if !refresh("of-cat-other", domain.CategoryTransport) || categoryOf(other) != "TRANSPORT" {
		t.Errorf("OTHER deveria ser promovida: %s", categoryOf(other))
	}
	refresh("of-cat-food", domain.CategoryMarket)
	if categoryOf(set) != "FOOD" {
		t.Errorf("categoria já definida não pode ser trocada: %s", categoryOf(set))
	}
	other2 := save("of-cat-other2", domain.CategoryOther)
	refresh("of-cat-other2", domain.CategoryOther)
	if categoryOf(other2) != "OTHER" {
		t.Errorf("OTHER -> OTHER não muda nada: %s", categoryOf(other2))
	}
	if refresh("of-cat-inexistente", domain.CategoryFood) {
		t.Error("transação desconhecida não pode ser reportada como existente")
	}
}

func TestInstitutions_UpsertLogoAndAccounts(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	const item = "of-test-inst"
	t.Cleanup(func() {
		pg.Pool.Exec(ctx, `DELETE FROM accounts WHERE item_id = $1`, item)
		pg.Pool.Exec(ctx, `DELETE FROM institutions WHERE item_id = $1`, item)
	})

	id, needsLogo, err := repo.UpsertInstitution(ctx, item, ports.ExternalInstitution{Name: "Banco Teste", Color: "ec0000"})
	if err != nil || !needsLogo {
		t.Fatalf("primeira vez deveria pedir o logo: %v %v", needsLogo, err)
	}
	if id2, _, _ := repo.UpsertInstitution(ctx, item, ports.ExternalInstitution{Name: "Banco Teste 2"}); id2 != id {
		t.Error("a instituição não pode duplicar")
	}

	// Falha na busca: registra a tentativa e só repete depois de 1 dia.
	if err := repo.SaveInstitutionLogo(ctx, id, nil, ""); err != nil {
		t.Fatal(err)
	}
	if _, needs, _ := repo.UpsertInstitution(ctx, item, ports.ExternalInstitution{Name: "Banco Teste"}); needs {
		t.Error("logo com busca recente não deveria ser pedido de novo")
	}
	if _, _, found, _ := repo.InstitutionLogo(ctx, id); found {
		t.Error("sem logo não deveria ser encontrado")
	}

	if err := repo.SaveInstitutionLogo(ctx, id, []byte("<svg/>"), "image/svg+xml"); err != nil {
		t.Fatal(err)
	}
	data, mime, found, err := repo.InstitutionLogo(ctx, id)
	if err != nil || !found || mime != "image/svg+xml" || string(data) != "<svg/>" {
		t.Fatalf("logo: %q %q %v %v", data, mime, found, err)
	}
	// Uma falha posterior não apaga o logo que já existe.
	repo.SaveInstitutionLogo(ctx, id, nil, "") //nolint:errcheck
	if _, _, found, _ := repo.InstitutionLogo(ctx, id); !found {
		t.Error("tentativa falha não pode apagar o logo")
	}
	if err := repo.SaveInstitutionLogo(ctx, id, []byte("x"), "text/html"); err == nil {
		t.Error("tipo fora da lista deveria ser recusado pelo banco")
	}

	// Conta com instituição e dados do cartão.
	due := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	min := 120.0
	if _, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-card", ItemID: item, Type: "CREDIT", Name: "Cartão",
		Brand: "VISA", DueDate: &due, MinimumPayment: &min, InstitutionID: &id}); err != nil {
		t.Fatal(err)
	}
	// Sync sem a instituição (conector falhou) não desvincula a conta.
	if _, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-card", ItemID: item, Type: "CREDIT", Name: "Cartão", Brand: "VISA", DueDate: &due}); err != nil {
		t.Fatal(err)
	}
	accounts, err := repo.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accounts {
		if a.ItemID == item {
			if a.InstitutionID == nil || *a.InstitutionID != id || a.Brand != "VISA" || a.DueDate == nil || !a.DueDate.Equal(due) {
				t.Errorf("conta inesperada: %+v", a)
			}
			insts, err := repo.Institutions(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, i := range insts {
				if i.ID == id && (!i.HasLogo || i.Color != "ec0000") {
					t.Errorf("instituição inesperada: %+v", i)
				}
			}
			return
		}
	}
	t.Fatal("conta não encontrada")
}

func TestInstitutions_ColorCheck(t *testing.T) {
	repo, pg := newTestRepo(t)
	const item = "of-test-color"
	t.Cleanup(func() { pg.Pool.Exec(context.Background(), `DELETE FROM institutions WHERE item_id = $1`, item) })
	if _, _, err := repo.UpsertInstitution(context.Background(), item, ports.ExternalInstitution{Name: "B", Color: "red;x"}); err == nil {
		t.Error("cor fora do padrão hexadecimal deveria ser recusada pelo banco")
	}
}

// As grafias do mesmo comércio no débito do Itaú caem na mesma conta, e o rótulo mostra o comércio.
func TestExpenseKey_DebitPrefix(t *testing.T) {
	_, pg := newTestRepo(t)
	ctx := context.Background()
	for desc, want := range map[string]string{
		"DEBITO VISA ELECTRON BRASIL   20/09 NETFLIX ENTRETENIME": "debito netflix",
		"DEBITO VISA ELECTRON BRASIL   20/08 NETFLIX.COM":         "debito netflix",
		"DEBITO VISA ELECTRON BRASIL   13/07 EBN         .SPOTIF": "debito ebn",
		"Pix enviado FULANO DE TAL":                               "pix enviado fulano de tal",
		"OTICA VENDRAMEBIRI01/12 (1/12)":                          "otica vendramebiri",
	} {
		var key, label string
		if err := pg.Pool.QueryRow(ctx, `SELECT `+expenseKey+`, `+cleanDescription+` FROM (SELECT $1::text AS description) p`, desc).Scan(&key, &label); err != nil {
			t.Fatal(err)
		}
		if key != want {
			t.Errorf("%q: chave %q, quer %q", desc, key, want)
		}
		if strings.HasPrefix(desc, "DEBITO") && strings.Contains(label, "DEBITO") {
			t.Errorf("%q: rótulo ainda tem o prefixo: %q", desc, label)
		}
	}
}

// Parcela já gravada na data da compra passa para o mês da fatura quando a sincronização a reconhece de novo.
func TestRefreshExternal_MovesInstallmentDate(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-inst-acc", ItemID: "of-test", Type: "CREDIT", Name: "Cartão"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM accounts WHERE external_id = 'of-test-inst-acc'`) })

	purchase := manualExpense(t, repo, pg, 143.08, domain.PurchaseTypeSingle)
	buy := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	if _, err := pg.Pool.Exec(ctx, `UPDATE payments SET external_id = 'of-test-parcela', due_date = $2::date, paid_at = $2::timestamptz WHERE purchase_id = $1`, purchase.ID, buy); err != nil {
		t.Fatal(err)
	}

	moved := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	if ok, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: "of-test-parcela", Date: moved, Installment: true, Category: domain.CategoryOther}, acc); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var due, paid time.Time
	if err := pg.Pool.QueryRow(ctx, `SELECT due_date, paid_at FROM payments WHERE external_id = 'of-test-parcela'`).Scan(&due, &paid); err != nil {
		t.Fatal(err)
	}
	if due.Format("2006-01-02") != "2026-09-05" || paid.UTC().Format("2006-01-02") != "2026-09-05" {
		t.Errorf("due=%v paid=%v", due, paid)
	}

	// Lançamento comum não muda de data.
	if _, err := repo.RefreshExternal(ctx, ports.ExternalTransaction{ID: "of-test-parcela", Date: buy, Installment: false, Category: domain.CategoryOther}, acc); err != nil {
		t.Fatal(err)
	}
	pg.Pool.QueryRow(ctx, `SELECT due_date FROM payments WHERE external_id = 'of-test-parcela'`).Scan(&due) //nolint:errcheck
	if due.Format("2006-01-02") != "2026-09-05" {
		t.Errorf("só parcela é movida: %v", due)
	}
}

func TestSettingsStore_RoundTrip(t *testing.T) {
	_, pg := newTestRepo(t)
	ctx := context.Background()
	st := NewSettingsStore(pg)
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM settings WHERE key = 'DIGEST_HOUR'`) })

	if err := st.SaveSetting(ctx, settings.Row{Key: "DIGEST_HOUR", Value: "7"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSetting(ctx, settings.Row{Key: "DIGEST_HOUR", Value: "8", Secret: true}); err != nil { // atualiza no lugar
		t.Fatal(err)
	}
	rows, err := st.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, r := range rows {
		if r.Key == "DIGEST_HOUR" {
			found++
			if r.Value != "8" || !r.Secret {
				t.Errorf("linha: %+v", r)
			}
		}
	}
	if found != 1 {
		t.Errorf("a chave não pode duplicar: %d", found)
	}
	if err := st.SaveSetting(ctx, settings.Row{Key: "chave minuscula", Value: "x"}); err == nil {
		t.Error("o banco recusa chaves fora do padrão")
	}
	if err := st.DeleteSetting(ctx, "DIGEST_HOUR"); err != nil {
		t.Fatal(err)
	}
}

func TestOwnTransferCandidatesAndCancel(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc, err := repo.UpsertAccount(ctx, ports.ExternalAccount{ID: "of-test-own-acc", ItemID: "of-test", Type: "BANK", Name: "Conta"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM accounts WHERE external_id = 'of-test-own-acc'`) })

	mk := func(extID, desc string, kind domain.PurchaseKind) *domain.Purchase {
		var p *domain.Purchase
		var err error
		if kind == domain.KindIncome {
			p, err = domain.NewIncome(10, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		} else {
			p, err = domain.NewPurchase(10, &desc, domain.CategoryOther, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "raw")
		}
		if err != nil {
			t.Fatal(err)
		}
		day := time.Date(1999, 3, 1, 0, 0, 0, 0, time.UTC)
		pay := domain.NewPayment(p.ID, 10, domain.PaymentStatusPaid)
		pay.DueDate, pay.PaidAt, pay.ExternalID, pay.AccountID = &day, &day, &extID, &acc
		if err := repo.SaveExternal(ctx, p, pay); err != nil {
			t.Fatal(err)
		}
		cleanup(t, pg, p)
		return p
	}
	mk("of-own-1", "Pix enviado ZZ PESSOA TESTE", domain.KindExpense)
	mk("of-own-2", "Pix recebido ZZ PESSOA TESTE", domain.KindIncome)
	mk("of-own-3", "Compra débito PADARIA ZZ", domain.KindExpense) // não é transferência

	cands, err := repo.OwnTransferCandidates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	for _, c := range cands {
		if strings.Contains(c.Description, "ZZ PESSOA TESTE") {
			ids = append(ids, c.PaymentID)
		}
		if strings.Contains(c.Description, "PADARIA ZZ") {
			t.Error("compra comum não é candidata")
		}
	}
	if len(ids) != 2 {
		t.Fatalf("esperava 2 candidatas, got %d", len(ids))
	}
	n, err := repo.CancelPayments(ctx, ids)
	if err != nil || n != 2 {
		t.Fatalf("cancelados %d, %v", n, err)
	}
	if again, _ := repo.CancelPayments(ctx, ids); again != 0 {
		t.Errorf("cancelar de novo não muda nada: %d", again)
	}
	after, _ := repo.OwnTransferCandidates(ctx)
	for _, c := range after {
		if strings.Contains(c.Description, "ZZ PESSOA TESTE") {
			t.Error("cancelada não é mais candidata")
		}
	}
}
