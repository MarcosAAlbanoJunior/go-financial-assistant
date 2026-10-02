package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

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

// CoachAnalysis é uma análise guardada. Advice é o JSON da análise (formato definido pelo usecase) e Answers,
// as respostas da pessoa às perguntas, por chave.
type CoachAnalysis struct {
	ID        uuid.UUID
	Month     time.Time
	CreatedAt time.Time
	Advice    []byte
	Answers   map[string]string
}

// CoachStore guarda o histórico do Coach.
type CoachStore interface {
	SaveCoachAnalysis(ctx context.Context, a CoachAnalysis) error
	// CoachAnalyses devolve da mais nova para a mais antiga; month nil lê de todos os meses.
	CoachAnalyses(ctx context.Context, month *time.Time, limit int) ([]CoachAnalysis, error)
	// SetCoachAnswer grava a resposta de uma pergunta; answer vazio apaga. false se a análise não existe.
	SetCoachAnswer(ctx context.Context, id uuid.UUID, key, answer string) (bool, error)
	// DeleteCoachAnalysis apaga a análise; false se ela não existe.
	DeleteCoachAnalysis(ctx context.Context, id uuid.UUID) (bool, error)
}
