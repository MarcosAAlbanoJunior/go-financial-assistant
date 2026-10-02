package db

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
	"github.com/google/uuid"
)

// Os testes usam março/abril de 1999 para não se misturar com dados reais do banco.
var (
	mar1999 = time.Date(1999, 3, 1, 0, 0, 0, 0, time.UTC)
	apr1999 = time.Date(1999, 4, 1, 0, 0, 0, 0, time.UTC)
)

type seedEntry struct {
	kind      domain.PurchaseKind
	dir       domain.TransferDirection
	cat       domain.Category
	desc      string
	amount    float64
	date      time.Time
	status    domain.PaymentStatus
	accountID *uuid.UUID
}

func seed(t *testing.T, repo *PostgresPurchaseRepository, pg *DB, e seedEntry) {
	t.Helper()
	var (
		p   *domain.Purchase
		err error
	)
	switch e.kind {
	case domain.KindIncome:
		p, err = domain.NewIncome(e.amount, &e.desc, e.cat, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "teste-dashboard")
	case domain.KindTransfer:
		p, err = domain.NewTransfer(e.amount, &e.desc, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "teste-dashboard", e.dir)
	default:
		p, err = domain.NewPurchase(e.amount, &e.desc, e.cat, domain.PaymentMethodPix, domain.PurchaseTypeSingle, "teste-dashboard")
	}
	if err != nil {
		t.Fatal(err)
	}
	status := e.status
	if status == "" {
		status = domain.PaymentStatusPaid
	}
	pay := domain.NewPayment(p.ID, e.amount, status)
	pay.DueDate, pay.AccountID = &e.date, e.accountID
	if err := repo.SaveExternal(context.Background(), p, pay); err != nil {
		t.Fatal(err)
	}
	cleanup(t, pg, p)
}

func TestDashboard_MonthlyTotalsAndInvestments(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()

	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "dash-a", amount: 40, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "dash-cancel", amount: 999, date: mar1999, status: domain.PaymentStatusCancelled})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "dash-b", amount: 500, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindTransfer, dir: domain.TransferDirectionOut, desc: "dash-c", amount: 100, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindTransfer, dir: domain.TransferDirectionIn, desc: "dash-d", amount: 30, date: apr1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindTransfer, dir: domain.TransferDirectionOut, desc: "dash-e", amount: 50, date: apr1999.AddDate(0, 1, 0)})

	totals, err := repo.MonthlyTotals(ctx, mar1999.AddDate(0, -1, 0), apr1999)
	if err != nil || len(totals) != 3 {
		t.Fatalf("esperava 3 meses (um deles vazio): %v %v", totals, err)
	}
	if z := totals[0]; z.Income != 0 || z.Expense != 0 || z.Applied != 0 {
		t.Errorf("mês sem lançamentos deveria vir zerado: %+v", z)
	}
	if m := totals[1]; m.Income != 500 || m.Expense != 40 || m.Applied != 100 || m.Redeemed != 0 {
		t.Errorf("março: %+v (a despesa cancelada não conta)", m)
	}
	if m := totals[2]; m.Redeemed != 30 || m.Expense != 0 {
		t.Errorf("abril: %+v", m)
	}

	// O acumulado vem desde o primeiro lançamento, mesmo recortando a janela só em abril/maio.
	inv, err := repo.InvestmentSeries(ctx, apr1999, apr1999.AddDate(0, 1, 0))
	if err != nil || len(inv) != 2 {
		t.Fatalf("série de investimentos: %v %v", inv, err)
	}
	if inv[0].Cumulative != 70 || inv[0].Redeemed != 30 || inv[1].Cumulative != 120 || inv[1].Applied != 50 {
		t.Errorf("acumulado incorreto: %+v", inv)
	}
}

