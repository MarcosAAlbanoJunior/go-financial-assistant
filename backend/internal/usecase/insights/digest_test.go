package insights

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/review"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
)

func TestMonthStartAndBankBalance(t *testing.T) {
	if got := MonthStart(time.Date(2026, 10, 17, 23, 59, 0, 0, time.FixedZone("x", -3*3600))); !got.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("MonthStart: %v", got)
	}
	if BankBalance(nil) != nil || BankBalance([]domain.Account{{Type: "CREDIT", Balance: 50}}) != nil {
		t.Error("sem conta corrente o saldo é desconhecido (nil), não zero")
	}
	if got := BankBalance([]domain.Account{{Type: "BANK", Balance: 100}, {Type: "CREDIT", Balance: 999}, {Type: "BANK", Balance: 50.5}}); got == nil || *got != 150.5 {
		t.Errorf("soma só as contas correntes: %v", got)
	}
}

func TestFormatDigest(t *testing.T) {
	today := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	over := 930.0
	noFit := false
	surplus := 800.0
	rv := review.Review{Candidates: []review.Candidate{
		{Kind: review.ReviewDuplicate, Label: "OFICINA (1/3)", Count: 2, Amount: 250},
		{Kind: review.ReviewNew, Label: "Curso A", Amount: 120},
		{Kind: review.ReviewNew, Label: "Curso B", Amount: 90},
		{Kind: review.ReviewNew, Label: "Curso C", Amount: 80},
		{Kind: review.ReviewNew, Label: "Curso D", Amount: 70}, // o quarto fica de fora
		{Kind: review.ReviewDuplicate, Label: "Dispensada", Count: 2, Amount: 10, Dismissed: true},
		{Kind: review.ReviewFixed, Label: "Streaming", Amount: 40, Recurring: true}, // fixa não é alerta
	}}
	goals := []planning.GoalProgress{
		{Goal: domain.Goal{Kind: domain.GoalCut, Name: "Comida"}, Current: 300, Target: 800, Projected: &over},
		{Goal: domain.Goal{Kind: domain.GoalCut, Name: "Lazer"}, Current: 900, Target: 800},
		{Goal: domain.Goal{Kind: domain.GoalCut, Name: "Calma"}, Current: 100, Target: 800},
		{Goal: domain.Goal{Kind: domain.GoalSave, Name: "Viagem"}, PerMonth: 2000, Surplus: &surplus, Fits: &noFit},
	}
	savings := []review.DecisionResult{
		{Decision: domain.Decision{Label: "Academia", Monthly: 99}, Status: review.SavingReturned, Returned: 99},
		{Decision: domain.Decision{Label: "Streaming", Monthly: 40}, Status: review.SavingConfirmed, MonthsConfirmed: 3, Realized: 120},
	}
	text := FormatDigest(today, domain.MonthTotals{Income: 5000, Expense: 1433.63}, rv, goals, savings)

	for _, want := range []string{
		"Resumo semanal — 12/10/2026", "despesas R$ 1.433,63 · receitas R$ 5.000,00", "Economia realizada com o que você cancelou: R$ 120,00 (R$ 40,00 por mês).",
		"A cobrança voltou: Academia", "Possível cobrança duplicada: OFICINA (2 vezes de R$ 250,00", "Conta nova neste mês: Curso A (R$ 120,00)", "Curso C",
		"Meta \"Comida\": no ritmo atual o mês fecha em R$ 930,00, acima do teto de R$ 800,00", "Meta \"Lazer\": o mês já passou R$ 100,00 do teto",
		"Meta \"Viagem\": guardar R$ 2.000,00 por mês não cabe na sobra projetada de R$ 800,00",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("falta %q em:\n%s", want, text)
		}
	}
	for _, absent := range []string{"Curso D", "Dispensada", "Calma", "Streaming (", "(1/3)"} {
		if strings.Contains(text, absent) {
			t.Errorf("não deveria citar %q em:\n%s", absent, text)
		}
	}
	if strings.Contains(text, "*") || strings.Contains(text, "_") {
		t.Errorf("texto simples, sem Markdown: %s", text)
	}
}

