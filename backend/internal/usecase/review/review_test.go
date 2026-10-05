package review

import (
	"math"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

var reviewTo = month(2026, time.September)

func day(d int) time.Time { return time.Date(2026, time.September, d, 0, 0, 0, 0, time.UTC) }

func pay(key string, amount float64, d int, method string) domain.ExpensePayment {
	return domain.ExpensePayment{Key: key, Label: key, Category: "FOOD", PaymentMethod: method, Date: day(d), Amount: amount}
}

func candidatesOf(r Review, kind string) map[string]Candidate {
	out := map[string]Candidate{}
	for _, c := range r.Candidates {
		if c.Kind == kind {
			out[c.Key] = c
		}
	}
	return out
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestBuildReview_MatrixAndIncreases(t *testing.T) {
	cm := []domain.CategoryMonth{
		{Category: "FOOD", Month: month(2026, time.June), Total: 200},
		{Category: "FOOD", Month: month(2026, time.July), Total: 220},
		{Category: "FOOD", Month: month(2026, time.August), Total: 180},
		{Category: "FOOD", Month: reviewTo, Total: 400}, // média 200 -> +200 (100%)
		// Cada mês com dados conta, inclusive os zerados; por isso o histórico das outras categorias vem completo.
		{Category: "MARKET", Month: month(2026, time.June), Total: 500},
		{Category: "MARKET", Month: month(2026, time.July), Total: 500},
		{Category: "MARKET", Month: month(2026, time.August), Total: 500},
		{Category: "MARKET", Month: reviewTo, Total: 540}, // +40: abaixo de R$ 50
		{Category: "HEALTH", Month: month(2026, time.June), Total: 100},
		{Category: "HEALTH", Month: month(2026, time.July), Total: 100},
		{Category: "HEALTH", Month: month(2026, time.August), Total: 100},
		{Category: "HEALTH", Month: reviewTo, Total: 140},                 // +40 e só +40%: não passa pelo valor
		{Category: "OTHER", Month: month(2026, time.January), Total: 999}, // fora da janela da matriz
	}
	r := BuildReview(cm, nil, nil, nil, nil, reviewTo)

	if len(r.Months) != ReviewMatrixMonths || !r.Months[0].Equal(month(2026, time.April)) || !r.Months[5].Equal(reviewTo) {
		t.Fatalf("meses da matriz: %v", r.Months)
	}
	if len(r.Matrix) != 3 || r.Matrix[0].Category != "MARKET" || r.Matrix[1].Category != "FOOD" {
		t.Fatalf("matriz ordenada pelo total da janela, sem a categoria de fora dela: %+v", r.Matrix)
	}
	if r.Matrix[1].Values[5] != 400 || r.Matrix[1].Values[0] != 0 {
		t.Errorf("valores por mês: %v", r.Matrix[1].Values)
	}
	inc := candidatesOf(r, ReviewIncrease)
	if len(inc) != 1 || !near(inc["FOOD"].Saving, 200) || !near(inc["FOOD"].Baseline, 200) || !inc["FOOD"].Recurring {
		t.Errorf("só alimentação passa nos limiares (média dos 3 meses anteriores = 200): %+v", inc)
	}
}

func TestBuildReview_IncreaseNeedsHistory(t *testing.T) {
	cm := []domain.CategoryMonth{{Category: "FOOD", Month: reviewTo, Total: 900}}
	if r := BuildReview(cm, nil, nil, nil, nil, reviewTo); len(r.Candidates) != 0 {
		t.Errorf("sem mês anterior não há como comparar: %+v", r.Candidates)
	}
}

func TestBuildReview_FixedAnnualCost(t *testing.T) {
	var rows []domain.ExpenseKeyMonth
	rows = append(rows, monthsFrom("streaming", time.April, 40, 40, 40, 40, 40, 40)...)
	rows = append(rows, monthsFrom("luz", time.April, 180, 210, 195, 205, 190, 200)...)
	rows = append(rows, monthsFrom("mercado", time.April, 400, 900, 150, 700, 300, 800)...)
	r := BuildReview(nil, rows, nil, nil, nil, reviewTo)

	fixed := candidatesOf(r, ReviewFixed)
	if len(fixed) != 2 || fixed["mercado"].Key != "" {
		t.Fatalf("só as contas fixas: %+v", fixed)
	}
	if s := fixed["streaming"]; s.Saving != 40 || s.Months != 6 || !s.Recurring {
		t.Errorf("assinatura: %+v", s)
	}
	if r.Candidates[0].Key != "luz" || r.Candidates[1].Key != "streaming" {
		t.Errorf("ordenadas pelo impacto: %+v", r.Candidates)
	}
}

// rowsOf resume as despesas do mês em uma linha por conta, como o banco devolveria.
func rowsOf(ps []domain.ExpensePayment) []domain.ExpenseKeyMonth {
	idx := map[string]int{}
	var rows []domain.ExpenseKeyMonth
	for _, p := range ps {
		i, ok := idx[p.Key]
		if !ok {
			i = len(rows)
			idx[p.Key] = i
			rows = append(rows, domain.ExpenseKeyMonth{Key: p.Key, Label: p.Label, Category: p.Category, Month: reviewTo, AllPaid: true})
		}
		rows[i].Total += p.Amount
		rows[i].Count++
	}
	return rows
}

func TestBuildReview_Ant(t *testing.T) {
	var ps []domain.ExpensePayment
	for d := 1; d <= 5; d++ {
		ps = append(ps, pay("padaria", 18, d*3, "CREDIT_CARD")) // 5 x 18 = 90
	}
	for d := 1; d <= 6; d++ {
		ps = append(ps, pay("pix", 30, d, "PIX")) // Pix fica de fora
	}
	ps = append(ps, pay("cafe", 20, 1, "DEBIT_CARD"), pay("cafe", 20, 9, "DEBIT_CARD"), pay("cafe", 20, 20, "DEBIT_CARD")) // só 3 vezes
	for d := 1; d <= 4; d++ {
		ps = append(ps, pay("lojao", 41, d*5, "CREDIT_CARD")) // acima de R$ 40
	}
	ps = append(ps, pay("banca", 9, 1, "CASH"), pay("banca", 9, 8, "CASH"), pay("banca", 9, 15, "CASH"), pay("banca", 9, 22, "CASH")) // 4 x 9 = 36, abaixo de R$ 80

	ant := candidatesOf(BuildReview(nil, rowsOf(ps), nil, ps, nil, reviewTo), ReviewAnt)
	if len(ant) != 1 || ant["padaria"].Count != 5 || !near(ant["padaria"].Amount, 90) || !near(ant["padaria"].Saving, 45) {
		t.Errorf("só a padaria é gasto formiga (economia = metade de 90): %+v", ant)
	}
}

func TestBuildReview_Duplicates(t *testing.T) {
	ps := []domain.ExpensePayment{
		pay("oficina", 250, 2, "CREDIT_CARD"), pay("oficina", 250, 3, "CREDIT_CARD"), // duplicada
		pay("farmacia", 60, 1, "DEBIT_CARD"), pay("farmacia", 60, 10, "DEBIT_CARD"), // longe demais
		pay("lanche", 12, 5, "DEBIT_CARD"), pay("lanche", 15, 5, "DEBIT_CARD"), // valores diferentes
		pay("bala", 3, 5, "CASH"), pay("bala", 3, 5, "CASH"), // abaixo de R$ 5
		pay("taxi", 40, 7, "CREDIT_CARD"), pay("taxi", 40, 8, "CREDIT_CARD"), pay("taxi", 40, 10, "CREDIT_CARD"), // 3 em cadeia
	}
	ps[0].Label = "Oficina"
	dup := candidatesOf(BuildReview(nil, rowsOf(ps), nil, ps, nil, reviewTo), ReviewDuplicate)
	if len(dup) != 2 {
		t.Fatalf("oficina e táxi: %+v", dup)
	}
	if o := dup["oficina"]; !near(o.Saving, 250) || o.Recurring || o.Count != 2 || o.Label != "Oficina" {
		t.Errorf("duplicata avulsa, sem anualizar: %+v", o)
	}
	if x := dup["taxi"]; !near(x.Saving, 80) || x.Count != 3 {
		t.Errorf("três cobranças em cadeia = duas a mais: %+v", x)
	}
}

func TestBuildReview_DuplicateIgnoresInstallments(t *testing.T) {
	ps := []domain.ExpensePayment{pay("geladeira", 150, 2, "CREDIT_CARD"), pay("geladeira", 150, 3, "CREDIT_CARD")}
	rows := rowsOf(ps)
	rows[0].Installment = true
	if dup := candidatesOf(BuildReview(nil, rows, nil, ps, nil, reviewTo), ReviewDuplicate); len(dup) != 0 {
		t.Errorf("parcelas não são duplicata: %+v", dup)
	}
}

func TestBuildReview_NewBills(t *testing.T) {
	rows := monthsFrom("streaming", time.June, 40, 40, 40, 40) // até setembro: não é nova
	rows = append(rows, domain.ExpenseKeyMonth{Key: "curso", Label: "Curso", Month: reviewTo, Total: 120, Count: 1})
	rows = append(rows, domain.ExpenseKeyMonth{Key: "balinha", Month: reviewTo, Total: 8, Count: 1})                 // abaixo de R$ 30
	rows = append(rows, domain.ExpenseKeyMonth{Key: "tv", Month: reviewTo, Total: 300, Count: 1, Installment: true}) // parcelada
	nw := candidatesOf(BuildReview(nil, rows, nil, nil, nil, reviewTo), ReviewNew)
	if len(nw) != 1 || nw["curso"].Recurring || !near(nw["curso"].Saving, 120) {
		t.Errorf("só o curso é conta nova: %+v", nw)
	}

	// Com menos de 2 meses anteriores, tudo seria "novo": não sugere nada.
	short := []domain.ExpenseKeyMonth{
		{Key: "a", Month: month(2026, time.August), Total: 50, Count: 1},
		{Key: "curso", Month: reviewTo, Total: 120, Count: 1},
	}
	if nw := candidatesOf(BuildReview(nil, short, nil, nil, nil, reviewTo), ReviewNew); len(nw) != 0 {
		t.Errorf("histórico curto: %+v", nw)
	}
}

func TestBuildReview_DismissedAndOrder(t *testing.T) {
	rows := monthsFrom("streaming", time.April, 40, 40, 40, 40, 40, 40)
	rows = append(rows, monthsFrom("seguro", time.April, 90, 90, 90, 90, 90, 90)...)
	rows = append(rows, domain.ExpenseKeyMonth{Key: "curso", Month: reviewTo, Total: 500, Count: 1})
	r := BuildReview(nil, rows, nil, nil, []domain.Dismissal{{Kind: ReviewFixed, Key: "streaming"}, {Kind: ReviewNew, Key: "outra"}}, reviewTo)

	got := ""
	for _, c := range r.Candidates {
		got += c.Key
		if c.Dismissed {
			got += "*"
		}
		got += " "
	}
	// Recorrentes primeiro (pelo valor), a avulsa por último, mesmo valendo mais; a dispensa vale por tipo e conta.
	if got != "seguro streaming* curso " {
		t.Errorf("ordem e dispensa: %q", got)
	}
}
