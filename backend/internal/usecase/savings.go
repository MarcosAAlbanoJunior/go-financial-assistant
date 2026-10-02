package usecase

import (
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

// Situação de uma decisão "cancelei".
const (
	SavingPending   = "PENDING"   // ainda não fechou nenhum mês depois da decisão
	SavingConfirmed = "CONFIRMED" // a cobrança sumiu nos meses seguintes
	SavingReturned  = "RETURNED"  // a cobrança voltou
)

// stillCharged: a conta conta como cobrada de novo se o mês trouxe ao menos metade do que ela custava.
const stillCharged = 0.5

// DecisionResult confere uma decisão contra o que foi cobrado depois dela.
type DecisionResult struct {
	Decision        domain.Decision
	Status          string
	MonthsConfirmed int     // meses fechados depois da decisão sem a cobrança
	Realized        float64 // economia acumulada: custo mensal x MonthsConfirmed
	Returned        float64 // quanto foi cobrado quando voltou (0 se não voltou)
}

// BuildSavings confere cada decisão mês a mês, do mês seguinte ao da decisão até hoje. now é o primeiro dia
// do mês atual: o mês em andamento só serve para flagrar a volta da cobrança, não conta como mês confirmado.
func BuildSavings(decisions []domain.Decision, rows []domain.ExpenseKeyMonth, now time.Time) []DecisionResult {
	charged := map[string]map[time.Time]float64{}
	for _, r := range rows {
		if charged[r.Key] == nil {
			charged[r.Key] = map[time.Time]float64{}
		}
		charged[r.Key][r.Month] += r.Total
	}

	out := make([]DecisionResult, 0, len(decisions))
	for _, d := range decisions {
		res := DecisionResult{Decision: d}
		for m := d.Month.AddDate(0, 1, 0); !m.After(now); m = m.AddDate(0, 1, 0) {
			total := charged[d.Key][m]
			switch {
			case total >= d.Monthly*stillCharged:
				res.Returned = total
			case m.Before(now):
				res.MonthsConfirmed++
			}
		}
		res.Realized = d.Monthly * float64(res.MonthsConfirmed)
		switch {
		case res.Returned > 0:
			res.Status = SavingReturned
		case res.MonthsConfirmed > 0:
			res.Status = SavingConfirmed
		default:
			res.Status = SavingPending
		}
		out = append(out, res)
	}
	return out
}

// SavingsTotals soma a economia já realizada e o ritmo mensal das decisões confirmadas.
func SavingsTotals(results []DecisionResult) (realized, perMonth float64) {
	for _, r := range results {
		realized += r.Realized
		if r.Status == SavingConfirmed {
			perMonth += r.Decision.Monthly
		}
	}
	return realized, perMonth
}