func TestDashboard_BreakdownAndAccounts(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "dash-f", amount: 30, date: mar1999, accountID: &acc})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryMarket, desc: "dash-g", amount: 20, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "dash-h", amount: 900, date: mar1999})

	byCat, err := repo.ExpenseBreakdown(ctx, mar1999, ports.BreakdownByCategory)
	if err != nil || len(byCat) != 2 || byCat[0].Key != "FOOD" || byCat[0].Total != 30 {
		t.Errorf("por categoria: %+v %v", byCat, err)
	}
	byAcc, err := repo.ExpenseBreakdown(ctx, mar1999, ports.BreakdownByAccount)
	if err != nil || len(byAcc) != 2 || byAcc[0].Key != acc.String() || byAcc[0].Name != "Conta teste 1234" || byAcc[1].Key != "" {
		t.Errorf("por conta: %+v %v", byAcc, err)
	}
	if _, err := repo.ExpenseBreakdown(ctx, mar1999, "x; DROP TABLE payments"); err == nil {
		t.Error("agrupamento desconhecido deve ser recusado")
	}

	accounts, err := repo.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range accounts {
		found = found || a.ID == acc
	}
	if !found {
		t.Error("conta de teste não listada")
	}
}

func TestDashboard_TransactionsFilterAndPaging(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	acc := testAccount(t, repo, pg)

	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "Padaria 100%_ok", amount: 10, date: mar1999.AddDate(0, 0, 1), accountID: &acc})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "Padariaxyz", amount: 11, date: mar1999.AddDate(0, 0, 2)})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "Salário", amount: 900, date: mar1999.AddDate(0, 0, 3)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "outra", amount: 5, date: apr1999})

	list := func(f ports.TransactionFilter) ([]ports.Transaction, int) {
		t.Helper()
		f.Month = &mar1999
		if f.Limit == 0 {
			f.Limit = 50
		}
		txs, total, err := repo.Transactions(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return txs, total
	}

	if txs, total := list(ports.TransactionFilter{}); total != 3 || len(txs) != 3 || txs[0].Description != "Salário" {
		t.Errorf("mais recentes primeiro, só março: %d %+v", total, txs)
	}
	if _, total := list(ports.TransactionFilter{Kind: "EXPENSE"}); total != 2 {
		t.Errorf("filtro por tipo: %d", total)
	}
	if txs, total := list(ports.TransactionFilter{AccountID: &acc}); total != 1 || txs[0].AccountName != "Conta teste 1234" {
		t.Errorf("filtro por conta: %d %+v", total, txs)
	}
	// % e _ da busca valem literalmente: "100%_" não pode casar "Padariaxyz".
	if txs, total := list(ports.TransactionFilter{Search: "100%_"}); total != 1 || txs[0].Description != "Padaria 100%_ok" {
		t.Errorf("curingas devem ser escapados: %d %+v", total, txs)
	}
	// Paginação: o total reflete o filtro inteiro, não só a página.
	if txs, total := list(ports.TransactionFilter{Limit: 2, Offset: 2}); total != 3 || len(txs) != 1 {
		t.Errorf("paginação: total=%d len=%d", total, len(txs))
	}
}

