package usecase

import (
	"math"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

var goalNow = month(2026, time.October)

// goalProjection: renda 5000, fixas 2000, variáveis 1500 (sobra 1500) e uma parcela de 300 em nov e dez.
func goalProjection() Projection {
	return Projection{
		Assumptions: Assumptions{Income: 5000, Fixed: 2000, Variable: 1500, BasedOn: 3},
		Months: []ProjectedMonth{
			{Month: goalNow}, {Month: month(2026, time.November), Installment: 300}, {Month: month(2026, time.December), Installment: 300}, {Month: month(2027, time.January)},
		},
	}
}

func TestBuildGoalProgress_Save(t *testing.T) {
	g := ports.Goal{Kind: ports.GoalSave, TargetAmount: 10000, TargetDate: month(2027, time.January)}

	gp := BuildGoalProgress(g, 4000, goalProjection(), nil, goalNow)
	// Faltam 6000 e a data (jan/27) está a 3 meses: 2000 por mês.
	if gp.MonthsLeft != 3 || math.Abs(gp.PerMonth-2000) > 0.01 || gp.Done {
		t.Fatalf("prazo e quanto guardar: %+v", gp)
	}
	// Sobra: 1500 - (300+300)/3 = 1300, menor que 2000: não cabe.
	if gp.Surplus == nil || math.Abs(*gp.Surplus-1300) > 0.01 || gp.Fits == nil || *gp.Fits {
		t.Errorf("não cabe na sobra: surplus=%v fits=%v", gp.Surplus, gp.Fits)
	}

	if gp := BuildGoalProgress(ports.Goal{Kind: ports.GoalSave, TargetAmount: 10000, TargetDate: month(2027, time.January)}, 7000, goalProjection(), nil, goalNow); gp.Fits == nil || !*gp.Fits {
		t.Errorf("faltam 3000 em 3 meses (1000/mês) cabe em 1300: %+v", gp.Fits)
	}
	if gp := BuildGoalProgress(g, 10000, goalProjection(), nil, goalNow); !gp.Done || gp.PerMonth != 0 || gp.Fits != nil {
		t.Errorf("meta batida: %+v", gp)
	}
}

func TestBuildGoalProgress_SaveEdges(t *testing.T) {
	late := ports.Goal{Kind: ports.GoalSave, TargetAmount: 1000, TargetDate: month(2026, time.August)}
	if gp := BuildGoalProgress(late, 400, goalProjection(), nil, goalNow); gp.MonthsLeft != 0 || gp.PerMonth != 600 {
		t.Errorf("data vencida: falta tudo de uma vez: %+v", gp)
	}
	noHistory := goalProjection()
	noHistory.Assumptions.BasedOn = 0
	if gp := BuildGoalProgress(ports.Goal{Kind: ports.GoalSave, TargetAmount: 1000, TargetDate: month(2027, time.January)}, 0, noHistory, nil, goalNow); gp.Surplus != nil || gp.Fits != nil {
		t.Errorf("sem histórico não há como projetar: %+v", gp)
	}
}

func TestBuildGoalProgress_Reserve(t *testing.T) {
	g := ports.Goal{Kind: ports.GoalReserve, ReserveMonths: 6}
	gp := BuildGoalProgress(g, 9000, goalProjection(), nil, goalNow)
	if gp.Target != 12000 || gp.Coverage != 4.5 || gp.Done {
		t.Errorf("6 x 2000 = 12000; 9000 cobre 4,5 meses: %+v", gp)
	}
	if gp := BuildGoalProgress(g, 12000, goalProjection(), nil, goalNow); !gp.Done {
		t.Errorf("reserva completa: %+v", gp)
	}
	none := goalProjection()
	none.Assumptions.Fixed = 0
	if gp := BuildGoalProgress(g, 5000, none, nil, goalNow); gp.Done || gp.Coverage != 0 {
		t.Errorf("sem contas fixas não há alvo: %+v", gp)
	}
}

func TestBuildGoalProgress_Cut(t *testing.T) {
	cats := []ports.CategoryMonth{
		{Category: "FOOD", Month: month(2026, time.August), Total: 900},
		{Category: "FOOD", Month: month(2026, time.September), Total: 700}, // acima do teto de 680
		{Category: "FOOD", Month: goalNow, Total: 600},
		{Category: "MARKET", Month: goalNow, Total: 9999},
	}
	g := ports.Goal{Kind: ports.GoalCut, Category: "FOOD", CutPercent: 15, Baseline: 800, CreatedAt: time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)}
	gp := BuildGoalProgress(g, 0, Projection{}, cats, goalNow)

	if math.Abs(gp.Target-680) > 0.01 || gp.Current != 600 {
		t.Errorf("teto de 15%% abaixo de 800 e gasto do mês: %+v", gp)
	}
	if len(gp.History) != 2 || gp.History[0].Hit || !gp.History[1].Hit || gp.History[0].Total != 700 {
		t.Errorf("histórico desde o mês da criação (set acima do teto, out dentro): %+v", gp.History)
	}
}

func TestCutBaseline(t *testing.T) {
	cats := []ports.CategoryMonth{
		{Category: "FOOD", Month: month(2026, time.June), Total: 5000}, // fora dos 3 meses
		{Category: "FOOD", Month: month(2026, time.August), Total: 800},
		{Category: "FOOD", Month: month(2026, time.September), Total: 600},
		{Category: "FOOD", Month: goalNow, Total: 100}, // mês atual não entra
		{Category: "MARKET", Month: month(2026, time.September), Total: 50},
	}
	if b, ok := CutBaseline(cats, "FOOD", goalNow); !ok || b != 700 {
		t.Errorf("média de ago e set: %v %v", b, ok)
	}
	if _, ok := CutBaseline(cats, "TRANSPORT", goalNow); ok {
		t.Error("categoria sem gasto não tem baseline")
	}
}
