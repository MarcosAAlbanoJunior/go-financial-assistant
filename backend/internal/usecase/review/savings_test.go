package review

import (
	"math"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func charge(key string, m time.Month, total float64) domain.ExpenseKeyMonth {
	return domain.ExpenseKeyMonth{Key: key, Month: month(2026, m), Total: total}
}

func TestBuildSavings(t *testing.T) {
	now := month(2026, time.October)
	dec := func(key string, m time.Month, monthly float64) domain.Decision {
		return domain.Decision{Kind: ReviewFixed, Key: key, Month: month(2026, m), Monthly: monthly}
	}
	decisions := []domain.Decision{
		dec("ok", time.June, 40),           // jul, ago, set sem cobrança: 3 meses confirmados
		dec("voltou", time.June, 40),       // voltou em ago
		dec("recente", time.September, 40), // decidiu em set; nenhum mês fechado depois (out está aberto)
		dec("voltou-aberto", time.August, 40),
		dec("parcial", time.August, 100),
	}
	rows := []domain.ExpenseKeyMonth{
		charge("ok", time.June, 40), // o mês da decisão não conta
		charge("voltou", time.August, 40),
		charge("voltou-aberto", time.October, 39.9), // voltou no mês em andamento
		charge("parcial", time.September, 30),       // abaixo de metade do custo: tarifa residual, não é a volta
		charge("recente", time.September, 40),
	}
	got := map[string]DecisionResult{}
	for _, r := range BuildSavings(decisions, rows, now) {
		got[r.Decision.Key] = r
	}

	if r := got["ok"]; r.Status != SavingConfirmed || r.MonthsConfirmed != 3 || r.Realized != 120 || r.Returned != 0 {
		t.Errorf("ok: %+v", r)
	}
	if r := got["voltou"]; r.Status != SavingReturned || r.Returned != 40 || r.MonthsConfirmed != 2 || r.Realized != 80 {
		t.Errorf("voltou em ago (jul e set sem cobrança): %+v", r)
	}
	if r := got["recente"]; r.Status != SavingPending || r.MonthsConfirmed != 0 || r.Realized != 0 {
		t.Errorf("recente: %+v", r)
	}
	if r := got["voltou-aberto"]; r.Status != SavingReturned || r.Returned != 39.9 || r.MonthsConfirmed != 1 {
		t.Errorf("volta no mês em andamento é flagrada: %+v", r)
	}
	if r := got["parcial"]; r.Status != SavingConfirmed || r.MonthsConfirmed != 1 {
		t.Errorf("cobrança residual abaixo de metade ainda é economia: %+v", r)
	}

	realized, perMonth := SavingsTotals(BuildSavings(decisions, rows, now))
	// ok 120 + voltou 80 + voltou-aberto 40 + parcial 100 = 340; ritmo = confirmadas (ok 40 + parcial 100).
	if math.Abs(realized-340) > 0.01 || perMonth != 140 {
		t.Errorf("totais: %v %v", realized, perMonth)
	}
}