func TestInvestments_SaveSnapshotsAndHistory(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	const item = "of-test-inv-item"
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM investments WHERE item_id = $1`, item) })

	save := func(day time.Time, positions ...ports.ExternalInvestment) {
		t.Helper()
		if err := repo.SaveInvestments(ctx, item, positions, day); err != nil {
			t.Fatal(err)
		}
	}
	cdb := func(balance float64) ports.ExternalInvestment {
		return ports.ExternalInvestment{ID: "of-test-cdb", Type: "FIXED_INCOME", Subtype: "CDB", Name: "CDB teste", Balance: balance, Amount: balance + 10}
	}
	fund := func(balance float64) ports.ExternalInvestment {
		return ports.ExternalInvestment{ID: "of-test-fund", Type: "MUTUAL_FUND", Name: "Fundo teste", Balance: balance}
	}

	// Março: duas posições (a primeira com dois saldos no mês: vale o último). Abril: o fundo foi resgatado.
	save(mar1999, cdb(1000), fund(500))
	save(mar1999.AddDate(0, 0, 20), cdb(1010), fund(500))
	save(apr1999.AddDate(0, 0, 5), cdb(1020))

	var active, total int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE active), COUNT(*) FROM investments WHERE item_id = $1`, item).Scan(&active, &total)
	if active != 1 || total != 2 {
		t.Errorf("o fundo resgatado deveria ficar inativo: ativas=%d total=%d", active, total)
	}

	hist, err := repo.PortfolioHistory(ctx, mar1999.AddDate(0, -1, 0), apr1999)
	if err != nil || len(hist) != 3 {
		t.Fatalf("histórico: %v %v", hist, err)
	}
	if hist[0].Balance != nil {
		t.Errorf("antes do primeiro registro o saldo é desconhecido (nil), got %v", *hist[0].Balance)
	}
	if hist[1].Balance == nil || *hist[1].Balance != 1510 {
		t.Errorf("março = 1010 + 500, got %v", hist[1].Balance)
	}
	if hist[2].Balance == nil || *hist[2].Balance != 1020 {
		t.Errorf("abril = 1020 + 0 (resgatado), got %v", hist[2].Balance)
	}

	// Posição que volta a aparecer é reativada.
	save(apr1999.AddDate(0, 0, 10), cdb(1030), fund(5))
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE active) FROM investments WHERE item_id = $1`, item).Scan(&active)
	if active != 2 {
		t.Errorf("posição que voltou deveria ser reativada, ativas=%d", active)
	}

	positions, err := repo.Positions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range positions {
		if p.Name == "CDB teste" && (p.Balance != 1030 || p.Amount != 1040 || p.Subtype != "CDB") {
			t.Errorf("posição incorreta: %+v", p)
		}
	}
}

func TestInvestments_EstimatesMonthsBeforeFirstSnapshot(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	const item = "of-test-est-item"
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM investments WHERE item_id = $1`, item) })

	day := func(m time.Month, d int) time.Time { return time.Date(1999, m, d, 0, 0, 0, 0, time.UTC) }
	mv := func(id string, m time.Month, d int, amount float64) ports.ExternalMovement {
		return ports.ExternalMovement{ID: id, Day: day(m, d), Amount: amount}
	}
	// A: aplicou 1000 em fev e 100 em mar. B: aplicou 500 em jan e resgatou 150 em mar.
	// Primeira sincronização em 15/abr, com saldos 1100 e 400.
	a := ports.ExternalInvestment{ID: "of-test-est-a", Type: "FIXED_INCOME", Name: "A", Balance: 1100,
		Movements: []ports.ExternalMovement{mv("a1", time.February, 10, 1000), mv("a2", time.March, 20, 100)}}
	b := ports.ExternalInvestment{ID: "of-test-est-b", Type: "FIXED_INCOME", Name: "B", Balance: 400,
		Movements: []ports.ExternalMovement{mv("b1", time.January, 5, 500), mv("b2", time.March, 10, -150)}}
	if err := repo.SaveInvestments(ctx, item, []ports.ExternalInvestment{a, b}, day(time.April, 15)); err != nil {
		t.Fatal(err)
	}
	// Gravar de novo não duplica movimentações.
	if err := repo.SaveInvestments(ctx, item, []ports.ExternalInvestment{a, b}, day(time.April, 16)); err != nil {
		t.Fatal(err)
	}
	var n int
	pg.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM investment_movements m JOIN investments i ON i.id = m.investment_id WHERE i.item_id = $1`, item).Scan(&n)
	if n != 4 {
		t.Errorf("movimentações duplicadas: %d", n)
	}

	hist, err := repo.PortfolioHistory(ctx, time.Date(1998, 12, 1, 0, 0, 0, 0, time.UTC), apr1999)
	if err != nil || len(hist) != 5 {
		t.Fatalf("histórico: %v %v", hist, err)
	}
	want := []struct {
		balance   *float64
		estimated bool
	}{
		{nil, true},          // dez/98: nada existia
		{ptrTo(550), true},   // jan: só B (500 aplicados; o resgate de 150 em mar ainda não tinha ocorrido => 400+150)
		{ptrTo(1550), true},  // fev: A = 1100 - 100 = 1000; B = 550
		{ptrTo(1500), true},  // mar: A = 1100; B = 400
		{ptrTo(1500), false}, // abr: saldo exato gravado
	}
	for i, w := range want {
		got := hist[i]
		if got.Estimated != w.estimated || (got.Balance == nil) != (w.balance == nil) || (w.balance != nil && *got.Balance != *w.balance) {
			t.Errorf("mês %d: got balance=%v estimated=%v, esperado %v/%v", i, deref(got.Balance), got.Estimated, deref(w.balance), w.estimated)
		}
	}
}

func ptrTo(v float64) *float64 { return &v }

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestDashboard_TransactionGroupsAndDayFilter(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()

	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "grp-a", amount: 30, date: mar1999.AddDate(0, 0, 1)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "grp-b", amount: 20, date: mar1999.AddDate(0, 0, 1)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryMarket, desc: "grp-c", amount: 70, date: mar1999.AddDate(0, 0, 4)})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "grp-d", amount: 900, date: mar1999.AddDate(0, 0, 4)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "grp-e", amount: 9, date: apr1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "grp-cancel", amount: 500, date: mar1999, status: domain.PaymentStatusCancelled})

	f := ports.TransactionFilter{Month: &mar1999}
	byCat, err := repo.TransactionGroups(ctx, f, ports.GroupByCategory)
	if err != nil || len(byCat) != 3 {
		t.Fatalf("por categoria: %+v %v", byCat, err)
	}
	if byCat[0].Key != "MARKET" || byCat[0].Expense != 70 || byCat[1].Key != "FOOD" || byCat[1].Expense != 50 || byCat[1].Count != 2 {
		t.Errorf("ordem e somas: %+v (o cancelado e abril não contam)", byCat)
	}
	if byCat[2].Key != "SALARY" || byCat[2].Income != 900 || byCat[2].Expense != 0 {
		t.Errorf("renda separada da despesa: %+v", byCat[2])
	}

	byDay, err := repo.TransactionGroups(ctx, f, ports.GroupByDay)
	if err != nil || len(byDay) != 2 || byDay[0].Key != "1999-03-05" || byDay[1].Key != "1999-03-02" || byDay[1].Expense != 50 {
		t.Errorf("por dia (mais recente primeiro): %+v %v", byDay, err)
	}

	// Mesmos filtros da lista: tipo e busca restringem os grupos.
	onlyFood := ports.TransactionFilter{Month: &mar1999, Kind: "EXPENSE", Search: "grp-a"}
	if g, _ := repo.TransactionGroups(ctx, onlyFood, ports.GroupByCategory); len(g) != 1 || g[0].Expense != 30 {
		t.Errorf("filtro por tipo e busca: %+v", g)
	}

	day := mar1999.AddDate(0, 0, 1)
	txs, total, err := repo.Transactions(ctx, ports.TransactionFilter{Day: &day, Limit: 50})
	if err != nil || total != 2 || len(txs) != 2 {
		t.Errorf("filtro por dia: total=%d %v", total, err)
	}
	if _, err := repo.TransactionGroups(ctx, f, "x; DROP TABLE payments"); err == nil {
		t.Error("agrupamento desconhecido deve ser recusado")
	}
}

func TestBudget_ExpenseKeyMonthsAndRules(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM expense_rules WHERE key LIKE 'zzteste%'`) })

	jan, feb := time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(1999, 2, 1, 0, 0, 0, 0, time.UTC)
	// A mesma conta em dois meses, com números e datas diferentes na descrição: vira uma chave só.
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryEntertainment, desc: "ZZTeste Assinatura 03/01", amount: 20, date: jan.AddDate(0, 0, 4)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryEntertainment, desc: "ZZTeste Assinatura 03/02", amount: 20, date: feb.AddDate(0, 0, 9), status: domain.PaymentStatusPending})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryShopping, desc: "ZZTeste Geladeira (2/12)", amount: 150, date: feb})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "ZZTeste salário", amount: 900, date: feb})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "ZZTeste cancelada", amount: 5, date: feb, status: domain.PaymentStatusCancelled})

	rows, err := repo.ExpenseKeyMonths(ctx, jan, feb)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]ports.ExpenseKeyMonth{}
	for _, r := range rows {
		got[r.Key] = append(got[r.Key], r)
	}
	sub := got["zzteste assinatura"]
	if len(sub) != 2 || sub[0].Total != 20 || sub[0].Day != 5 || !sub[0].AllPaid || sub[1].AllPaid || sub[1].Day != 10 || sub[0].Installment {
		t.Errorf("assinatura (mesma chave nos dois meses, o 2º pendente): %+v", sub)
	}
	if sub[0].Category != "ENTERTAINMENT" || sub[0].Label != "ZZTeste Assinatura 03/01" {
		t.Errorf("categoria e descrição de exemplo: %+v", sub[0])
	}
	if p := got["zzteste geladeira"]; len(p) != 1 || !p[0].Installment {
		t.Errorf("(2/12) é parcelada: %+v", p)
	}
	if len(got["zzteste sal rio"])+len(got["zzteste salario"])+len(got["zzteste cancelada"]) != 0 {
		t.Errorf("renda e cancelada não entram: %+v", got)
	}

	if err := repo.SetExpenseRule(ctx, "zzteste assinatura", ports.ClassFixed); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetExpenseRule(ctx, "zzteste assinatura", ports.ClassVariable); err != nil { // atualiza
		t.Fatal(err)
	}
	rules, _ := repo.ExpenseRules(ctx)
	if rules["zzteste assinatura"] != ports.ClassVariable {
		t.Errorf("regra gravada: %v", rules)
	}
	if err := repo.SetExpenseRule(ctx, "zzteste assinatura", ""); err != nil {
		t.Fatal(err)
	}
	if rules, _ = repo.ExpenseRules(ctx); len(rules["zzteste assinatura"]) != 0 {
		t.Errorf("AUTO apaga a regra: %v", rules)
	}
}

