package usecase

import (
	"sort"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const (
	// Uma conta só é "fixa" por detecção se aparecer em pelo menos 3 meses da janela (ou em todos os
	// meses com dados, quando há só 2) ...
	maxFixedMonthsNeeded = 3
	minFixedMonthsNeeded = 2
	// ...no máximo 2 vezes por mês (mais que isso é hábito de consumo, não conta)...
	maxFixedPerMonth = 2
	// ...e com valor parecido: (maior - menor) / mediana dos totais mensais até 30%. Com só 2 meses
	// de dados, um acaso é muito mais provável, então o valor precisa ser praticamente o mesmo.
	maxFixedSpread    = 0.30
	maxFixedSpreadTwo = 0.05
)

// BudgetItem é uma conta (descrição normalizada) no mês escolhido.
type BudgetItem struct {
	Key      string
	Label    string
	Category string
	Class    ports.ExpenseClass
	Manual   bool // a classe veio de uma correção manual, não da detecção
	Total    float64
	Count    int
	Day      int
	Paid     bool
	Months   int // em quantos meses da janela a conta aparece
}

// BudgetMonth soma as despesas do mês por classe.
type BudgetMonth struct {
	Month       time.Time
	Fixed       float64
	Installment float64
	Variable    float64
}

type Budget struct {
	Series []BudgetMonth
	Items  []BudgetItem // contas do último mês da janela, da maior para a menor
}

// BuildBudget classifica cada conta como fixa, parcelada ou variável e soma por mês.
// Ordem de decisão: correção manual, parcelada, recorrente cadastrada, detecção de conta fixa e,
// por fim, variável. A classe vale para a conta inteira na janela (from a to, primeiros dias dos meses).
func BuildBudget(rows []ports.ExpenseKeyMonth, rules map[string]ports.ExpenseClass, from, to time.Time) Budget {
	byKey := map[string][]ports.ExpenseKeyMonth{}
	for _, r := range rows {
		byKey[r.Key] = append(byKey[r.Key], r)
	}

	type verdict struct {
		class  ports.ExpenseClass
		manual bool
		months int
	}
	// Com pouco histórico (conta nova no Open Finance) pede-se menos meses; com um mês só não há como saber.
	dataMonths := map[time.Time]bool{}
	for _, r := range rows {
		dataMonths[r.Month] = true
	}
	needed := min(maxFixedMonthsNeeded, len(dataMonths))

	verdicts := make(map[string]verdict, len(byKey))
	for key, list := range byKey {
		v := verdict{months: len(list)}
		switch rule := rules[key]; {
		case rule == ports.ClassFixed || rule == ports.ClassVariable:
			v.class, v.manual = rule, true
		case anyRow(list, func(r ports.ExpenseKeyMonth) bool { return r.Installment }):
			v.class = ports.ClassInstallment
		case anyRow(list, func(r ports.ExpenseKeyMonth) bool { return r.Recurring }), looksFixed(list, needed):
			v.class = ports.ClassFixed
		default:
			v.class = ports.ClassVariable
		}
		verdicts[key] = v
	}

	var b Budget
	index := map[time.Time]int{}
	for m := from; !m.After(to); m = m.AddDate(0, 1, 0) {
		index[m] = len(b.Series)
		b.Series = append(b.Series, BudgetMonth{Month: m})
	}
	for _, r := range rows {
		i, ok := index[r.Month]
		if !ok {
			continue
		}
		switch verdicts[r.Key].class {
		case ports.ClassFixed:
			b.Series[i].Fixed += r.Total
		case ports.ClassInstallment:
			b.Series[i].Installment += r.Total
		default:
			b.Series[i].Variable += r.Total
		}
		if r.Month.Equal(to) {
			v := verdicts[r.Key]
			b.Items = append(b.Items, BudgetItem{
				Key: r.Key, Label: r.Label, Category: r.Category, Class: v.class, Manual: v.manual,
				Total: r.Total, Count: r.Count, Day: r.Day, Paid: r.AllPaid, Months: v.months,
			})
		}
	}
	sort.SliceStable(b.Items, func(i, j int) bool { return b.Items[i].Total > b.Items[j].Total })
	return b
}

func anyRow(list []ports.ExpenseKeyMonth, f func(ports.ExpenseKeyMonth) bool) bool {
	for _, r := range list {
		if f(r) {
			return true
		}
	}
	return false
}

// looksFixed detecta a conta que se repete todo mês com valor parecido (assinatura, seguro, mensalidade).
func looksFixed(list []ports.ExpenseKeyMonth, needed int) bool {
	if needed < minFixedMonthsNeeded || len(list) < needed {
		return false
	}
	totals := make([]float64, len(list))
	for i, r := range list {
		if r.Count > maxFixedPerMonth {
			return false
		}
		totals[i] = r.Total
	}
	sort.Float64s(totals)
	median := totals[len(totals)/2]
	if len(totals)%2 == 0 {
		median = (totals[len(totals)/2-1] + totals[len(totals)/2]) / 2
	}
	limit := maxFixedSpread
	if needed == minFixedMonthsNeeded {
		limit = maxFixedSpreadTwo
	}
	return median > 0 && (totals[len(totals)-1]-totals[0])/median <= limit
}
