package usecase

import (
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func month(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

// monthsOf cria uma linha por mês, a partir de abril, com os totais dados.
func monthsOf(key string, totals ...float64) []ports.ExpenseKeyMonth {
	return monthsFrom(key, time.April, totals...)
}

func monthsFrom(key string, start time.Month, totals ...float64) []ports.ExpenseKeyMonth {
	var rows []ports.ExpenseKeyMonth
	for i, t := range totals {
		rows = append(rows, ports.ExpenseKeyMonth{Key: key, Label: key, Month: month(2026, start+time.Month(i)), Total: t, Count: 1, Day: 10, AllPaid: true})
	}
	return rows
}

func classOf(t *testing.T, b Budget, key string) BudgetItem {
	t.Helper()
	for _, it := range b.Items {
		if it.Key == key {
			return it
		}
	}
	t.Fatalf("conta %q não está no último mês: %+v", key, b.Items)
	return BudgetItem{}
}

func TestBuildBudget_ClassifiesByRule(t *testing.T) {
	var rows []ports.ExpenseKeyMonth
	rows = append(rows, monthsOf("netflix", 44.9, 44.9, 44.9, 44.9, 44.9, 44.9)...) // fixa: todo mês, mesmo valor
	rows = append(rows, monthsOf("luz", 180, 210, 195, 205, 190, 200)...)           // fixa: valor varia dentro de 30%
	rows = append(rows, monthsOf("mercado", 400, 900, 150, 700, 300, 800)...)       // valor muito diferente: variável
	rows = append(rows,
		ports.ExpenseKeyMonth{Key: "academia", Label: "academia", Month: month(2026, time.August), Total: 100, Count: 1, Day: 5},
		ports.ExpenseKeyMonth{Key: "academia", Label: "academia", Month: month(2026, time.September), Total: 100, Count: 1, Day: 5}) // só 2 meses: ainda não dá para dizer
	rows = append(rows, monthsOf("seguro", 12, 12, 12, 12, 12, 12)...)
	for i := range rows {
		if rows[i].Key == "seguro" {
			rows[i].Installment = true // "Parc 001/012"
		}
	}
	// 5 lançamentos no mesmo mês, todo mês, com valor igual: hábito (ex.: pix de lanche), não conta fixa
	for _, r := range monthsOf("lanche", 50, 50, 50, 50, 50, 50) {
		r.Count = 5
		rows = append(rows, r)
	}

	b := BuildBudget(rows, nil, month(2026, time.April), month(2026, time.September))
	want := map[string]ports.ExpenseClass{
		"netflix": ports.ClassFixed, "luz": ports.ClassFixed, "mercado": ports.ClassVariable,
		"academia": ports.ClassVariable, "seguro": ports.ClassInstallment, "lanche": ports.ClassVariable,
	}
	for key, class := range want {
		if got := classOf(t, b, key); got.Class != class || got.Manual {
			t.Errorf("%s: got %s manual=%v, esperado %s", key, got.Class, got.Manual, class)
		}
	}
	if got := classOf(t, b, "netflix"); got.Months != 6 || got.Total != 44.9 || got.Day != 10 || !got.Paid {
		t.Errorf("detalhes da conta: %+v", got)
	}
}

func TestBuildBudget_FewMonthsOfHistory(t *testing.T) {
	// Só 2 meses de dados: uma conta que aparece nos dois com valor parecido já é candidata a fixa.
	rows := append(monthsOf("wellhub", 100, 100), monthsOf("mercado", 400, 900)...)
	b := BuildBudget(rows, nil, month(2026, time.April), month(2026, time.May))
	if classOf(t, b, "wellhub").Class != ports.ClassFixed || classOf(t, b, "mercado").Class != ports.ClassVariable {
		t.Errorf("com 2 meses: %+v", b.Items)
	}

	// Com 2 meses, valor que varia (mesmo pouco) não basta: 5% no máximo.
	rows = append(monthsOf("wellhub", 100, 100), monthsOf("birigui", 296, 228)...)
	rows = append(rows, monthsOf("netflix", 44.9, 45.9)...)
	b = BuildBudget(rows, nil, month(2026, time.April), month(2026, time.May))
	if classOf(t, b, "birigui").Class != ports.ClassVariable || classOf(t, b, "netflix").Class != ports.ClassFixed {
		t.Errorf("2 meses exigem valor quase igual: %+v", b.Items)
	}

	// Um mês só: não há como saber o que se repete.
	one := BuildBudget(monthsOf("wellhub", 100), nil, month(2026, time.April), month(2026, time.April))
	if classOf(t, one, "wellhub").Class != ports.ClassVariable {
		t.Errorf("com 1 mês tudo é variável: %+v", one.Items)
	}

	// Com 3 meses de dados, aparecer em 2 não basta.
	rows = append(monthsFrom("wellhub", time.May, 100, 100), monthsOf("mercado", 10, 20, 30)...)
	if classOf(t, BuildBudget(rows, nil, month(2026, time.April), month(2026, time.June)), "wellhub").Class != ports.ClassVariable {
		t.Error("com 3 meses de dados, 2 aparições não é conta fixa")
	}
}

func TestBuildBudget_ManualRuleBeatsDetection(t *testing.T) {
	rows := append(monthsOf("netflix", 45, 45, 45, 45), monthsOf("mercado", 400, 900, 150, 700)...)
	rules := map[string]ports.ExpenseClass{"netflix": ports.ClassVariable, "mercado": ports.ClassFixed}

	b := BuildBudget(rows, rules, month(2026, time.April), month(2026, time.July))
	if n := classOf(t, b, "netflix"); n.Class != ports.ClassVariable || !n.Manual {
		t.Errorf("manual variável: %+v", n)
	}
	if m := classOf(t, b, "mercado"); m.Class != ports.ClassFixed || !m.Manual {
		t.Errorf("manual fixa: %+v", m)
	}
}

func TestBuildBudget_RecurringIsFixedAndParcelBeatsDetection(t *testing.T) {
	rows := monthsOf("aluguel", 1500)
	rows[0].Recurring = true // cadastrada como recorrente: fixa desde o primeiro mês
	b := BuildBudget(rows, nil, month(2026, time.April), month(2026, time.April))
	if got := classOf(t, b, "aluguel"); got.Class != ports.ClassFixed {
		t.Errorf("recorrente: %+v", got)
	}

	p := monthsOf("tv", 300, 300, 300, 300)
	for i := range p {
		p[i].Installment = true
	}
	if got := classOf(t, BuildBudget(p, nil, month(2026, time.April), month(2026, time.July)), "tv"); got.Class != ports.ClassInstallment {
		t.Errorf("parcelada vence a detecção de fixa: %+v", got)
	}
}

func TestBuildBudget_SeriesSumsPerMonthAndFillsGaps(t *testing.T) {
	rows := append(monthsOf("netflix", 45, 45, 45), monthsOf("mercado", 100, 200, 300)...)
	rows = append(rows, ports.ExpenseKeyMonth{Key: "tv", Month: month(2026, time.May), Total: 70, Count: 1, Installment: true})

	b := BuildBudget(rows, nil, month(2026, time.March), month(2026, time.June))
	if len(b.Series) != 4 {
		t.Fatalf("esperava 4 meses (março sem dados incluído): %+v", b.Series)
	}
	want := []BudgetMonth{
		{Month: month(2026, time.March)},
		{Month: month(2026, time.April), Fixed: 45, Variable: 100},
		{Month: month(2026, time.May), Fixed: 45, Installment: 70, Variable: 200},
		{Month: month(2026, time.June), Fixed: 45, Variable: 300},
	}
	for i, w := range want {
		if b.Series[i] != w {
			t.Errorf("mês %d: got %+v, esperado %+v", i, b.Series[i], w)
		}
	}
	if len(b.Items) != 2 || b.Items[0].Key != "mercado" {
		t.Errorf("itens do último mês, do maior para o menor: %+v", b.Items)
	}
}

func TestBuildBudget_Empty(t *testing.T) {
	b := BuildBudget(nil, nil, month(2026, time.January), month(2026, time.March))
	if len(b.Series) != 3 || len(b.Items) != 0 {
		t.Errorf("sem dados: %+v", b)
	}
}
