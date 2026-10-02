package usecase

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/review"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/usecase/planning"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

func TestCoachLabel(t *testing.T) {
	for in, want := range map[string]string{
		"Pix enviado Fulano de Tal":               personTransfer,
		"PIX RECEBIDO CICLANA":                    personTransfer,
		"Transferência enviada Beltrano":          personTransfer,
		"TED 12345678 Fulano":                     personTransfer,
		"Pagamento de Pix QR Code LOJA":           personTransfer,
		"NETFLIX.COM":                             "NETFLIX COM",
		"Anuidade Diferenciada (1/12)":            "Anuidade Diferenciada",
		"Seguro Cartão ag 1234 cc 98765-4":        "Seguro Cartão ag cc",
		"Mensalidade 123.456.789-09 Academia":     "Mensalidade Academia",
		"Cobrança contato@exemplo.com Hospedagem": "Cobrança Hospedagem",
		"":                      "lançamento",
		"12345":                 "lançamento",
		strings.Repeat("a", 60): strings.Repeat("a", 40),
	} {
		if got := CoachLabel(in); got != want {
			t.Errorf("CoachLabel(%q) = %q, esperava %q", in, got, want)
		}
	}
}

func sampleReview() review.Review {
	return review.Review{
		Months: []time.Time{month(2026, time.August), month(2026, time.September)},
		Matrix: []review.ReviewRow{{Category: "FOOD", Values: []float64{800, 1200}}},
		Candidates: []review.Candidate{
			{Kind: review.ReviewFixed, Key: "streaming", Label: "STREAMING 12345678900", Category: "ENTERTAINMENT", Saving: 40, Recurring: true, Amount: 40, Months: 6},
			{Kind: review.ReviewDuplicate, Key: "pix", Label: "Pix enviado Fulano", Category: "OTHER", Saving: 100, Amount: 100, Count: 2},
			{Kind: review.ReviewIncrease, Key: "FOOD", Category: "FOOD", Saving: 400, Recurring: true, Amount: 1200, Baseline: 800},
			{Kind: review.ReviewAnt, Key: "padaria", Label: "Padaria", Category: "FOOD", Saving: 45, Recurring: true, Amount: 90, Dismissed: true},
		},
	}
}

func TestBuildCoachContext(t *testing.T) {
	goals := []planning.GoalProgress{{Goal: domain.Goal{Kind: domain.GoalSave, Name: "Viagem"}, Current: 100, Target: 1000}}
	c := BuildCoachContext(sampleReview(), "2026-09", domain.MonthTotals{Income: 5000, Expense: 3000}, goals, nil, nil)

	if len(c.Suggestions) != 3 || c.Suggestions[0].ID != "s1" || c.Suggestions[2].ID != "s3" {
		t.Fatalf("dispensadas ficam de fora e os IDs seguem a ordem: %+v", c.Suggestions)
	}
	if c.Suggestions[0].Name != "STREAMING" || c.Suggestions[1].Name != personTransfer || c.Suggestions[2].Name != "Alimentação" {
		t.Errorf("nomes sanitizados (Pix vira transferência, aumento usa a categoria): %+v", c.Suggestions)
	}
	if y := c.Suggestions[0].SavingYear; y == nil || *y != 480 || c.Suggestions[1].SavingYear != nil {
		t.Errorf("anual só nas recorrentes: %+v", c.Suggestions)
	}
	if len(c.Goals) != 1 || c.Goals[0].ID != "m1" || c.Goals[0].Name != "Viagem" || c.Categories[0].Name != "Alimentação" {
		t.Errorf("metas e categorias: %+v %+v", c.Goals, c.Categories)
	}

	raw, _ := json.Marshal(c)
	for _, leaked := range []string{"Fulano", "12345678900", "pix"} {
		if strings.Contains(string(raw), leaked) {
			t.Errorf("o contexto enviado não pode conter %q: %s", leaked, raw)
		}
	}
}

