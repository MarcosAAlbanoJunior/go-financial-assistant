package ports

import (
	"context"
	"time"

	"github.com/MarcosAAlbanoJunior/go-financial-assistant/internal/domain"
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

// CategorySuggestion é a categoria que a IA sugere para uma conta (ID é o enviado no contexto).
type CategorySuggestion struct {
	ID       string
	Category string
}

// Coach interpreta o contexto financeiro (JSON já sanitizado) e devolve sugestões e perguntas.
type Coach interface {
	Advise(ctx context.Context, contextJSON []byte) (CoachAdvice, error)
	// Categorize sugere categorias para contas (JSON já sanitizado). Quem chama valida a resposta.
	Categorize(ctx context.Context, contextJSON []byte) ([]CategorySuggestion, error)
}

// CoachStore guarda o histórico do Coach.
type CoachStore interface {
	SaveCoachAnalysis(ctx context.Context, a domain.CoachAnalysis) error
	// CoachAnalyses devolve da mais nova para a mais antiga; month nil lê de todos os meses.
	CoachAnalyses(ctx context.Context, month *time.Time, limit int) ([]domain.CoachAnalysis, error)
	// SetCoachAnswer grava a resposta de uma pergunta; answer vazio apaga. false se a análise não existe.
	SetCoachAnswer(ctx context.Context, id uuid.UUID, key, answer string) (bool, error)
	// DeleteCoachAnalysis apaga a análise; false se ela não existe.
	DeleteCoachAnalysis(ctx context.Context, id uuid.UUID) (bool, error)
}
