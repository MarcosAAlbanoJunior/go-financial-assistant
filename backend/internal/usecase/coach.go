package usecase

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const (
	// Limites do que vai para a IA (controle de custo e de exposição).
	MaxCoachSuggestions = 15
	MaxCoachBytes       = 16 << 10
	maxCoachLabel       = 40

	// Limites da resposta da IA.
	maxAdviceActions   = 8
	maxAdviceGoals     = 5
	maxAdviceQuestions = 3
	maxSummaryLen      = 400
	maxCommentLen      = 300
	maxQuestionLen     = 200

	personTransfer = "transferência para pessoa"
)

// CoachContext é exatamente o que é enviado à IA: agregados e nomes de serviços, sem CPF, titular, número
// de conta nem nome de pessoa em Pix e transferências.
type CoachContext struct {
	Month       string            `json:"mes"`
	Months      []string          `json:"meses"`
	Income      float64           `json:"receita_do_mes"`
	Expense     float64           `json:"despesa_do_mes"`
	Categories  []CoachCategory   `json:"categorias"`
	Suggestions []CoachSuggestion `json:"sugestoes"`
	Goals       []CoachGoal       `json:"metas"`
	// Memory são as análises passadas e as respostas da pessoa: a IA lembra o que já foi dito.
	Memory []CoachMemory `json:"memoria"`
}

type CoachCategory struct {
	Name   string    `json:"nome"`
	Values []float64 `json:"gasto_por_mes"`
}

// CoachSuggestion é um candidato da revisão. Os valores são calculados pelo código.
type CoachSuggestion struct {
	ID          string   `json:"id"`
	Kind        string   `json:"tipo"`
	Name        string   `json:"nome"`
	Category    string   `json:"categoria"`
	Amount      float64  `json:"valor_no_mes"`
	SavingMonth float64  `json:"economia_possivel"`
	SavingYear  *float64 `json:"economia_possivel_no_ano"` // nulo nas avulsas
	Months      int      `json:"meses_repetindo,omitempty"`
	Count       int      `json:"lancamentos,omitempty"`
}

type CoachGoal struct {
	ID       string  `json:"id"`
	Kind     string  `json:"tipo"`
	Name     string  `json:"nome"`
	Current  float64 `json:"atual"`
	Target   float64 `json:"alvo"`
	Achieved bool    `json:"atingida"`
}

var coachKindNames = map[string]string{
	ReviewIncrease: "aumento de categoria", ReviewFixed: "conta fixa ou assinatura", ReviewAnt: "gasto formiga",
	ReviewDuplicate: "possível cobrança duplicada", ReviewNew: "conta nova no mês",
}

var coachGoalNames = map[ports.GoalKind]string{
	ports.GoalSave: "juntar valor", ports.GoalCut: "reduzir categoria (alvo = teto mensal)", ports.GoalReserve: "reserva de emergência",
}

var (
	// Pix, TED, DOC e transferências têm o nome da pessoa na descrição.
	transferRE = regexp.MustCompile(`(?i)pix|transfer|\b(ted|doc)\b`)
	// Trechos numéricos (CPF, CNPJ, conta, agência, parcela) e e-mails.
	numberBlobRE = regexp.MustCompile(`\d[\d.\-/*]{2,}|\d{3,}`)
	emailRE      = regexp.MustCompile(`\S+@\S+`)
	spacesRE     = regexp.MustCompile(`\s+`)
)

// CoachLabel limpa a descrição de uma conta antes de ir para a IA. Pix e transferências viram
// "transferência para pessoa"; números longos, e-mails e símbolos somem. Nomes de pessoas dentro de
// descrições de cartão não têm como ser reconhecidos: a pessoa vê o envio antes e pode dispensar o item.
func CoachLabel(label string) string {
	if transferRE.MatchString(label) {
		return personTransfer
	}
	label = emailRE.ReplaceAllString(label, " ")
	label = numberBlobRE.ReplaceAllString(label, " ")
	label = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '&' || r == '-' || r == '\'' {
			return r
		}
		return ' '
	}, label)
	label = strings.TrimSpace(spacesRE.ReplaceAllString(label, " "))
	if utf8.RuneCountInString(label) > maxCoachLabel {
		label = string([]rune(label)[:maxCoachLabel])
	}
	if label == "" {
		return "lançamento"
	}
	return label
}

// CoachCandidates são as sugestões que vão para a IA, na ordem dos IDs "s1", "s2"...: as dispensadas ficam
// de fora (é assim que a pessoa tira um item do envio) e só as MaxCoachSuggestions de maior impacto seguem.
func CoachCandidates(rev Review) []Candidate {
	var out []Candidate
	for _, c := range rev.Candidates {
		if !c.Dismissed && len(out) < MaxCoachSuggestions {
			out = append(out, c)
		}
	}
	return out
}

