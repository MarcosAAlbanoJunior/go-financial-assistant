package usecase

import (
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// averageMonths é quantos meses (com dados) entram nas médias de renda, fixas e variáveis.
const averageMonths = 3

// Assumptions são as premissas da projeção: o que se espera de um mês comum daqui para a frente.
type Assumptions struct {
	Income   float64
	Fixed    float64
	Variable float64
	BasedOn  int // quantos meses com dados sustentam as médias (0 = sem histórico)
}

type ProjectedMonth struct {
	Month       time.Time
	Fixed       float64
	Installment float64 // parcelas já conhecidas
	Variable    float64
}

type Projection struct {
	Assumptions Assumptions
	Months      []ProjectedMonth
}

var installmentRE = regexp.MustCompile(`(?i)(?:\(\s*(\d+)\s*/\s*(\d+)\s*\)|parc[a-z.]*\s*(\d+)\s*/\s*(\d+))`)

// parseInstallment lê "n/m" de "(2/12)" ou "Parc 011/012": a parcela atual e o total.
func parseInstallment(label string) (current, total int, ok bool) {
	m := installmentRE.FindStringSubmatch(label)
	if m == nil {
		return 0, 0, false
	}
	a, b := m[1], m[2]
	if a == "" {
		a, b = m[3], m[4]
	}
	current, _ = strconv.Atoi(a)
	total, _ = strconv.Atoi(b)
	return current, total, total > 1 && current >= 1 && current <= total
}

// BuildProjection projeta `months` meses a partir de start (primeiro dia do mês, em geral o mês atual).
//
//   - Renda, fixas e variáveis valem a média dos últimos meses completos com dados (anteriores a start);
//   - parcelas são as já conhecidas: as que faltam das compras parceladas no cartão (inferidas do "n/m" da
//     descrição, já que o banco só informa a parcela do mês) e as cadastradas, em knownInstallments.
//
// rows são as despesas por conta dos 12 meses completos antes de start; incomeByMonth, a renda de cada mês.
func BuildProjection(rows []ports.ExpenseKeyMonth, rules map[string]ports.ExpenseClass, incomeByMonth map[time.Time]float64,
	knownInstallments map[time.Time]float64, start time.Time, months int) Projection {

	b := BuildBudget(rows, rules, start.AddDate(0, -12, 0), start.AddDate(0, -1, 0))

	// Médias dos últimos meses que tiveram alguma despesa.
	var a Assumptions
	for i := len(b.Series) - 1; i >= 0 && a.BasedOn < averageMonths; i-- {
		m := b.Series[i]
		if m.Fixed+m.Installment+m.Variable <= 0 {
			continue
		}
		a.BasedOn++
		a.Fixed += m.Fixed
		a.Variable += m.Variable
		a.Income += incomeByMonth[m.Month]
	}
	if a.BasedOn > 0 {
		n := float64(a.BasedOn)
		a.Fixed, a.Variable, a.Income = a.Fixed/n, a.Variable/n, a.Income/n
	}

	// Parcelas que faltam: a última aparição de cada conta parcelada diz em que parcela está.
	remaining := map[time.Time]float64{}
	latest := map[string]ports.ExpenseKeyMonth{}
	for _, r := range rows {
		if b.Classes[r.Key] == ports.ClassInstallment && !r.Month.Before(latest[r.Key].Month) {
			latest[r.Key] = r
		}
	}
	for _, r := range latest {
		cur, total, ok := parseInstallment(r.Label)
		if !ok {
			continue
		}
		for k := 1; k <= total-cur; k++ {
			remaining[r.Month.AddDate(0, k, 0)] += r.Total
		}
	}

	p := Projection{Assumptions: a}
	for i := 0; i < months; i++ {
		m := start.AddDate(0, i, 0)
		p.Months = append(p.Months, ProjectedMonth{Month: m, Fixed: a.Fixed, Variable: a.Variable, Installment: remaining[m] + knownInstallments[m]})
	}
	sort.Slice(p.Months, func(i, j int) bool { return p.Months[i].Month.Before(p.Months[j].Month) })
	return p
}