func TestBudget_KnownInstallments(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM purchases WHERE raw_input = 'teste-parcelas'`) })

	// Compra parcelada cadastrada à mão: 3 parcelas de 80 (uma já cancelada) em meses de 1999.
	desc := "Compra parcelada teste"
	p, err := domain.NewPurchase(240, &desc, domain.CategoryShopping, domain.PaymentMethodCreditCard, domain.PurchaseTypeInstallment, "teste-parcelas")
	if err != nil {
		t.Fatal(err)
	}
	var pays []domain.Payment
	for i, st := range []domain.PaymentStatus{domain.PaymentStatusPending, domain.PaymentStatusPending, domain.PaymentStatusCancelled} {
		due := mar1999.AddDate(0, i, 0)
		pay := domain.NewPayment(p.ID, 80, st)
		pay.DueDate = &due
		pays = append(pays, *pay)
	}
	if err := repo.Save(ctx, p, pays); err != nil {
		t.Fatal(err)
	}
	// Avulsa no mesmo mês não entra.
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "avulsa", amount: 999, date: mar1999})

	got, err := repo.KnownInstallments(ctx, mar1999, apr1999.AddDate(0, 1, 0))
	if err != nil || len(got) != 2 || got[mar1999] != 80 || got[apr1999] != 80 {
		t.Errorf("parcelas por mês (cancelada e avulsa ficam de fora): %v %v", got, err)
	}
}

func TestBudget_IncomePayments(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()

	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "ZZSalário Empresa 03/01", amount: 6000, date: mar1999.AddDate(0, 0, 1)})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "ZZSalário Empresa 03/02", amount: 9000, date: mar1999.AddDate(0, 0, 20)})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "ZZSalário Empresa 04/01", amount: 6100, date: apr1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "ZZSalário despesa", amount: 50, date: mar1999})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "ZZSalário cancelado", amount: 1, date: mar1999, status: domain.PaymentStatusCancelled})

	got, err := repo.IncomePayments(ctx, mar1999, apr1999)
	if err != nil {
		t.Fatal(err)
	}
	var mine []ports.IncomePayment
	for _, p := range got {
		if p.Key == "zzsal rio empresa" || p.Key == "zzsalário empresa" {
			mine = append(mine, p)
		}
	}
	if len(mine) != 3 {
		t.Fatalf("3 recebimentos da mesma fonte (chave sem números), despesa e cancelado de fora: %+v", got)
	}
	if mine[0].Amount != 9000 || mine[0].Month.Month() != time.March || mine[2].Month.Month() != time.April {
		t.Errorf("ordenado por mês e valor, com o valor de cada pagamento: %+v", mine)
	}
}

func TestReview_CategoryMonthsPaymentsAndDismissals(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM review_dismissals WHERE key LIKE 'zzteste%'`) })

	jan, feb := time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(1999, 2, 1, 0, 0, 0, 0, time.UTC)
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "ZZTeste Lanche 01", amount: 10, date: jan.AddDate(0, 0, 2)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "ZZTeste Lanche 02", amount: 15, date: feb.AddDate(0, 0, 3)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryFood, desc: "ZZTeste Lanche 03", amount: 15, date: feb.AddDate(0, 0, 1)})
	seed(t, repo, pg, seedEntry{kind: domain.KindExpense, cat: domain.CategoryTransport, desc: "ZZTeste cancelada", amount: 99, date: feb, status: domain.PaymentStatusCancelled})
	seed(t, repo, pg, seedEntry{kind: domain.KindIncome, cat: domain.CategorySalary, desc: "ZZTeste renda", amount: 500, date: feb})

	months, err := repo.CategoryMonths(ctx, jan, feb)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, m := range months {
		got[m.Category+m.Month.Format("-01")] = m.Total
	}
	if got["FOOD-01"] != 10 || got["FOOD-02"] != 30 || got["TRANSPORT-02"] != 0 || got["SALARY-02"] != 0 {
		t.Errorf("despesa por categoria e mês (sem renda nem cancelada): %v", got)
	}

	pays, err := repo.ExpensePayments(ctx, feb, feb)
	if err != nil {
		t.Fatal(err)
	}
	var mine []ports.ExpensePayment
	for _, p := range pays {
		if p.Key == "zzteste lanche" {
			mine = append(mine, p)
		}
	}
	if len(mine) != 2 || mine[0].Amount != 15 || !mine[0].Date.Before(mine[1].Date) || mine[0].Category != "FOOD" || mine[0].PaymentMethod != "PIX" {
		t.Errorf("despesas individuais de fevereiro, em ordem de data: %+v", mine)
	}

	d := ports.Dismissal{Kind: "ANT", Key: "zzteste lanche"}
	for range 2 { // dispensar duas vezes não falha nem duplica
		if err := repo.SetDismissal(ctx, d, true); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := repo.Dismissals(ctx)
	found := 0
	for _, x := range all {
		if x == d {
			found++
		}
	}
	if found != 1 {
		t.Errorf("sugestão dispensada: %+v", all)
	}
	if err := repo.SetDismissal(ctx, d, false); err != nil {
		t.Fatal(err)
	}
	if all, _ = repo.Dismissals(ctx); slices.Contains(all, d) {
		t.Errorf("restaurar apaga a dispensa: %+v", all)
	}
}

