package ports

import "context"

// CoachAction é o comentário da IA sobre uma sugestão da revisão (SuggestionID é o ID enviado no contexto).
type CoachAction struct {
	SuggestionID string
	Priority     int // 1 (mais importante) a 5
	Comment      string
	Question     string // pergunta para a pessoa, ex.: "você ainda usa este serviço?"
}

// CoachNote é o comentário da IA sobre uma meta (GoalID é o ID enviado no contexto).
type CoachNote struct {
	GoalID  string
	Comment string
}

// CoachAdvice é a resposta da IA: só texto. Os números vêm sempre do código, nunca daqui.
type CoachAdvice struct {
	Summary   string
	Actions   []CoachAction
	Goals     []CoachNote
	Questions []string // perguntas gerais
}

// Coach interpreta o contexto financeiro (JSON já sanitizado) e devolve sugestões e perguntas.
type Coach interface {
	Advise(ctx context.Context, contextJSON []byte) (CoachAdvice, error)
}