// BuildCoachContext monta o que vai para a IA a partir dos resultados dos detectores e das metas (os IDs "m1",
// "m2"... seguem a ordem de goals). past são as análises guardadas, da mais nova para a mais antiga.
func BuildCoachContext(rev Review, month string, totals ports.MonthTotals, goals []GoalProgress, past []ports.CoachAnalysis) CoachContext {
	c := CoachContext{Month: month, Income: totals.Income, Expense: totals.Expense,
		Months: []string{}, Categories: []CoachCategory{}, Suggestions: []CoachSuggestion{}, Goals: []CoachGoal{}, Memory: BuildCoachMemory(past)}
	for _, m := range rev.Months {
		c.Months = append(c.Months, m.Format("2006-01"))
	}
	for _, r := range rev.Matrix {
		c.Categories = append(c.Categories, CoachCategory{Name: domain.Category(r.Category).Label(), Values: r.Values})
	}
	for i, cand := range CoachCandidates(rev) {
		s := CoachSuggestion{
			ID: "s" + strconv.Itoa(i+1), Kind: coachKindNames[cand.Kind], Category: domain.Category(cand.Category).Label(),
			Amount: cand.Amount, SavingMonth: cand.Saving, Months: cand.Months, Count: cand.Count,
		}
		if cand.Kind == ReviewIncrease {
			s.Name = s.Category
		} else {
			s.Name = CoachLabel(cand.Label)
		}
		if cand.Recurring {
			year := cand.Saving * 12
			s.SavingYear = &year
		}
		c.Suggestions = append(c.Suggestions, s)
	}
	for i, g := range goals {
		c.Goals = append(c.Goals, CoachGoal{ID: "m" + strconv.Itoa(i+1), Kind: coachGoalNames[g.Goal.Kind], Name: g.Goal.Name, Current: g.Current, Target: g.Target, Achieved: g.Done})
	}
	return c
}

// ErrEmptyAdvice indica que nada da resposta da IA passou na validação.
var ErrEmptyAdvice = errors.New("a IA não devolveu nenhum conteúdo válido")

// cleanAdviceText aceita o texto da IA só se for curto e sem números: valores vêm do código, então um
// número escrito pela IA seria inventado. Devolve "" quando o texto não serve.
func cleanAdviceText(s string, max int) string {
	s = strings.TrimSpace(spacesRE.ReplaceAllString(s, " "))
	if s == "" || utf8.RuneCountInString(s) > max || strings.ContainsFunc(s, unicode.IsDigit) || strings.Contains(s, "R$") {
		return ""
	}
	return s
}

// ValidateAdvice descarta da resposta da IA tudo o que não bate com o contexto enviado: IDs
// desconhecidos ou repetidos, textos longos ou com números, quantidade acima do limite.
func ValidateAdvice(raw ports.CoachAdvice, c CoachContext) (ports.CoachAdvice, error) {
	out := ports.CoachAdvice{Summary: cleanAdviceText(raw.Summary, maxSummaryLen)}

	suggestions := map[string]bool{}
	for _, s := range c.Suggestions {
		suggestions[s.ID] = true
	}
	for _, a := range raw.Actions {
		comment, question := cleanAdviceText(a.Comment, maxCommentLen), cleanAdviceText(a.Question, maxQuestionLen)
		if !suggestions[a.SuggestionID] || (comment == "" && question == "") || len(out.Actions) == maxAdviceActions {
			continue
		}
		delete(suggestions, a.SuggestionID) // uma ação por sugestão
		out.Actions = append(out.Actions, ports.CoachAction{SuggestionID: a.SuggestionID, Priority: min(5, max(1, a.Priority)), Comment: comment, Question: question})
	}

	goals := map[string]bool{}
	for _, g := range c.Goals {
		goals[g.ID] = true
	}
	for _, n := range raw.Goals {
		comment := cleanAdviceText(n.Comment, maxCommentLen)
		if !goals[n.GoalID] || comment == "" || len(out.Goals) == maxAdviceGoals {
			continue
		}
		delete(goals, n.GoalID)
		out.Goals = append(out.Goals, ports.CoachNote{GoalID: n.GoalID, Comment: comment})
	}

	for _, q := range raw.Questions {
		if q = cleanAdviceText(q, maxQuestionLen); q != "" && len(out.Questions) < maxAdviceQuestions {
			out.Questions = append(out.Questions, q)
		}
	}

	if out.Summary == "" && len(out.Actions) == 0 && len(out.Goals) == 0 && len(out.Questions) == 0 {
		return ports.CoachAdvice{}, ErrEmptyAdvice
	}
	return out, nil
}