func TestGoals_CreateListDelete(t *testing.T) {
	repo, pg := newTestRepo(t)
	ctx := context.Background()
	t.Cleanup(func() { pg.Pool.Exec(ctx, `DELETE FROM goals WHERE name LIKE 'zzteste%'`) })

	save := ports.Goal{ID: uuid.New(), Kind: ports.GoalSave, Name: "zzteste viagem", TargetAmount: 5000.5, TargetDate: time.Date(1999, 12, 1, 0, 0, 0, 0, time.UTC)}
	cut := ports.Goal{ID: uuid.New(), Kind: ports.GoalCut, Name: "zzteste comida", Category: "FOOD", CutPercent: 15, Baseline: 800}
	reserve := ports.Goal{ID: uuid.New(), Kind: ports.GoalReserve, Name: "zzteste reserva", ReserveMonths: 6}
	for _, g := range []ports.Goal{save, cut, reserve} {
		if err := repo.CreateGoal(ctx, g); err != nil {
			t.Fatal(err)
		}
	}
	// O banco recusa meta sem os campos do tipo.
	if err := repo.CreateGoal(ctx, ports.Goal{ID: uuid.New(), Kind: ports.GoalSave, Name: "zzteste inválida"}); err == nil {
		t.Error("SAVE sem valor e data deveria falhar")
	}

	all, err := repo.Goals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]ports.Goal{}
	for _, g := range all {
		got[g.ID] = g
	}
	if g := got[save.ID]; g.TargetAmount != 5000.5 || !g.TargetDate.Equal(save.TargetDate) || g.Kind != ports.GoalSave || g.CreatedAt.IsZero() {
		t.Errorf("SAVE: %+v", g)
	}
	if g := got[cut.ID]; g.Category != "FOOD" || g.CutPercent != 15 || g.Baseline != 800 || !g.TargetDate.IsZero() {
		t.Errorf("CUT: %+v", g)
	}
	if g := got[reserve.ID]; g.ReserveMonths != 6 || g.TargetAmount != 0 {
		t.Errorf("RESERVE: %+v", g)
	}

	if ok, err := repo.DeleteGoal(ctx, save.ID); err != nil || !ok {
		t.Errorf("apagar existente: %v %v", ok, err)
	}
	if ok, err := repo.DeleteGoal(ctx, save.ID); err != nil || ok {
		t.Errorf("apagar de novo: %v %v", ok, err)
	}
}
