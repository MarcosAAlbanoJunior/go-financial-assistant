package db

import (
	"context"
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
