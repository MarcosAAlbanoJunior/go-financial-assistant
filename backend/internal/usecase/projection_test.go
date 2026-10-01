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
	income := map[time.Time]float64{month(2026, time.August): 5000, month(2026, time.September): 7000}
	known := map[time.Time]float64{month(2026, time.December): 300} // parcela cadastrada à mão

	p := BuildProjection(rows, nil, income, known, start, 4)

	a := p.Assumptions
	if a.BasedOn != 2 || a.Income != 6000 || a.Fixed != 100 || a.Variable != 500 {
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
