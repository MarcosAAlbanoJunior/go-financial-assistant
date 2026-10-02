package usecase

import (
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestParseInstallment(t *testing.T) {
	cases := []struct {
		label      string
		cur, total int
		ok         bool
	}{
		{"ANUIDADE DIFERENCIADA (1/12)", 1, 12, true},
		{"MENSALIDADE DE SEGURO Parc 011/012 RISCOS", 11, 12, true},
		{"Loja ( 3 / 6 )", 3, 6, true},
		{"Compra (1/1)", 0, 0, false}, // parcela única não é parcelamento
		{"Compra (7/6)", 0, 0, false}, // inconsistente
		{"Pix 12/09", 0, 0, false},    // data, não parcela
		{"Netflix", 0, 0, false},
	}
	for _, tc := range cases {
		cur, total, ok := parseInstallment(tc.label)
		if ok != tc.ok || (ok && (cur != tc.cur || total != tc.total)) {
			t.Errorf("%q: got %d/%d ok=%v, esperado %d/%d ok=%v", tc.label, cur, total, ok, tc.cur, tc.total, tc.ok)
		}
	}
}

func TestBuildProjection_AveragesAndKnownInstallments(t *testing.T) {
	start := month(2026, time.October)
	var rows []ports.ExpenseKeyMonth
	// Fixa (todo mês, 100) e variável (varia) em ago e set; nenhuma despesa antes disso.
	for _, m := range []time.Month{time.August, time.September} {
		rows = append(rows,
			ports.ExpenseKeyMonth{Key: "netflix", Label: "Netflix", Month: month(2026, m), Total: 100, Count: 1},
			ports.ExpenseKeyMonth{Key: "mercado", Label: "Mercado", Month: month(2026, m), Total: float64(400 + 200*int(m-time.August)), Count: 9})
	}
	// Parcelada: em set está na 10/12, então faltam 2 parcelas (out e nov) de 50.
	rows = append(rows,
		ports.ExpenseKeyMonth{Key: "tv", Label: "TV (9/12)", Month: month(2026, time.August), Total: 50, Count: 1, Installment: true},
		ports.ExpenseKeyMonth{Key: "tv", Label: "TV (10/12)", Month: month(2026, time.September), Total: 50, Count: 1, Installment: true})
	income := []ports.IncomePayment{
		{Key: "salario", Label: "Salário", Month: month(2026, time.August), Amount: 5000},
		{Key: "salario", Label: "Salário", Month: month(2026, time.September), Amount: 7000},
	}
	known := map[time.Time]float64{month(2026, time.December): 300} // parcela cadastrada à mão

	p := BuildProjection(rows, nil, income, known, start, 4)

	a := p.Assumptions
	if a.BasedOn != 2 || a.Income != 6000 || len(a.IncomeSources) != 1 || a.Fixed != 100 || a.Variable != 500 {
		t.Errorf("premissas (média dos 2 meses com dados): %+v", a)
	}
	if len(p.Months) != 4 || !p.Months[0].Month.Equal(start) {
		t.Fatalf("4 meses a partir de out: %+v", p.Months)
	}
	wantInstallment := []float64{50, 50, 300, 0} // out e nov: TV; dez: cadastrada; jan: nada
	for i, w := range wantInstallment {
		if p.Months[i].Installment != w || p.Months[i].Fixed != 100 || p.Months[i].Variable != 500 {
			t.Errorf("mês %d: %+v (parcelas esperadas %v)", i, p.Months[i], w)
		}
	}
}

func TestBuildProjection_NoHistory(t *testing.T) {
	p := BuildProjection(nil, nil, nil, nil, month(2026, time.October), 3)
	if p.Assumptions.BasedOn != 0 || len(p.Months) != 3 || p.Months[0].Fixed != 0 {
		t.Errorf("sem histórico tudo é zero: %+v", p)
	}
}

func TestBuildProjection_FinishedInstallmentsAndGaps(t *testing.T) {
	start := month(2026, time.October)
	rows := []ports.ExpenseKeyMonth{
		{Key: "seguro", Label: "SEGURO Parc 012/012", Month: month(2026, time.September), Total: 12, Count: 1, Installment: true}, // última
		{Key: "geladeira", Label: "Geladeira (2/6)", Month: month(2026, time.July), Total: 200, Count: 1, Installment: true},      // sumiu em ago e set
	}
	p := BuildProjection(rows, nil, nil, nil, start, 6)
	// Seguro acabou. Geladeira estava na 2/6 em jul: restam 4 parcelas a partir de ago (ago, set, out, nov)
	// e só out e nov caem na janela da projeção.
	got := []float64{p.Months[0].Installment, p.Months[1].Installment, p.Months[2].Installment}
	if got[0] != 200 || got[1] != 200 || got[2] != 0 {
		t.Errorf("parcelas restantes: %v", got)
	}
}

func inc(key string, m time.Month, amount float64) ports.IncomePayment {
	return ports.IncomePayment{Key: key, Label: key, Month: month(2026, m), Amount: amount}
}

func TestEstimateIncome_OneOffPaymentDoesNotInflateTheSalary(t *testing.T) {
	months := []time.Time{month(2026, time.August), month(2026, time.September)}
	payments := []ports.IncomePayment{
		inc("salário empresa", time.August, 6362),
		inc("entrada empresa", time.August, 1700),
		inc("salário empresa", time.September, 6372),
		inc("entrada empresa", time.September, 1700),
		inc("salário empresa", time.September, 9352),   // adiantamento de férias: mesma descrição do salário
		inc("pix recebido fulano", time.September, 72), // só em um mês: eventual
		inc("pix recebido fulano", time.September, 100),
	}
	got := EstimateIncome(payments, months)

	var total float64
	for _, s := range got {
		total += s.Monthly
	}
	// salário: mediana dos pagamentos (6372) x 1 por mês (mediana inferior de 1 e 2); entrada: 1700.
	if len(got) != 2 || got[0].Label != "salário empresa" || got[0].Monthly != 6372 || got[1].Monthly != 1700 || total != 8072 {
		t.Errorf("renda estimada: %+v (total %v)", got, total)
	}
}

func TestEstimateIncome_NeededMonthsAdaptToHistory(t *testing.T) {
	three := []time.Time{month(2026, time.July), month(2026, time.August), month(2026, time.September)}
	payments := []ports.IncomePayment{
		inc("salário", time.July, 5000), inc("salário", time.August, 5000), inc("salário", time.September, 5000),
		inc("bônus", time.August, 3000), inc("bônus", time.September, 3000), // 2 de 3 meses: eventual
	}
	if got := EstimateIncome(payments, three); len(got) != 1 || got[0].Monthly != 5000 {
		t.Errorf("com 3 meses de dados, 2 aparições não é recorrente: %+v", got)
	}

	// Com um mês só não há como distinguir: tudo entra.
	one := []time.Time{month(2026, time.September)}
	if got := EstimateIncome([]ports.IncomePayment{inc("salário", time.September, 5000), inc("bônus", time.September, 800)}, one); len(got) != 2 {
		t.Errorf("com 1 mês tudo entra: %+v", got)
	}

	// Fora da janela não conta; sem pagamentos, nada.
	if got := EstimateIncome([]ports.IncomePayment{inc("salário", time.January, 9999)}, three); len(got) != 0 {
		t.Errorf("fora da janela: %+v", got)
	}
	if got := EstimateIncome(nil, three); len(got) != 0 {
		t.Errorf("sem pagamentos: %+v", got)
	}
}

func TestEstimateIncome_MedianAndCounts(t *testing.T) {
	months := []time.Time{month(2026, time.July), month(2026, time.August), month(2026, time.September)}
	// Quinzena: dois pagamentos por mês, um deles com valor atípico no último mês.
	payments := []ports.IncomePayment{
		inc("quinzena", time.July, 2000), inc("quinzena", time.July, 2000),
		inc("quinzena", time.August, 2000), inc("quinzena", time.August, 2000),
		inc("quinzena", time.September, 2000), inc("quinzena", time.September, 7000),
	}
	if got := EstimateIncome(payments, months); len(got) != 1 || got[0].Monthly != 4000 {
		t.Errorf("mediana 2000 x 2 por mês: %+v", got)
	}
}

func TestMedianMonthlyIncome(t *testing.T) {
	months := []time.Time{month(2026, time.April), month(2026, time.May), month(2026, time.June), month(2026, time.July)}
	payments := []ports.IncomePayment{
		inc("a", time.April, 3000), inc("b", time.April, 5000), // abril: 8000
		inc("c", time.May, 8100),                            // maio: 8100
		inc("d", time.June, 7900), inc("e", time.June, 100), // junho: 8000
		inc("f", time.July, 18000), // julho atípico (13º, venda): não pesa
	}
	if got := MedianMonthlyIncome(payments, months); got != 8050 {
		t.Errorf("mediana de 8000, 8100, 8000 e 18000 = 8050, got %v", got)
	}
	// Mês com despesa e nenhuma entrada conta como zero.
	if got := MedianMonthlyIncome([]ports.IncomePayment{inc("a", time.April, 100)}, months[:3]); got != 0 {
		t.Errorf("2 de 3 meses sem renda: mediana 0, got %v", got)
	}
	if MedianMonthlyIncome(payments, nil) != 0 {
		t.Error("sem meses não há renda")
	}
}

// Caso real que a estimativa só por fonte errava: a renda vem de origens variadas (cada mês de um pagador
// diferente), só uma fonte se repete, e o mês atípico não pode puxar a média para cima.
func TestBuildProjection_VariedIncomeSourcesAreNotLost(t *testing.T) {
	start := month(2026, time.October)
	var rows []ports.ExpenseKeyMonth
	var income []ports.IncomePayment
	amounts := map[time.Month]float64{time.April: 5000, time.May: 5100, time.June: 5000, time.July: 6500, time.August: 5100, time.September: 12000}
	for m := time.April; m <= time.September; m++ {
		rows = append(rows, ports.ExpenseKeyMonth{Key: "mercado", Label: "Mercado", Month: month(2026, m), Total: 6000, Count: 9})
		income = append(income,
			ports.IncomePayment{Key: "aluguel recebido", Label: "Aluguel recebido", Month: month(2026, m), Amount: 1000},                    // única fonte fixa
			ports.IncomePayment{Key: "pix recebido " + m.String(), Label: "Pix recebido", Month: month(2026, m), Amount: amounts[m] - 1000}) // origem diferente a cada mês
	}
	a := BuildProjection(rows, nil, income, nil, start, 3).Assumptions

	// Mediana de 5000, 5100, 5000, 6500, 5100 e 12000 = 5100; as fontes recorrentes explicam só 1000.
	if a.Income != 5100 {
		t.Fatalf("renda = mediana da renda total dos 6 meses (5100), não só a fonte recorrente (1000): %v", a.Income)
	}
	if len(a.IncomeSources) != 2 || a.IncomeSources[0].Monthly != 1000 || a.IncomeSources[1].Label != otherIncomeLabel || a.IncomeSources[1].Monthly != 4100 {
		t.Errorf("fontes: recorrente 1000 + outras entradas 4100: %+v", a.IncomeSources)
	}
	if a.Variable != 6000 || a.BasedOn != 3 {
		t.Errorf("despesas seguem a média dos 3 últimos meses: %+v", a)
	}
}

// Quando as fontes recorrentes já passam da mediana (mês fraco recente), elas são o piso.
func TestBuildProjection_RecurringSourcesAreTheFloor(t *testing.T) {
	start := month(2026, time.October)
	var rows []ports.ExpenseKeyMonth
	var income []ports.IncomePayment
	for _, m := range []time.Month{time.April, time.May, time.June, time.July, time.August, time.September} {
		rows = append(rows, ports.ExpenseKeyMonth{Key: "mercado", Label: "Mercado", Month: month(2026, m), Total: 1000, Count: 9})
	}
	for _, m := range []time.Month{time.July, time.August, time.September} { // salário só nos 3 últimos meses
		income = append(income, ports.IncomePayment{Key: "salario", Label: "Salário", Month: month(2026, m), Amount: 5000})
	}
	// Mediana dos 6 meses = (0 + 5000) / 2 = 2500, abaixo dos 5000 recorrentes: vale a fonte recorrente.
	a := BuildProjection(rows, nil, income, nil, start, 3).Assumptions
	if a.Income != 5000 || len(a.IncomeSources) != 1 {
		t.Errorf("piso = fontes recorrentes: %+v", a)
	}
}
