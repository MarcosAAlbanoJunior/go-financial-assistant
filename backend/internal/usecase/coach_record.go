package usecase

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain/ports"
)

const (
	// MaxCoachMemory é quantas análises passadas entram no contexto da próxima.
	MaxCoachMemory = 3
	maxAnswerLen   = 300
)

// CoachSuggestionRef são os números e o nome da sugestão, copiados dos detectores na hora da análise.
type CoachSuggestionRef struct {
	Kind          string   `json:"kind"`
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	Category      string   `json:"category"`
	CategoryLabel string   `json:"categoryLabel"`
	Monthly       float64  `json:"monthly"`
	Annual        *float64 `json:"annual"`
	Amount        float64  `json:"amount"`
}

type CoachRecordAction struct {
	SuggestionID string              `json:"suggestionId"`
	Priority     int                 `json:"priority"`
	Comment      string              `json:"comment"`
	Question     string              `json:"question"`
	Suggestion   *CoachSuggestionRef `json:"suggestion"`
}

type CoachRecordGoal struct {
	GoalID  string `json:"goalId"`
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// CoachRecord é a análise como a tela a mostra e o banco a guarda: o texto validado da IA ao lado dos
// números calculados pelo app.
type CoachRecord struct {
	Summary   string              `json:"summary"`
	Actions   []CoachRecordAction `json:"actions"`
	Goals     []CoachRecordGoal   `json:"goals"`
	Questions []string            `json:"questions"`
}

// NewCoachRecord liga a resposta validada aos números dos detectores. candidates e goals são os mesmos
// que geraram os IDs "s1".. e "m1".. do contexto.
func NewCoachRecord(advice ports.CoachAdvice, candidates []Candidate, goals []GoalProgress) CoachRecord {
	r := CoachRecord{Summary: advice.Summary, Actions: []CoachRecordAction{}, Goals: []CoachRecordGoal{}, Questions: []string{}}
	r.Questions = append(r.Questions, advice.Questions...)
	for _, a := range advice.Actions {
		act := CoachRecordAction{SuggestionID: a.SuggestionID, Priority: a.Priority, Comment: a.Comment, Question: a.Question}
		if i, ok := coachIndex(a.SuggestionID, "s", len(candidates)); ok {
			c := candidates[i]
			ref := CoachSuggestionRef{Kind: c.Kind, Key: c.Key, Label: c.Label, Category: c.Category, CategoryLabel: domain.Category(c.Category).Label(), Monthly: c.Saving, Amount: c.Amount}
			if ref.Label == "" {
				ref.Label = ref.CategoryLabel
			}
			if c.Recurring {
				annual := c.Saving * 12
				ref.Annual = &annual
			}
			act.Suggestion = &ref
		}
		r.Actions = append(r.Actions, act)
	}
	for _, g := range advice.Goals {
		if i, ok := coachIndex(g.GoalID, "m", len(goals)); ok {
			r.Goals = append(r.Goals, CoachRecordGoal{GoalID: g.GoalID, Name: goals[i].Goal.Name, Comment: g.Comment})
		}
	}
	return r
}

// coachIndex lê "s3" (prefixo s, base 1) como índice 2 de uma lista de n itens.
func coachIndex(id, prefix string, n int) (int, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	i, err := strconv.Atoi(id[len(prefix):])
	return i - 1, err == nil && i >= 1 && i <= n
}

// QuestionKeys lista as perguntas da análise que a pessoa pode responder: "a:s3" (sobre a sugestão s3) e
// "q:0" (pergunta geral de índice 0).
func (r CoachRecord) QuestionKeys() map[string]string {
	keys := map[string]string{}
	for _, a := range r.Actions {
		if a.Question != "" {
			keys["a:"+a.SuggestionID] = a.Question
		}
	}
	for i, q := range r.Questions {
		keys["q:"+strconv.Itoa(i)] = q
	}
	return keys
}

// CleanAnswer aceita a resposta da pessoa se for curta e sem caracteres de controle; espaços e quebras de
// linha viram um espaço só. Ela é dado, vai na prévia e no contexto da próxima análise.
func CleanAnswer(s string) (string, bool) {
	s = strings.TrimSpace(spacesRE.ReplaceAllString(s, " "))
	if s == "" || utf8.RuneCountInString(s) > maxAnswerLen || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return "", false
	}
	return s, true
}

// CoachMemory é o que a IA lembra de uma análise passada: o resumo e o que a pessoa respondeu.
type CoachMemory struct {
	Month   string              `json:"mes"`
	Summary string              `json:"resumo"`
	Answers []CoachMemoryAnswer `json:"respostas"`
}

type CoachMemoryAnswer struct {
	About    string `json:"sobre,omitempty"`
	Question string `json:"pergunta"`
	Answer   string `json:"resposta"`
}

// BuildCoachMemory resume as análises guardadas (da mais nova para a mais antiga, até MaxCoachMemory).
// Os nomes das sugestões passam pela mesma limpeza do contexto.
func BuildCoachMemory(analyses []ports.CoachAnalysis) []CoachMemory {
	out := []CoachMemory{}
	for _, a := range analyses {
		var rec CoachRecord
		if len(out) == MaxCoachMemory || json.Unmarshal(a.Advice, &rec) != nil {
			continue
		}
		m := CoachMemory{Month: a.Month.Format("2006-01"), Summary: rec.Summary, Answers: []CoachMemoryAnswer{}}
		questions := rec.QuestionKeys()
		for _, act := range rec.Actions {
			key := "a:" + act.SuggestionID
			if answer := a.Answers[key]; answer != "" && act.Suggestion != nil {
				m.Answers = append(m.Answers, CoachMemoryAnswer{About: CoachLabel(act.Suggestion.Label), Question: questions[key], Answer: answer})
			}
		}
		for i := range rec.Questions {
			key := "q:" + strconv.Itoa(i)
			if answer := a.Answers[key]; answer != "" {
				m.Answers = append(m.Answers, CoachMemoryAnswer{Question: questions[key], Answer: answer})
			}
		}
		out = append(out, m)
	}
	return out
}