func TestBuildCoachContext_LimitsSizeAndCount(t *testing.T) {
	rev := review.Review{Months: make([]time.Time, review.ReviewMatrixMonths)}
	for i := 0; i < 40; i++ {
		rev.Candidates = append(rev.Candidates, review.Candidate{Kind: review.ReviewFixed, Label: strings.Repeat("Serviço ", 20), Category: "ENTERTAINMENT", Saving: 10, Recurring: true, Amount: 10, Months: 12})
	}
	for _, cat := range []string{"FOOD", "MARKET", "TRANSPORT", "HEALTH", "ENTERTAINMENT", "SHOPPING", "INVESTMENT", "SALARY", "OTHER"} {
		rev.Matrix = append(rev.Matrix, review.ReviewRow{Category: cat, Values: make([]float64, review.ReviewMatrixMonths)})
	}
	var goals []planning.GoalProgress
	for i := 0; i < 20; i++ {
		goals = append(goals, planning.GoalProgress{Goal: domain.Goal{Kind: domain.GoalCut, Name: strings.Repeat("m", 60)}})
	}
	c := BuildCoachContext(rev, "2026-09", domain.MonthTotals{}, goals, nil, nil)
	raw, _ := json.Marshal(c)
	if len(c.Suggestions) != MaxCoachSuggestions || len(raw) > MaxCoachBytes {
		t.Errorf("%d sugestões, %d bytes (limite %d)", len(c.Suggestions), len(raw), MaxCoachBytes)
	}
}

