package usecase

import (
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

// cutBaselineMonths é quantos meses anteriores (com gasto na categoria) formam a média de uma meta de redução.
const cutBaselineMonths = 3

// maxCutHistory limita os meses mostrados no acompanhamento de uma meta de redução.
const maxCutHistory = 12

// CutMonth é o gasto da categoria em um mês e se ficou dentro do teto da meta.
type CutMonth struct {
	Month time.Time
	Total float64
	Hit   bool
}

// GoalProgress é o andamento de uma meta, calculado na hora (nada disso é gravado).
type GoalProgress struct {
	Goal    ports.Goal
	Current float64 // SAVE e RESERVE: patrimônio; CUT: gasto da categoria no mês atual
	Target  float64 // SAVE: valor; RESERVE: meses x fixas; CUT: teto mensal
	Done    bool

	// SAVE
	MonthsLeft int      // meses até a data (0 = vence neste mês ou já venceu)
	PerMonth   float64  // quanto guardar por mês para chegar lá
	Surplus    *float64 // sobra mensal média projetada até a data; nil sem histórico para projetar
	Fits       *bool    // PerMonth cabe na sobra; nil quando Surplus é desconhecida

	// RESERVE
	Coverage float64 // quantos meses de despesas fixas o patrimônio cobre

	// CUT
	History []CutMonth // do mês da criação até o atual
}

// CutBaseline é a média mensal da categoria nos até 3 meses anteriores a now (primeiro dia do mês) em que
// ela teve gasto; false quando não há nenhum.
func CutBaseline(cats []ports.CategoryMonth, category string, now time.Time) (float64, bool) {
	var total float64
	var n int
	for _, c := range cats {
		if c.Category == category && c.Total > 0 && c.Month.Before(now) && !c.Month.Before(now.AddDate(0, -cutBaselineMonths, 0)) {
			total += c.Total
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return total / float64(n), true
}

// BuildGoalProgress mede uma meta contra o patrimônio (saldo das contas correntes + investimentos), a
// projeção e os gastos por categoria. now é o primeiro dia do mês atual.
func BuildGoalProgress(g ports.Goal, wealth float64, p Projection, cats []ports.CategoryMonth, now time.Time) GoalProgress {
	gp := GoalProgress{Goal: g}
	switch g.Kind {
	case ports.GoalSave:
		gp.Current, gp.Target = wealth, g.TargetAmount
		gp.Done = wealth >= g.TargetAmount
		gp.MonthsLeft = max(0, (g.TargetDate.Year()-now.Year())*12+int(g.TargetDate.Month()-now.Month()))
		if !gp.Done {
			gp.PerMonth = (g.TargetAmount - wealth) / float64(max(gp.MonthsLeft, 1))
			gp.Surplus = projectedSurplus(p, now, max(gp.MonthsLeft, 1))
			if gp.Surplus != nil {
				fits := gp.PerMonth <= *gp.Surplus
				gp.Fits = &fits
			}
		}
	case ports.GoalReserve:
		gp.Current, gp.Target = wealth, float64(g.ReserveMonths)*p.Assumptions.Fixed
		if p.Assumptions.Fixed > 0 {
			gp.Coverage = wealth / p.Assumptions.Fixed
			gp.Done = wealth >= gp.Target
		}
	case ports.GoalCut:
		gp.Target = g.Baseline * (1 - float64(g.CutPercent)/100)
		totals := map[time.Time]float64{}
		for _, c := range cats {
			if c.Category == g.Category {
				totals[c.Month] += c.Total
			}
		}
		created := time.Date(g.CreatedAt.Year(), g.CreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		start := max2(created, now.AddDate(0, -(maxCutHistory-1), 0))
		for m := start; !m.After(now); m = m.AddDate(0, 1, 0) {
			gp.History = append(gp.History, CutMonth{Month: m, Total: totals[m], Hit: totals[m] <= gp.Target})
		}
		gp.Current = totals[now]
	}
	return gp
}

func max2(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// projectedSurplus é a sobra média por mês (renda - fixas - variáveis - parcelas) nos próximos `months`
// meses a partir de now. Além do alcance da projeção, vale só o mês comum (sem parcelas conhecidas).
func projectedSurplus(p Projection, now time.Time, months int) *float64 {
	a := p.Assumptions
	if a.BasedOn == 0 {
		return nil
	}
	var installments float64
	for _, m := range p.Months {
		if !m.Month.Before(now) && m.Month.Before(now.AddDate(0, months, 0)) {
			installments += m.Installment
		}
	}
	s := a.Income - a.Fixed - a.Variable - installments/float64(months)
	return &s
}