func TestFormatDigest_QuietWeekAndLimits(t *testing.T) {
	today := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	quiet := FormatDigest(today, domain.MonthTotals{}, review.Review{}, nil, nil)
	if !strings.Contains(quiet, "Sem alertas esta semana.") || strings.Contains(quiet, "Economia realizada") {
		t.Errorf("semana tranquila: %s", quiet)
	}

	var dups []review.Candidate
	for i := 0; i < 30; i++ {
		dups = append(dups, review.Candidate{Kind: review.ReviewDuplicate, Label: strings.Repeat("x", 100), Count: 2, Amount: 10})
	}
	long := FormatDigest(today, domain.MonthTotals{}, review.Review{Candidates: dups}, nil, nil)
	if n := strings.Count(long, "•"); n != maxDigestAlerts || len([]rune(long)) > 4000 {
		t.Errorf("limite de alertas e de tamanho (Telegram aceita 4096): %d alertas, %d caracteres", n, len([]rune(long)))
	}
}

func TestNextDigestTime(t *testing.T) {
	sp, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Skip("sem tzdata")
	}
	at := func(y int, m time.Month, d, h, min int, loc *time.Location) time.Time {
		return time.Date(y, m, d, h, min, 0, 0, loc)
	}

	// Quinta 01/10/2026 às 22h de Brasília: a próxima segunda às 9h é 05/10.
	if got := NextDigestTime(at(2026, 10, 1, 22, 0, sp), time.Monday, 9, sp); !got.Equal(at(2026, 10, 5, 9, 0, sp)) {
		t.Errorf("próxima segunda: %v", got)
	}
	// Na segunda antes das 9h, é hoje; depois das 9h (ou exatamente às 9h), só na semana seguinte.
	if got := NextDigestTime(at(2026, 10, 5, 8, 59, sp), time.Monday, 9, sp); !got.Equal(at(2026, 10, 5, 9, 0, sp)) {
		t.Errorf("hoje mesmo: %v", got)
	}
	for _, now := range []time.Time{at(2026, 10, 5, 9, 0, sp), at(2026, 10, 5, 9, 1, sp)} {
		if got := NextDigestTime(now, time.Monday, 9, sp); !got.Equal(at(2026, 10, 12, 9, 0, sp)) {
			t.Errorf("semana seguinte a partir de %v: %v", now, got)
		}
	}
	// O dia vale no fuso configurado, não no UTC: 02h UTC de terça é 23h de segunda em Brasília.
	utc := time.Date(2026, 10, 6, 2, 0, 0, 0, time.UTC)
	// (segunda 23h em Brasília já passou, ou é agora: a próxima é a segunda seguinte)
	if got := NextDigestTime(utc, time.Monday, 23, sp); !got.Equal(at(2026, 10, 12, 23, 0, sp)) {
		t.Errorf("fuso: %v", got)
	}
	if got := NextDigestTime(utc, time.Monday, 9, sp); got.Weekday() != time.Monday || got.Hour() != 9 || !got.After(utc) {
		t.Errorf("sempre na segunda às 9h locais, no futuro: %v", got)
	}
}

// Desligado, o job só espera; ao ligar pela configuração (changed), passa a agendar, e sai com o contexto.
func TestDigestJob_FollowsScheduleChanges(t *testing.T) {
	var enabled atomic.Bool
	changed := make(chan struct{}, 1)
	job := NewDigestJob(nil, nil, "", slog.New(slog.NewTextHandler(io.Discard, nil)),
		func() (bool, time.Weekday, int, *time.Location) { return enabled.Load(), time.Monday, 9, time.UTC }, changed)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { job.Run(ctx); close(done) }()

	time.Sleep(30 * time.Millisecond)
	enabled.Store(true)
	changed <- struct{}{} // acorda; agora espera até a próxima segunda às 9h
	time.Sleep(30 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("o job não deveria ter terminado")
	default:
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("o job deveria terminar com o contexto")
	}
}