func TestValidateAdvice(t *testing.T) {
	c := CoachContext{Suggestions: []CoachSuggestion{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}, Goals: []CoachGoal{{ID: "m1"}}}
	raw := ports.CoachAdvice{
		Summary: "  Os gastos fixos  pesam\nno mês.  ",
		Actions: []ports.CoachAction{
			{SuggestionID: "s1", Priority: 9, Comment: "Vale rever este serviço."},
			{SuggestionID: "s1", Priority: 1, Comment: "Repetida: ignorada."},
			{SuggestionID: "s9", Priority: 1, Comment: "ID que não existe."},
			{SuggestionID: "s2", Priority: 0, Comment: "Economiza R$ 50 por mês."},                                // número inventado: sem comentário válido
			{SuggestionID: "s3", Priority: 2, Comment: "Compare com 3 meses atrás.", Question: "Você ainda usa?"}, // comentário descartado, pergunta fica
		},
		Goals:     []ports.CoachNote{{GoalID: "m1", Comment: "Está no caminho."}, {GoalID: "m2", Comment: "Meta que não existe."}},
		Questions: []string{"Há algo que mudou na rotina?", "", strings.Repeat("x", 201), "Outra?", "Mais uma?", "Excesso?"},
	}
	got, err := ValidateAdvice(raw, c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "Os gastos fixos pesam no mês." {
		t.Errorf("resumo normalizado: %q", got.Summary)
	}
	if len(got.Actions) != 2 || got.Actions[0].SuggestionID != "s1" || got.Actions[0].Priority != 5 || got.Actions[1].SuggestionID != "s3" || got.Actions[1].Comment != "" || got.Actions[1].Question != "Você ainda usa?" {
		t.Errorf("ações: %+v", got.Actions)
	}
	if len(got.Goals) != 1 || got.Goals[0].GoalID != "m1" {
		t.Errorf("metas: %+v", got.Goals)
	}
	if len(got.Questions) != 3 {
		t.Errorf("no máximo 3 perguntas válidas: %+v", got.Questions)
	}
}

func TestValidateAdvice_CapsActionsAndRejectsEmpty(t *testing.T) {
	var c CoachContext
	var raw ports.CoachAdvice
	for i := 1; i <= 12; i++ {
		id := "s" + string(rune('a'+i))
		c.Suggestions = append(c.Suggestions, CoachSuggestion{ID: id})
		raw.Actions = append(raw.Actions, ports.CoachAction{SuggestionID: id, Comment: "Vale rever."})
	}
	got, err := ValidateAdvice(raw, c)
	if err != nil || len(got.Actions) != maxAdviceActions {
		t.Errorf("limite de ações: %d %v", len(got.Actions), err)
	}
	if _, err := ValidateAdvice(ports.CoachAdvice{Summary: "Gastou R$ 500."}, c); err == nil {
		t.Error("resposta só com texto inválido deveria falhar")
	}
}

func TestCleanAnswer(t *testing.T) {
	for in, want := range map[string]string{
		"  sim,\n  uso toda semana ": "sim, uso toda semana",
		"cancelei":                   "cancelei",
	} {
		if got, ok := CleanAnswer(in); !ok || got != want {
			t.Errorf("CleanAnswer(%q) = %q %v", in, got, ok)
		}
	}
	for name, in := range map[string]string{"vazia": "   ", "longa": strings.Repeat("a", 301), "controle": "a\x00b", "escape": "a\x1b[31mb"} {
		if _, ok := CleanAnswer(in); ok {
			t.Errorf("%s deveria ser recusada", name)
		}
	}
}

func TestNewCoachRecordAndQuestionKeys(t *testing.T) {
	cands := []review.Candidate{
		{Kind: review.ReviewFixed, Key: "streaming", Label: "STREAMING", Category: "ENTERTAINMENT", Saving: 40, Recurring: true, Amount: 40},
		{Kind: review.ReviewDuplicate, Key: "oficina", Label: "Oficina", Category: "OTHER", Saving: 100, Amount: 100},
	}
	goals := []planning.GoalProgress{{Goal: domain.Goal{Name: "Reserva"}}}
	adv := ports.CoachAdvice{
		Summary:   "Resumo.",
		Actions:   []ports.CoachAction{{SuggestionID: "s1", Priority: 1, Comment: "Rever.", Question: "Ainda usa?"}, {SuggestionID: "s2", Priority: 2, Comment: "Conferir."}},
		Goals:     []ports.CoachNote{{GoalID: "m1", Comment: "Ok."}, {GoalID: "m7", Comment: "sem meta"}},
		Questions: []string{"Mudou algo?"},
	}
	r := NewCoachRecord(adv, cands, goals)

	if s := r.Actions[0].Suggestion; s == nil || s.Label != "STREAMING" || s.Annual == nil || *s.Annual != 480 || s.CategoryLabel != "Lazer" {
		t.Errorf("números vêm dos detectores: %+v", s)
	}
	if s := r.Actions[1].Suggestion; s == nil || s.Annual != nil {
		t.Errorf("avulsa sem anual: %+v", s)
	}
	if len(r.Goals) != 1 || r.Goals[0].Name != "Reserva" {
		t.Errorf("meta desconhecida some: %+v", r.Goals)
	}
	keys := r.QuestionKeys()
	if len(keys) != 2 || keys["a:s1"] != "Ainda usa?" || keys["q:0"] != "Mudou algo?" {
		t.Errorf("chaves das perguntas (só as que existem): %v", keys)
	}
}

func TestBuildCoachMemory(t *testing.T) {
	rec := CoachRecord{
		Summary:   "Resumo antigo.",
		Actions:   []CoachRecordAction{{SuggestionID: "s1", Question: "Ainda usa?", Suggestion: &CoachSuggestionRef{Label: "Pix enviado Fulano"}}, {SuggestionID: "s2", Question: "E esta?", Suggestion: &CoachSuggestionRef{Label: "STREAMING"}}},
		Questions: []string{"Mudou algo?"},
	}
	raw, _ := json.Marshal(rec)
	mk := func(m time.Month, answers map[string]string) domain.CoachAnalysis {
		return domain.CoachAnalysis{Month: month(2026, m), Advice: raw, Answers: answers}
	}
	past := []domain.CoachAnalysis{
		mk(time.September, map[string]string{"a:s1": "cancelei", "a:s2": "uso sim", "q:0": "mudei de emprego", "a:s9": "sem pergunta"}),
		{Month: month(2026, time.August), Advice: []byte("não é json")},
		mk(time.July, nil),
		mk(time.June, nil),
		mk(time.May, nil),
	}
	got := BuildCoachMemory(past)
	if len(got) != MaxCoachMemory || got[0].Month != "2026-09" || got[1].Month != "2026-07" || got[1].Answers == nil || len(got[1].Answers) != 0 {
		t.Fatalf("até 3 análises legíveis, da mais nova: %+v", got)
	}
	if a := got[0].Answers; len(a) != 3 || a[0].About != personTransfer || a[0].Answer != "cancelei" || a[1].About != "STREAMING" || a[2].Question != "Mudou algo?" || a[2].About != "" {
		t.Errorf("respostas ligadas às perguntas, com nomes sanitizados: %+v", a)
	}
	b, _ := json.Marshal(got)
	if strings.Contains(string(b), "Fulano") || strings.Contains(string(b), "sem pergunta") {
		t.Errorf("nome de pessoa ou resposta órfã vazou: %s", b)
	}
}
